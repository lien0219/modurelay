package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/mail"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

const SettingKeyInvoiceEnabled = "invoice_enabled"
const InvoiceMinimumAmountCents int64 = 30000
const InvoiceDocumentMaxBytes int64 = 10 * 1024 * 1024

var (
	ErrInvoiceDocumentRequired = infraerrors.BadRequest("INVOICE_DOCUMENT_REQUIRED", "标记已开票前请先上传 PDF 电子发票")
	ErrInvoiceDocumentNotFound = infraerrors.NotFound("INVOICE_DOCUMENT_NOT_FOUND", "电子发票不存在或尚未开票")
	ErrInvoiceDisabled         = infraerrors.Forbidden("INVOICE_DISABLED", "发票申请功能暂未开放")
	ErrInvoiceNotFound         = infraerrors.NotFound("INVOICE_NOT_FOUND", "发票申请不存在")
	ErrInvoiceAmount           = infraerrors.BadRequest("INVOICE_AMOUNT_INVALID", "单次开票金额最低为人民币 300 元")
	ErrInvoiceFunds            = infraerrors.BadRequest("INVOICE_INSUFFICIENT_AMOUNT", "累计净实付人民币充值或剩余可开票金额不足（最低 300 元）")
	ErrInvoiceStatus           = infraerrors.Conflict("INVOICE_STATUS_CONFLICT", "申请状态已变化或不允许此操作，请刷新后重试")
	ErrInvoiceRefund           = infraerrors.Conflict("INVOICE_REFUND_CONFLICT", "退款后充值金额不足以覆盖发票申请，请先取消待处理申请；已开票记录请联系财务处理")
)

type InvoiceInput struct {
	Contact            string `json:"contact"`
	RequestedIssueDate string `json:"requested_issue_date"`
	AmountCents        int64  `json:"amount_cents"`
	InvoiceType        string `json:"invoice_type"`
	TitleType          string `json:"title_type"`
	Title              string `json:"title"`
	TaxNumber          string `json:"tax_number"`
	Email              string `json:"email"`
	Address            string `json:"address"`
	Phone              string `json:"phone"`
	BankName           string `json:"bank_name"`
	BankAccount        string `json:"bank_account"`
	Remark             string `json:"remark"`
}

type Invoice struct {
	InvoiceInput
	ID            int64                    `json:"id"`
	ApplicationNo string                   `json:"application_no"`
	UserID        int64                    `json:"user_id"`
	UserEmail     string                   `json:"user_email"`
	UserName      string                   `json:"user_name"`
	Status        string                   `json:"status"`
	AdminRemark   string                   `json:"admin_remark"`
	InvoiceNumber string                   `json:"invoice_number"`
	CreatedAt     time.Time                `json:"created_at"`
	UpdatedAt     time.Time                `json:"updated_at"`
	ProcessedAt   *time.Time               `json:"processed_at"`
	Document      *InvoiceDocumentMetadata `json:"document"`
}

type InvoiceDocumentMetadata struct {
	Filename   string    `json:"filename"`
	SizeBytes  int64     `json:"size_bytes"`
	UploadedAt time.Time `json:"uploaded_at"`
	SHA256     string    `json:"sha256"`
}

type InvoiceDocument struct {
	InvoiceDocumentMetadata
	Content []byte `json:"-"`
}

type InvoiceEligibility struct {
	Enabled              bool  `json:"enabled"`
	MinimumAmountCents   int64 `json:"minimum_amount_cents"`
	TotalRechargedCents  int64 `json:"total_recharged_cents"`
	ReservedAmountCents  int64 `json:"reserved_amount_cents"`
	AvailableAmountCents int64 `json:"available_amount_cents"`
}

type InvoiceUpdate struct {
	Status        string `json:"status"`
	AdminRemark   string `json:"admin_remark"`
	InvoiceNumber string `json:"invoice_number"`
}

type InvoiceFilter struct {
	IDs      []int64
	UserID   int64
	Status   string
	Search   string
	Page     int
	PageSize int
}

