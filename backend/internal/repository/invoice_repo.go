package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

type invoiceRepository struct{ db *sql.DB }

func NewInvoiceRepository(db *sql.DB) service.InvoiceRepository { return &invoiceRepository{db: db} }

type invoiceQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// All invoice reservations and entry into a refund state share this lock.
// It never locks users/payment_orders in reverse order to the payment service.
func lockInvoiceUser(ctx context.Context, q invoiceQuerier, userID int64) error {
	rows, err := q.QueryContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('invoice-user:' || $1::text, 0))`, userID)
	if err != nil {
		return err
	}
	return rows.Close()
}

// pay_amount is cash paid (including fees); amount and refund_amount are wallet
// units including bonuses. Match calculateGatewayRefundAmount's rounding to cents.
// Historical orders without a verifiable currency snapshot are deliberately not
// converted at today's exchange rate or inferred from the current provider config.
const invoiceOrderNetSQL = `CASE
 WHEN order_type = 'balance' AND (UPPER(TRIM(provider_snapshot->>'currency')) = 'CNY' OR (COALESCE(TRIM(provider_snapshot->>'currency'), '') = '' AND COALESCE(NULLIF(TRIM(provider_snapshot->>'provider_key'), ''), provider_key) IN ('alipay','wxpay','easypay'))) AND paid_at IS NOT NULL AND amount > 0 AND pay_amount > 0 THEN
 CASE
  WHEN status IN ('COMPLETED', 'REFUND_FAILED') THEN ROUND(pay_amount * 100)
  WHEN status IN ('PARTIALLY_REFUNDED', 'REFUND_REQUESTED', 'REFUNDING', 'REFUND_PENDING') THEN
   CASE WHEN refund_amount <= 0 OR amount - refund_amount <= 0.01 THEN 0 ELSE GREATEST(0, ROUND(pay_amount * 100) - ROUND(pay_amount * refund_amount / amount * 100)) END
  ELSE 0 END
 ELSE 0 END`

const invoiceReservedSQL = `SELECT COALESCE(SUM(amount_cents), 0) FROM invoice_applications WHERE user_id = $1 AND status IN ('pending', 'processing', 'issued')`

func invoiceFunds(ctx context.Context, q invoiceQuerier, userID int64, excludeOrderID int64) (*service.InvoiceEligibility, error) {
	rows, err := q.QueryContext(ctx, `SELECT
 (SELECT COALESCE(SUM(`+invoiceOrderNetSQL+`),0)::bigint FROM payment_orders WHERE user_id = $1 AND id <> $2),
 (`+invoiceReservedSQL+`)::bigint`, userID, excludeOrderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := &service.InvoiceEligibility{MinimumAmountCents: service.InvoiceMinimumAmountCents}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	if err := rows.Scan(&result.TotalRechargedCents, &result.ReservedAmountCents); err != nil {
		return nil, err
	}
	result.AvailableAmountCents = max(int64(0), result.TotalRechargedCents-result.ReservedAmountCents)
	return result, rows.Err()
}

func (r *invoiceRepository) Eligibility(ctx context.Context, userID int64) (*service.InvoiceEligibility, error) {
	return invoiceFunds(ctx, r.db, userID, 0)
}

func (r *invoiceRepository) Create(ctx context.Context, userID int64, input service.InvoiceInput) (*service.Invoice, error) {
	if err := service.ValidateInvoiceInput(&input); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := lockInvoiceUser(ctx, tx, userID); err != nil {
		return nil, err
	}
	var enabled bool
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT value = 'true' FROM settings WHERE key = 'invoice_enabled'), false)`).Scan(&enabled); err != nil {
		return nil, err
	}
	if !enabled {
		return nil, service.ErrInvoiceDisabled
	}
	funds, err := invoiceFunds(ctx, tx, userID, 0)
	if err != nil {
		return nil, err
	}
	if funds.TotalRechargedCents < service.InvoiceMinimumAmountCents || input.AmountCents > funds.AvailableAmountCents {
		return nil, service.ErrInvoiceFunds
	}
	inv, err := scanInvoice(tx.QueryRowContext(ctx, `INSERT INTO invoice_applications
 (application_no,user_id,user_email,user_name,amount_cents,invoice_type,title_type,title,tax_number,email,address,phone,bank_name,bank_account,remark,contact,requested_issue_date)
 SELECT $1,id,email,username,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,NULLIF($15,'')::date FROM users WHERE id = $2 AND deleted_at IS NULL
 RETURNING `+invoiceColumns, "INV"+strings.ReplaceAll(uuid.NewString(), "-", ""), userID, input.AmountCents, input.InvoiceType, input.TitleType, input.Title, input.TaxNumber, input.Email, input.Address, input.Phone, input.BankName, input.BankAccount, input.Remark, input.Contact, input.RequestedIssueDate))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return inv, nil
}