type InvoiceRepository interface {
	SaveDocument(context.Context, int64, *InvoiceDocument) (*Invoice, error)
	GetDocument(context.Context, int64, int64) (*InvoiceDocument, error)
	Eligibility(context.Context, int64) (*InvoiceEligibility, error)
	Create(context.Context, int64, InvoiceInput) (*Invoice, error)
	Get(context.Context, int64, int64) (*Invoice, error)
	List(context.Context, InvoiceFilter) ([]*Invoice, int64, error)
	Export(context.Context, InvoiceFilter) ([]*Invoice, error)
	Update(context.Context, int64, int64, InvoiceUpdate) (*Invoice, error)
	// BeforeRefund runs in the payment transaction, sharing the invoice user lock.
	BeforeRefund(context.Context, *dbent.Client, *dbent.PaymentOrder, float64) error
}

type InvoiceService struct {
	repo     InvoiceRepository
	settings SettingRepository
	onUpdate func()
}

func NewInvoiceService(repo InvoiceRepository, settings SettingRepository) *InvoiceService {
	return &InvoiceService{repo: repo, settings: settings}
}

func (s *InvoiceService) SetOnUpdateCallback(callback func()) { s.onUpdate = callback }

func (s *InvoiceService) Enabled(ctx context.Context) (bool, error) {
	value, err := s.settings.GetValue(ctx, SettingKeyInvoiceEnabled)
	if errors.Is(err, ErrSettingNotFound) {
		return false, nil
	}
	return value == "true", err
}

func (s *InvoiceService) SetEnabled(ctx context.Context, enabled bool) error {
	if err := s.settings.Set(ctx, SettingKeyInvoiceEnabled, strconv.FormatBool(enabled)); err != nil {
		return err
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}
	return nil
}

func (s *InvoiceService) Eligibility(ctx context.Context, userID int64) (*InvoiceEligibility, error) {
	enabled, err := s.Enabled(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.repo.Eligibility(ctx, userID)
	if err != nil {
		return nil, err
	}
	result.Enabled = enabled
	return result, nil
}

func (s *InvoiceService) Create(ctx context.Context, userID int64, input InvoiceInput) (*Invoice, error) {
	if err := ValidateInvoiceInput(&input); err != nil {
		return nil, err
	}
	enabled, err := s.Enabled(ctx)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, ErrInvoiceDisabled
	}
	return s.repo.Create(ctx, userID, input)
}

func (s *InvoiceService) Get(ctx context.Context, id, userID int64) (*Invoice, error) {
	return s.repo.Get(ctx, id, userID)
}

func (s *InvoiceService) List(ctx context.Context, filter InvoiceFilter) ([]*Invoice, int64, error) {
	if filter.Status != "" && !ValidInvoiceStatus(filter.Status) {
		return nil, 0, invoiceInvalid("申请状态无效")
	}
	filter.Search = strings.TrimSpace(filter.Search)
	if utf8.RuneCountInString(filter.Search) > 200 {
		return nil, 0, invoiceInvalid("搜索内容不能超过 200 字")
	}
	return s.repo.List(ctx, filter)
}

func (s *InvoiceService) Cancel(ctx context.Context, id, userID int64) (*Invoice, error) {
	return s.repo.Update(ctx, id, userID, InvoiceUpdate{Status: "cancelled"})
}

func (s *InvoiceService) Update(ctx context.Context, id int64, update InvoiceUpdate) (*Invoice, error) {
	update.AdminRemark = strings.TrimSpace(update.AdminRemark)
	update.InvoiceNumber = strings.TrimSpace(update.InvoiceNumber)
	if update.Status != "processing" && update.Status != "issued" && update.Status != "rejected" {
		return nil, invoiceInvalid("审核状态无效")
	}
	if utf8.RuneCountInString(update.AdminRemark) > 1000 || utf8.RuneCountInString(update.InvoiceNumber) > 100 || strings.ContainsRune(update.AdminRemark+update.InvoiceNumber, '\x00') {
		return nil, invoiceInvalid("审核信息过长或包含无效字符")
	}
	if update.Status == "issued" && update.InvoiceNumber == "" {
		return nil, invoiceInvalid("已开票必须填写发票号码")
	}
	if update.Status == "rejected" && update.AdminRemark == "" {
		return nil, invoiceInvalid("拒绝申请必须填写原因")
	}
	return s.repo.Update(ctx, id, 0, update)
}

func invoiceInvalid(message string) error {
	return infraerrors.BadRequest("INVOICE_INVALID_INPUT", message)
}

func ValidateInvoiceInput(input *InvoiceInput) error {
	if input.AmountCents < InvoiceMinimumAmountCents || input.AmountCents > 9007199254740991 {
		return ErrInvoiceAmount
	}
	fields := []struct {
		value *string
		limit int
	}{{&input.Contact, 200}, {&input.RequestedIssueDate, 10}, {&input.Title, 200}, {&input.TaxNumber, 32}, {&input.Email, 254}, {&input.Address, 500}, {&input.Phone, 50}, {&input.BankName, 200}, {&input.BankAccount, 100}, {&input.Remark, 1000}}
	for _, field := range fields {
		*field.value = strings.TrimSpace(*field.value)
		if utf8.RuneCountInString(*field.value) > field.limit || strings.ContainsRune(*field.value, '\x00') {
			return invoiceInvalid("申请信息过长或包含无效字符")
		}
	}
	if input.RequestedIssueDate != "" {
		if parsed, err := time.Parse("2006-01-02", input.RequestedIssueDate); err != nil || parsed.Format("2006-01-02") != input.RequestedIssueDate {
			return invoiceInvalid("期望开票日期格式必须为 YYYY-MM-DD")
		}
	}
	if input.InvoiceType != "vat_normal" && input.InvoiceType != "vat_special" {
		return invoiceInvalid("发票类型无效")
	}
	if input.TitleType != "personal" && input.TitleType != "enterprise" {
		return invoiceInvalid("抬头类型无效")
	}
	if input.Title == "" {
		return invoiceInvalid("请填写发票抬头")
	}
	address, err := mail.ParseAddress(input.Email)
	if err != nil || address.Address != input.Email || !strings.Contains(input.Email, "@") {
		return invoiceInvalid("请填写有效的收票邮箱")
	}
	if input.TitleType == "enterprise" && input.TaxNumber == "" {
		return invoiceInvalid("企业发票必须填写税号")
	}
	if input.InvoiceType == "vat_special" && (input.TitleType != "enterprise" || input.Address == "" || input.Phone == "" || input.BankName == "" || input.BankAccount == "") {
		return invoiceInvalid("增值税专用发票需要企业抬头、地址、电话、开户银行及账号")
	}
	return nil
}

func ValidInvoiceStatus(status string) bool {
	switch status {
	case "pending", "processing", "issued", "rejected", "cancelled":
		return true
	}
	return false
}

func CanTransitionInvoice(from, to string, userAction bool) bool {
	if userAction {
		return from == "pending" && to == "cancelled"
	}
	return (from == "pending" && (to == "processing" || to == "issued" || to == "rejected")) || (from == "processing" && (to == "issued" || to == "rejected"))
}

// CSVText prevents formulas even when an attacker prefixes whitespace/control characters.
func InvoiceCSVText(value string) string {
	trimmed := strings.TrimLeftFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) || r == '\ufeff' })
	if strings.HasPrefix(trimmed, "=") || strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "@") || strings.ContainsAny(value, "\t\r\n") {
		return "'" + value
	}
	return value
}

func invoiceCSVLabel(value string) string {
	switch value {
	case "vat_normal":
		return "增值税普通发票"
	case "vat_special":
		return "增值税专用发票"
	case "personal":
		return "个人"
	case "enterprise":
		return "企业"
	case "pending":
		return "待处理"
	case "processing":
		return "开票中"
	case "issued":
		return "已开票"
	case "rejected":
		return "已拒绝"
	case "cancelled":
		return "已取消"
	default:
		return value
	}
}

// InvoiceExportColumn is a fixed allowlist; arbitrary field paths are never evaluated.
type InvoiceExportColumn struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