const invoiceColumns = `id,application_no,user_id,user_email,user_name,amount_cents,invoice_type,title_type,title,tax_number,email,address,phone,bank_name,bank_account,remark,status,admin_remark,invoice_number,created_at,updated_at,processed_at,contact,COALESCE(requested_issue_date::text,''),
 (SELECT jsonb_build_object('filename',d.filename,'size_bytes',d.size_bytes,'uploaded_at',d.uploaded_at,'sha256',d.sha256)
 FROM invoice_documents d WHERE d.invoice_id=invoice_applications.id)`

func scanInvoice(row interface{ Scan(...any) error }) (*service.Invoice, error) {
	i := &service.Invoice{}
	var metadata []byte
	err := row.Scan(&i.ID, &i.ApplicationNo, &i.UserID, &i.UserEmail, &i.UserName, &i.AmountCents, &i.InvoiceType, &i.TitleType, &i.Title, &i.TaxNumber, &i.Email, &i.Address, &i.Phone, &i.BankName, &i.BankAccount, &i.Remark, &i.Status, &i.AdminRemark, &i.InvoiceNumber, &i.CreatedAt, &i.UpdatedAt, &i.ProcessedAt, &i.Contact, &i.RequestedIssueDate, &metadata)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrInvoiceNotFound
	}
	if err == nil && len(metadata) > 0 {
		i.Document = &service.InvoiceDocumentMetadata{}
		err = json.Unmarshal(metadata, i.Document)
	}
	return i, err
}

func (r *invoiceRepository) Get(ctx context.Context, id, userID int64) (*service.Invoice, error) {
	return scanInvoice(r.db.QueryRowContext(ctx, `SELECT `+invoiceColumns+` FROM invoice_applications WHERE id = $1 AND ($2::bigint = 0 OR user_id = $2)`, id, userID))
}

func invoiceWhere(filter service.InvoiceFilter) (string, []any) {
	where := " WHERE 1=1"
	args := []any{}
	add := func(condition string, value any) {
		args = append(args, value)
		where += fmt.Sprintf(condition, len(args))
	}
	if len(filter.IDs) > 0 {
		placeholders := make([]string, len(filter.IDs))
		for i, id := range filter.IDs {
			args = append(args, id)
			placeholders[i] = "$" + strconv.Itoa(len(args))
		}
		where += " AND id IN (" + strings.Join(placeholders, ",") + ")"
	}
	if filter.UserID > 0 {
		add(" AND user_id = $%d", filter.UserID)
	}
	if filter.Status != "" {
		add(" AND status = $%d", filter.Status)
	}
	if filter.Search != "" {
		args = append(args, "%"+strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(filter.Search)+"%")
		p := strconv.Itoa(len(args))
		where += " AND (application_no ILIKE $" + p + " OR user_email ILIKE $" + p + " OR user_name ILIKE $" + p + " OR title ILIKE $" + p + " OR tax_number ILIKE $" + p + " OR contact ILIKE $" + p + ")"
	}
	return where, args
}

func (r *invoiceRepository) List(ctx context.Context, filter service.InvoiceFilter) ([]*service.Invoice, int64, error) {
	where, args := invoiceWhere(filter)
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoice_applications`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	page := max(filter.Page, 1)
	size := filter.PageSize
	if size <= 0 {
		size = 20
	}
	if size > 1000 {
		size = 1000
	}
	args = append(args, size, (page-1)*size)
	rows, err := r.db.QueryContext(ctx, `SELECT `+invoiceColumns+` FROM invoice_applications`+where+` ORDER BY created_at DESC,id DESC LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*service.Invoice, 0)
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, inv)
	}
	return items, total, rows.Err()
}