var InvoiceExportColumns = []InvoiceExportColumn{
	{"application_no", "申请编号"}, {"user_id", "用户ID"}, {"user_email", "用户账号"}, {"user_name", "昵称"},
	{"amount", "金额（人民币元）"}, {"invoice_type", "发票类型"}, {"title_type", "抬头类型"}, {"title", "抬头"},
	{"tax_number", "税号"}, {"email", "收票邮箱"}, {"address", "地址"}, {"phone", "电话"},
	{"bank_name", "开户银行"}, {"bank_account", "银行账号"}, {"remark", "申请备注"}, {"status", "状态"},
	{"admin_remark", "审核备注"}, {"invoice_number", "发票号码"}, {"created_at", "申请时间"}, {"processed_at", "处理时间"},
	{"contact", "联系方式"}, {"requested_issue_date", "期望开票日期"},
}

func ValidateInvoiceExportColumns(columns []string) ([]string, error) {
	if len(columns) == 0 {
		result := make([]string, len(InvoiceExportColumns))
		for i, column := range InvoiceExportColumns {
			result[i] = column.Key
		}
		return result, nil
	}
	allowed := make(map[string]bool, len(InvoiceExportColumns))
	for _, column := range InvoiceExportColumns {
		allowed[column.Key] = true
	}
	seen := make(map[string]bool, len(columns))
	for _, key := range columns {
		if !allowed[key] || seen[key] {
			return nil, invoiceInvalid("导出列无效或重复")
		}
		seen[key] = true
	}
	return columns, nil
}

func WriteInvoiceCSV(w io.Writer, invoices []*Invoice) error {
	return WriteInvoiceCSVColumns(w, invoices, nil)
}

func WriteInvoiceCSVColumns(w io.Writer, invoices []*Invoice, columns []string) error {
	columns, err := ValidateInvoiceExportColumns(columns)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\xef\xbb\xbf"); err != nil {
		return err
	}
	labels := make(map[string]string, len(InvoiceExportColumns))
	for _, column := range InvoiceExportColumns {
		labels[column.Key] = column.Label
	}
	header := make([]string, len(columns))
	for i, key := range columns {
		header[i] = labels[key]
	}
	csvWriter := csv.NewWriter(w)
	if err := csvWriter.Write(header); err != nil {
		return err
	}
	for _, inv := range invoices {
		processed := ""
		if inv.ProcessedAt != nil {
			processed = inv.ProcessedAt.UTC().Format(time.RFC3339)
		}
		values := map[string]string{
			"application_no": inv.ApplicationNo, "user_id": strconv.FormatInt(inv.UserID, 10), "user_email": inv.UserEmail, "user_name": inv.UserName,
			"contact": inv.Contact, "requested_issue_date": inv.RequestedIssueDate, "amount": fmt.Sprintf("%d.%02d", inv.AmountCents/100, inv.AmountCents%100),
			"invoice_type": invoiceCSVLabel(inv.InvoiceType), "title_type": invoiceCSVLabel(inv.TitleType), "title": inv.Title, "tax_number": inv.TaxNumber,
			"email": inv.Email, "address": inv.Address, "phone": inv.Phone, "bank_name": inv.BankName, "bank_account": inv.BankAccount,
			"remark": inv.Remark, "status": invoiceCSVLabel(inv.Status), "admin_remark": inv.AdminRemark, "invoice_number": inv.InvoiceNumber,
			"created_at": inv.CreatedAt.UTC().Format(time.RFC3339), "processed_at": processed,
		}
		row := make([]string, len(columns))
		for i, key := range columns {
			row[i] = InvoiceCSVText(values[key])
			// Preserve financial identifiers, leading zeroes and long numeric contact details.
			switch key {
			case "tax_number", "phone", "bank_account", "invoice_number", "contact":
				if row[i] != "" && !strings.HasPrefix(row[i], "'") {
					row[i] = "'" + row[i]
				}
			}
		}
		if err := csvWriter.Write(row); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

// ValidateInvoiceDocument accepts a bounded PDF payload and stores a safe filename.
// The media type is checked against the bytes, never trusted from the extension alone.
func ValidateInvoiceDocument(filename, contentType string, content []byte) (*InvoiceDocument, error) {
	if int64(len(content)) > InvoiceDocumentMaxBytes || len(content) < 16 {
		return nil, invoiceInvalid("电子发票必须为不超过 10 MiB 的 PDF 文件")
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || (mediaType != "application/pdf" && mediaType != "application/octet-stream") || http.DetectContentType(content) != "application/pdf" {
		return nil, invoiceInvalid("电子发票只支持 PDF 文件")
	}
	if !bytes.HasPrefix(content, []byte("%PDF-")) || !bytes.HasSuffix(bytes.TrimSpace(content), []byte("%%EOF")) {
		return nil, invoiceInvalid("PDF 文件无效或不完整")
	}
	filename = path.Base(strings.ReplaceAll(filename, "\\", "/"))
	filename = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, filename))
	if filename == "" || filename == "." || filename == "/" || !strings.EqualFold(path.Ext(filename), ".pdf") {
		return nil, invoiceInvalid("电子发票文件名必须以 .pdf 结尾")
	}
	if utf8.RuneCountInString(filename) > 200 {
		return nil, invoiceInvalid("PDF 文件名不能超过 200 字")
	}
	digest := sha256.Sum256(content)
	return &InvoiceDocument{InvoiceDocumentMetadata: InvoiceDocumentMetadata{Filename: filename, SizeBytes: int64(len(content)), SHA256: hex.EncodeToString(digest[:])}, Content: content}, nil
}

func (s *InvoiceService) UploadDocument(ctx context.Context, id int64, filename, contentType string, content []byte) (*Invoice, error) {
	document, err := ValidateInvoiceDocument(filename, contentType, content)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveDocument(ctx, id, document)
}

func (s *InvoiceService) GetDocument(ctx context.Context, id, userID int64) (*InvoiceDocument, error) {
	return s.repo.GetDocument(ctx, id, userID)
}

// InvoicePaymentRemainderCents converts refund wallet units back into historical
// cash units using the same rounding as the gateway refund calculation.
func InvoicePaymentRemainderCents(order *dbent.PaymentOrder, refundAmount float64) int64 {
	if order == nil || order.OrderType != "balance" || order.PaidAt == nil || order.Amount <= 0 || order.PayAmount <= 0 {
		return 0
	}
	currency := psSnapshotStringValue(order.ProviderSnapshot["currency"])
	if currency == "" {
		providerKey := psSnapshotStringValue(order.ProviderSnapshot["provider_key"])
		if providerKey == "" {
			providerKey = psStringValue(order.ProviderKey)
		}
		switch providerKey {
		case "alipay", "wxpay", "easypay":
			currency = "CNY"
		}
	}
	if !strings.EqualFold(currency, "CNY") {
		return 0
	}
	refund := calculateGatewayRefundAmount(order.Amount, order.PayAmount, refundAmount, "CNY")
	remaining := decimal.NewFromFloat(order.PayAmount).Sub(decimal.NewFromFloat(refund)).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (s *InvoiceService) Export(ctx context.Context, filter InvoiceFilter) ([]*Invoice, error) {
	if filter.Status != "" && !ValidInvoiceStatus(filter.Status) {
		return nil, invoiceInvalid("申请状态无效")
	}
	filter.Search = strings.TrimSpace(filter.Search)
	if utf8.RuneCountInString(filter.Search) > 200 {
		return nil, invoiceInvalid("搜索内容不能超过 200 字")
	}
	if len(filter.IDs) > 1000 {
		return nil, invoiceInvalid("单次最多选择 1000 条申请")
	}
	seen := make(map[int64]bool, len(filter.IDs))
	for _, id := range filter.IDs {
		if id <= 0 || seen[id] {
			return nil, invoiceInvalid("申请编号无效或重复")
		}
		seen[id] = true
	}
	return s.repo.Export(ctx, filter)
}