func (r *invoiceRepository) Update(ctx context.Context, id, userID int64, update service.InvoiceUpdate) (*service.Invoice, error) {
	if update.Status == "rejected" && strings.TrimSpace(update.AdminRemark) == "" {
		return nil, infraerrors.BadRequest("INVOICE_INVALID_INPUT", "拒绝申请必须填写原因")
	}
	if update.Status == "issued" && strings.TrimSpace(update.InvoiceNumber) == "" {
		return nil, infraerrors.BadRequest("INVOICE_INVALID_INPUT", "已开票必须填写发票号码")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Read owner without a row lock, then take the shared user lock before the row.
	var owner int64
	if err := tx.QueryRowContext(ctx, `SELECT user_id FROM invoice_applications WHERE id=$1 AND ($2::bigint=0 OR user_id=$2)`, id, userID).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrInvoiceNotFound
		}
		return nil, err
	}
	if err := lockInvoiceUser(ctx, tx, owner); err != nil {
		return nil, err
	}
	current, err := scanInvoice(tx.QueryRowContext(ctx, `SELECT `+invoiceColumns+` FROM invoice_applications WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return nil, err
	}
	if !service.CanTransitionInvoice(current.Status, update.Status, userID > 0) {
		return nil, service.ErrInvoiceStatus
	}
	if update.Status == "issued" && current.Document == nil {
		return nil, service.ErrInvoiceDocumentRequired
	}
	if update.Status == "processing" || update.Status == "issued" {
		funds, err := invoiceFunds(ctx, tx, owner, 0)
		if err != nil {
			return nil, err
		}
		if funds.TotalRechargedCents < funds.ReservedAmountCents {
			return nil, service.ErrInvoiceFunds
		}
	}
	inv, err := scanInvoice(tx.QueryRowContext(ctx, `UPDATE invoice_applications SET status=$2,admin_remark=$3,invoice_number=$4,updated_at=NOW(),processed_at=NOW() WHERE id=$1 RETURNING `+invoiceColumns, id, update.Status, update.AdminRemark, update.InvoiceNumber))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return inv, nil
}

func (r *invoiceRepository) BeforeRefund(ctx context.Context, client *dbent.Client, order *dbent.PaymentOrder, refundAmount float64) error {
	if err := lockInvoiceUser(ctx, client, order.UserID); err != nil {
		return err
	}
	funds, err := invoiceFunds(ctx, client, order.UserID, order.ID)
	if err != nil {
		return err
	}
	remaining := service.InvoicePaymentRemainderCents(order, refundAmount)
	if funds.ReservedAmountCents > 0 && funds.TotalRechargedCents+remaining < funds.ReservedAmountCents {
		return service.ErrInvoiceRefund
	}
	return nil
}

// Export uses one statement/snapshot so concurrent submissions cannot shift pages.
func (r *invoiceRepository) Export(ctx context.Context, filter service.InvoiceFilter) ([]*service.Invoice, error) {
	where, args := invoiceWhere(filter)
	rows, err := r.db.QueryContext(ctx, `SELECT `+invoiceColumns+` FROM invoice_applications`+where+` ORDER BY created_at DESC,id DESC LIMIT 50001`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]*service.Invoice, 0)
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) > 50000 {
		return nil, infraerrors.BadRequest("INVOICE_EXPORT_TOO_LARGE", "单次最多导出 50000 条，请使用状态或搜索条件缩小范围")
	}
	return items, nil
}

// SaveDocument and Update use the same lock order so a PDF cannot race an issued
// transition. An already issued PDF is immutable; legacy issued rows may be backfilled.
func (r *invoiceRepository) SaveDocument(ctx context.Context, id int64, document *service.InvoiceDocument) (*service.Invoice, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var owner int64
	if err := tx.QueryRowContext(ctx, `SELECT user_id FROM invoice_applications WHERE id=$1`, id).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrInvoiceNotFound
		}
		return nil, err
	}
	if err := lockInvoiceUser(ctx, tx, owner); err != nil {
		return nil, err
	}
	current, err := scanInvoice(tx.QueryRowContext(ctx, `SELECT `+invoiceColumns+` FROM invoice_applications WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return nil, err
	}
	if current.Status == "rejected" || current.Status == "cancelled" || (current.Status == "issued" && current.Document != nil) {
		return nil, service.ErrInvoiceStatus
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO invoice_documents(invoice_id,filename,size_bytes,sha256,content)
 VALUES($1,$2,$3,$4,$5) ON CONFLICT(invoice_id) DO UPDATE SET filename=EXCLUDED.filename,size_bytes=EXCLUDED.size_bytes,sha256=EXCLUDED.sha256,content=EXCLUDED.content,uploaded_at=NOW()`, id, document.Filename, document.SizeBytes, document.SHA256, document.Content); err != nil {
		return nil, err
	}
	inv, err := scanInvoice(tx.QueryRowContext(ctx, `UPDATE invoice_applications SET updated_at=NOW() WHERE id=$1 RETURNING `+invoiceColumns, id))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return inv, nil
}

func (r *invoiceRepository) GetDocument(ctx context.Context, id, userID int64) (*service.InvoiceDocument, error) {
	document := &service.InvoiceDocument{}
	err := r.db.QueryRowContext(ctx, `SELECT d.filename,d.size_bytes,d.uploaded_at,d.sha256,d.content
 FROM invoice_documents d JOIN invoice_applications i ON i.id=d.invoice_id
 WHERE i.id=$1 AND ($2::bigint=0 OR (i.user_id=$2 AND i.status='issued'))`, id, userID).
		Scan(&document.Filename, &document.SizeBytes, &document.UploadedAt, &document.SHA256, &document.Content)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrInvoiceDocumentNotFound
	}
	return document, err
}
