package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalidVerificationRecordFilter = errors.New("invalid verification record filter")

type VerificationRecordListOptions struct {
	Page        int
	PageSize    int
	UserID      int64
	Type        string
	Outcome     string
	ServiceCode string
	Region      string
	Keyword     string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

type VerificationRecord struct {
	ID                       string     `json:"id"`
	OrderNo                  string     `json:"order_no"`
	VerificationType         string     `json:"verification_type"`
	ProductType              string     `json:"product_type"`
	UserID                   *int64     `json:"user_id,omitempty"`
	UserEmail                string     `json:"user_email,omitempty"`
	ServiceCode              string     `json:"service_code"`
	ChannelCode              string     `json:"channel_code"`
	ChannelName              string     `json:"channel_name"`
	ProviderCode             string     `json:"provider_code,omitempty"`
	Target                   string     `json:"target"`
	Region                   string     `json:"region,omitempty"`
	Status                   string     `json:"status"`
	Outcome                  string     `json:"outcome"`
	RefundStatus             string     `json:"refund_status"`
	RefundReason             string     `json:"refund_reason,omitempty"`
	SaleAmount               float64    `json:"sale_amount"`
	ProviderCost             *float64   `json:"provider_cost,omitempty"`
	ProviderCostEstimated    bool       `json:"provider_cost_estimated,omitempty"`
	SettlementEstimated      bool       `json:"settlement_estimated,omitempty"`
	UserDebitAmount          float64    `json:"user_debit_amount"`
	ReservedAmount           float64    `json:"reserved_amount"`
	CapturedAmount           float64    `json:"captured_amount"`
	ReleasedAmount           float64    `json:"released_amount"`
	RefundedAmount           float64    `json:"refunded_amount"`
	Currency                 string     `json:"currency"`
	ProviderRequestCount     *int       `json:"provider_request_count,omitempty"`
	ErrorCode                string     `json:"error_code,omitempty"`
	ErrorMessage             string     `json:"error_message,omitempty"`
	PublicErrorMessage       string     `json:"public_error_message,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
	CompletedAt              *time.Time `json:"completed_at,omitempty"`
	ExpiresAt                *time.Time `json:"expires_at,omitempty"`
}

type VerificationRecordSummary struct {
	Total           int64    `json:"total"`
	Processing      int64    `json:"processing"`
	Success         int64    `json:"success"`
	Failed          int64    `json:"failed"`
	Refunded        int64    `json:"refunded"`
	Cancelled       int64    `json:"cancelled"`
	Expired         int64    `json:"expired"`
	SaleAmount      float64  `json:"sale_amount"`
	UserDebitAmount float64  `json:"user_debit_amount"`
	ReservedAmount  float64  `json:"reserved_amount"`
	CapturedAmount  float64  `json:"captured_amount"`
	ReleasedAmount  float64  `json:"released_amount"`
	RefundedAmount  float64  `json:"refunded_amount"`
	ProviderCost    *float64 `json:"provider_cost,omitempty"`
	NetRevenue      float64  `json:"net_revenue"`
	EstimatedProfit *float64 `json:"estimated_profit,omitempty"`
}

type VerificationRecordPage struct {
	Items     []VerificationRecord         `json:"items"`
	Total     int64                        `json:"total"`
	Page      int                          `json:"page"`
	PageSize  int                          `json:"page_size"`
	Pages     int                          `json:"pages"`
	Summary   VerificationRecordSummary    `json:"summary"`
	Analytics *VerificationRecordAnalytics `json:"analytics,omitempty"`
}

type VerificationRecordBreakdown struct {
	Key         string  `json:"key"`
	Total       int64   `json:"total"`
	Success     int64   `json:"success"`
	SuccessRate float64 `json:"success_rate"`
}

type VerificationRecordFinancialTotal struct {
	Currency        string  `json:"currency"`
	SaleAmount      float64 `json:"sale_amount"`
	ProviderCost    float64 `json:"provider_cost"`
	CapturedAmount  float64 `json:"captured_amount"`
	RefundedAmount  float64 `json:"refunded_amount"`
	NetRevenue      float64 `json:"net_revenue"`
	EstimatedProfit float64 `json:"estimated_profit"`
	Estimated       bool    `json:"estimated"`
}

type VerificationRecordAnalytics struct {
	ByPlatform []VerificationRecordBreakdown      `json:"by_platform"`
	ByCountry  []VerificationRecordBreakdown      `json:"by_country"`
	ByType     []VerificationRecordBreakdown      `json:"by_type"`
	Financial  []VerificationRecordFinancialTotal `json:"financial"`
}

type VerificationRecordOptionListOptions struct {
	Kind     string
	Type     string
	Query    string
	Page     int
	PageSize int
}

type VerificationRecordOption struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	LabelEN string `json:"label_en,omitempty"`
	Icon    string `json:"icon,omitempty"`
}

type VerificationRecordOptionPage struct {
	Items    []VerificationRecordOption `json:"items"`
	Total    int64                      `json:"total"`
	Page     int                        `json:"page"`
	PageSize int                        `json:"page_size"`
	Pages    int                        `json:"pages"`
}

type VerificationRecordService struct {
	db *sql.DB
}

func NewVerificationRecordService(db *sql.DB) *VerificationRecordService {
	return &VerificationRecordService{db: db}
}

const verificationRecordsCTE = `WITH email_usage AS (
	SELECT email_order_id,
		COUNT(*)::integer AS request_count,
		COALESCE(SUM(estimated_request_cost),0)::float8 AS usage_cost
	FROM email_provider_usage
	GROUP BY email_order_id
), records AS (
	SELECT
		o.public_id::text AS id,
		o.public_id::text AS order_no,
		'sms'::text AS verification_type,
		o.product_type,
		o.user_id,
		u.email AS user_email,
		sv.code AS service_code,
		c.code AS channel_code,
		c.public_name AS channel_name,
		p.code AS provider_code,
		COALESCE(o.phone_number, '') AS target,
		co.iso2 AS region,
		o.status,
		CASE
			WHEN o.status = 'refunded' OR o.refund_status = 'approved' THEN 'refunded'
			WHEN o.status = 'completed' THEN 'success'
			WHEN o.status = 'failed' THEN 'failed'
			WHEN o.status = 'cancelled' THEN 'cancelled'
			WHEN o.status = 'expired' THEN 'expired'
			ELSE 'processing'
		END AS outcome,
		o.refund_status,
		COALESCE(o.refund_reason, '') AS refund_reason,
		o.sale_price_snapshot::float8 AS sale_amount,
		CASE
			WHEN (o.settlement_status='released' AND o.captured_amount=0)
			  OR o.provider_refund_status IN ('succeeded','not_required')
			  OR (p.code='smspva' AND o.product_type='temporary' AND o.settlement_status='held' AND o.first_sms_received_at IS NULL)
			THEN 0::float8
			ELSE o.provider_cost_snapshot::float8
		END AS provider_cost,
		CASE
			WHEN o.settlement_status='legacy' THEN
				CASE
					WHEN o.refund_status='approved' OR o.status='refunded' THEN o.sale_price_snapshot::float8
					WHEN o.provider_order_id<>'' AND o.status IN ('active','completed','expired','cancelled','provider_unknown','reconciling') THEN o.sale_price_snapshot::float8
					ELSE 0::float8
				END
			ELSE o.captured_amount::float8
		END AS user_debit_amount,
		CASE WHEN o.settlement_status='legacy' THEN 0::float8 ELSE o.reserved_amount::float8 END AS reserved_amount,
		CASE
			WHEN o.settlement_status='legacy' THEN
				CASE
					WHEN o.refund_status='approved' OR o.status='refunded' THEN o.sale_price_snapshot::float8
					WHEN o.provider_order_id<>'' AND o.status IN ('active','completed','expired','cancelled','provider_unknown','reconciling') THEN o.sale_price_snapshot::float8
					ELSE 0::float8
				END
			ELSE o.captured_amount::float8
		END AS captured_amount,
		CASE
			WHEN o.settlement_status='legacy' AND o.status='failed' THEN o.sale_price_snapshot::float8
			WHEN o.settlement_status='legacy' THEN 0::float8
			ELSE o.released_amount::float8
		END AS released_amount,
		CASE
			WHEN o.settlement_status='legacy' AND (o.status='refunded' OR o.refund_status='approved') THEN o.sale_price_snapshot::float8
			WHEN o.settlement_status='legacy' THEN 0::float8
			ELSE o.refunded_amount::float8
		END AS refunded_amount,
		COALESCE(NULLIF(o.currency_snapshot, ''), 'USD') AS currency,
		NULL::integer AS provider_request_count,
		''::text AS error_code,
		COALESCE(o.last_provider_error, '') AS error_message,
		''::text AS public_error_message,
		o.created_at,
		o.updated_at,
		CASE WHEN o.status = 'completed' THEN o.updated_at ELSE NULL::timestamptz END AS completed_at,
		o.expires_at,
		CASE
			WHEN (o.settlement_status='released' AND o.captured_amount=0)
			  OR o.provider_refund_status IN ('succeeded','not_required')
			  OR (p.code='smspva' AND o.product_type='temporary' AND o.settlement_status='held' AND o.first_sms_received_at IS NULL)
			THEN FALSE
			WHEN o.provider_cost_snapshot<=0 THEN FALSE
			ELSE p.code <> '5sim'
		END AS provider_cost_estimated,
		(o.settlement_status='legacy') AS settlement_estimated
	FROM sms_orders o
	JOIN users u ON u.id = o.user_id
	JOIN sms_services sv ON sv.id = o.service_id
	JOIN sms_channels c ON c.id = o.channel_id
	JOIN sms_providers p ON p.id = o.provider_id
	JOIN sms_countries co ON co.id = o.country_id

	UNION ALL

	SELECT
		o.public_id::text AS id,
		o.order_no,
		'email'::text AS verification_type,
		o.address_type AS product_type,
		o.user_id,
		u.email AS user_email,
		sv.code AS service_code,
		c.code AS channel_code,
		c.public_name AS channel_name,
		p.code AS provider_code,
		COALESCE(o.email_address, '') AS target,
		''::text AS region,
		o.status,
		CASE
			WHEN o.status = 'refunded' OR o.refund_status = 'approved' THEN 'refunded'
			WHEN o.status = 'completed' THEN 'success'
			WHEN o.status = 'failed' THEN 'failed'
			WHEN o.status = 'cancelled' THEN 'cancelled'
			WHEN o.status = 'expired' THEN 'expired'
			ELSE 'processing'
		END AS outcome,
		o.refund_status,
		COALESCE(o.refund_reason, '') AS refund_reason,
		o.sale_price_snapshot::float8 AS sale_amount,
		CASE
			WHEN lower(COALESCE(p.billing->>'cost_mode',''))='fixed_per_order' THEN
				CASE WHEN o.provider_inbox_id<>'' OR o.email_address<>'' THEN o.provider_cost_estimate_snapshot::float8 ELSE 0::float8 END
			WHEN COALESCE(eu.request_count,0)>0 THEN COALESCE(eu.usage_cost,0)::float8
			ELSE o.provider_cost_estimate_snapshot::float8
		END AS provider_cost,
		o.captured_amount::float8 AS user_debit_amount,
		o.reserved_amount::float8,
		o.captured_amount::float8,
		o.released_amount::float8,
		o.refunded_amount::float8,
		'CNY'::text AS currency,
		GREATEST(o.provider_request_count,COALESCE(eu.request_count,0))::integer AS provider_request_count,
		COALESCE(o.error_code, '') AS error_code,
		COALESCE(o.error_admin_message, '') AS error_message,
		COALESCE(o.error_public_message, '') AS public_error_message,
		o.created_at,
		o.updated_at,
		o.completed_at,
		o.expires_at,
		CASE
			WHEN lower(COALESCE(p.billing->>'cost_mode',''))='fixed_per_order'
				THEN o.provider_cost_estimate_snapshot>0 AND (o.provider_inbox_id<>'' OR o.email_address<>'')
			WHEN COALESCE(eu.request_count,0)>0 THEN COALESCE(eu.usage_cost,0)>0
			ELSE o.provider_cost_estimate_snapshot>0
		END AS provider_cost_estimated,
		FALSE AS settlement_estimated
	FROM email_orders o
	JOIN users u ON u.id = o.user_id
	JOIN email_services sv ON sv.id = o.service_id
	JOIN email_channels c ON c.id = o.channel_id
	JOIN email_providers p ON p.id = o.provider_id
	LEFT JOIN email_usage eu ON eu.email_order_id=o.id
)`

func (s *VerificationRecordService) List(ctx context.Context, options VerificationRecordListOptions, admin bool) (*VerificationRecordPage, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("verification record database unavailable")
	}
	if options.Page <= 0 {
		options.Page = 1
	}
	if options.PageSize <= 0 {
		options.PageSize = 20
	}
	if options.PageSize > 100 {
		options.PageSize = 100
	}
	options.Type = strings.ToLower(strings.TrimSpace(options.Type))
	options.Outcome = strings.ToLower(strings.TrimSpace(options.Outcome))
	options.ServiceCode = strings.ToLower(strings.TrimSpace(options.ServiceCode))
	options.Region = strings.ToUpper(strings.TrimSpace(options.Region))
	options.Keyword = strings.TrimSpace(options.Keyword)
	if options.Type != "" && options.Type != "sms" && options.Type != "email" {
		return nil, fmt.Errorf("%w: unsupported verification type", ErrInvalidVerificationRecordFilter)
	}
	if options.Outcome != "" && !isVerificationOutcome(options.Outcome) {
		return nil, fmt.Errorf("%w: unsupported outcome", ErrInvalidVerificationRecordFilter)
	}
	if !admin && options.UserID <= 0 {
		return nil, fmt.Errorf("%w: user is required", ErrInvalidVerificationRecordFilter)
	}
	if options.CreatedFrom != nil && options.CreatedTo != nil && !options.CreatedFrom.Before(*options.CreatedTo) {
		return nil, fmt.Errorf("%w: invalid date range", ErrInvalidVerificationRecordFilter)
	}

	where, args := buildVerificationRecordWhere(options, admin)
	summaryQuery := verificationRecordsCTE + `
	SELECT
		COUNT(*)::bigint,
		COUNT(*) FILTER (WHERE outcome = 'processing')::bigint,
		COUNT(*) FILTER (WHERE outcome = 'success')::bigint,
		COUNT(*) FILTER (WHERE outcome = 'failed')::bigint,
		COUNT(*) FILTER (WHERE outcome = 'refunded')::bigint,
		COUNT(*) FILTER (WHERE outcome = 'cancelled')::bigint,
		COUNT(*) FILTER (WHERE outcome = 'expired')::bigint,
		COALESCE(SUM(sale_amount), 0)::float8,
		COALESCE(SUM(user_debit_amount), 0)::float8,
		COALESCE(SUM(reserved_amount), 0)::float8,
		COALESCE(SUM(captured_amount), 0)::float8,
		COALESCE(SUM(released_amount), 0)::float8,
		COALESCE(SUM(refunded_amount), 0)::float8,
		COALESCE(SUM(provider_cost), 0)::float8
	FROM records` + where

	var summary VerificationRecordSummary
	var providerCost float64
	err := s.db.QueryRowContext(ctx, summaryQuery, args...).Scan(
		&summary.Total,
		&summary.Processing,
		&summary.Success,
		&summary.Failed,
		&summary.Refunded,
		&summary.Cancelled,
		&summary.Expired,
		&summary.SaleAmount,
		&summary.UserDebitAmount,
		&summary.ReservedAmount,
		&summary.CapturedAmount,
		&summary.ReleasedAmount,
		&summary.RefundedAmount,
		&providerCost,
	)
	if err != nil {
		return nil, err
	}
	summary.NetRevenue = summary.CapturedAmount - summary.RefundedAmount
	if admin {
		summary.ProviderCost = verificationFloat64Ptr(providerCost)
		summary.EstimatedProfit = verificationFloat64Ptr(summary.NetRevenue - providerCost)
	}

	listArgs := append([]any{}, args...)
	limitPlaceholder := fmt.Sprintf("$%d", len(listArgs)+1)
	listArgs = append(listArgs, options.PageSize)
	offsetPlaceholder := fmt.Sprintf("$%d", len(listArgs)+1)
	listArgs = append(listArgs, (options.Page-1)*options.PageSize)
	listQuery := verificationRecordsCTE + `
	SELECT id, order_no, verification_type, product_type, user_id, user_email,
		service_code, channel_code, channel_name, provider_code, target, region,
		status, outcome, refund_status, refund_reason, sale_amount, provider_cost,
		user_debit_amount, reserved_amount, captured_amount, released_amount,
		refunded_amount, currency, provider_request_count, error_code, error_message,
		public_error_message, created_at, updated_at, completed_at, expires_at,
		provider_cost_estimated, settlement_estimated
	FROM records` + where + `
	ORDER BY created_at DESC, id DESC
	LIMIT ` + limitPlaceholder + ` OFFSET ` + offsetPlaceholder

	rows, err := s.db.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]VerificationRecord, 0, options.PageSize)
	for rows.Next() {
		var item VerificationRecord
		var userID int64
		var providerItemCost float64
		var providerRequestCount sql.NullInt64
		var providerCostEstimated, settlementEstimated bool
		var errorCode, errorMessage string
		var completedAt, expiresAt sql.NullTime
		if err := rows.Scan(
			&item.ID,
			&item.OrderNo,
			&item.VerificationType,
			&item.ProductType,
			&userID,
			&item.UserEmail,
			&item.ServiceCode,
			&item.ChannelCode,
			&item.ChannelName,
			&item.ProviderCode,
			&item.Target,
			&item.Region,
			&item.Status,
			&item.Outcome,
			&item.RefundStatus,
			&item.RefundReason,
			&item.SaleAmount,
			&providerItemCost,
			&item.UserDebitAmount,
			&item.ReservedAmount,
			&item.CapturedAmount,
			&item.ReleasedAmount,
			&item.RefundedAmount,
			&item.Currency,
			&providerRequestCount,
			&errorCode,
			&errorMessage,
			&item.PublicErrorMessage,
			&item.CreatedAt,
			&item.UpdatedAt,
			&completedAt,
			&expiresAt,
			&providerCostEstimated,
			&settlementEstimated,
		); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			item.CompletedAt = &completedAt.Time
		}
		if expiresAt.Valid {
			item.ExpiresAt = &expiresAt.Time
		}
		if admin {
			item.UserID = verificationInt64Ptr(userID)
			item.ProviderCost = verificationFloat64Ptr(providerItemCost)
			item.ProviderCostEstimated = providerCostEstimated
			item.SettlementEstimated = settlementEstimated
			if providerRequestCount.Valid {
				item.ProviderRequestCount = verificationIntPtr(int(providerRequestCount.Int64))
			}
			item.ErrorCode = errorCode
			item.ErrorMessage = errorMessage
		} else {
			item.UserEmail = ""
			item.ProviderCode = ""
			item.RefundReason = ""
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	pages := int((summary.Total + int64(options.PageSize) - 1) / int64(options.PageSize))
	if pages < 1 {
		pages = 1
	}
	return &VerificationRecordPage{
		Items:    items,
		Total:    summary.Total,
		Page:     options.Page,
		PageSize: options.PageSize,
		Pages:    pages,
		Summary:  summary,
	}, nil
}

// Analytics returns server-side aggregate data for the same filters as List.
// It is intentionally separate from List so existing callers and SQL contracts
// remain stable while dashboards can request a bounded aggregate response.
func (s *VerificationRecordService) Analytics(ctx context.Context, options VerificationRecordListOptions, admin bool) (*VerificationRecordAnalytics, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("verification record database unavailable")
	}
	options.Type = strings.ToLower(strings.TrimSpace(options.Type))
	options.Outcome = strings.ToLower(strings.TrimSpace(options.Outcome))
	options.ServiceCode = strings.ToLower(strings.TrimSpace(options.ServiceCode))
	options.Region = strings.ToUpper(strings.TrimSpace(options.Region))
	options.Keyword = strings.TrimSpace(options.Keyword)
	if options.Type != "" && options.Type != "sms" && options.Type != "email" {
		return nil, fmt.Errorf("%w: unsupported verification type", ErrInvalidVerificationRecordFilter)
	}
	if options.Outcome != "" && !isVerificationOutcome(options.Outcome) {
		return nil, fmt.Errorf("%w: unsupported outcome", ErrInvalidVerificationRecordFilter)
	}
	if !admin && options.UserID <= 0 {
		return nil, fmt.Errorf("%w: user is required", ErrInvalidVerificationRecordFilter)
	}
	where, args := buildVerificationRecordWhere(options, admin)
	query := verificationRecordsCTE + `, filtered AS (SELECT * FROM records` + where + `)
	SELECT dimension, key, total, success, CASE WHEN total = 0 THEN 0 ELSE success::float8 / total::float8 END AS success_rate
	FROM (
		SELECT 'platform'::text AS dimension, service_code AS key, COUNT(*) FILTER (WHERE outcome <> 'processing')::bigint AS total, COUNT(*) FILTER (WHERE outcome='success')::bigint AS success FROM filtered GROUP BY service_code
		UNION ALL
		SELECT 'country'::text, region, COUNT(*) FILTER (WHERE outcome <> 'processing')::bigint, COUNT(*) FILTER (WHERE outcome='success')::bigint FROM filtered WHERE verification_type='sms' AND region <> '' GROUP BY region
		UNION ALL
		SELECT 'type'::text, verification_type, COUNT(*) FILTER (WHERE outcome <> 'processing')::bigint, COUNT(*) FILTER (WHERE outcome='success')::bigint FROM filtered GROUP BY verification_type
	) breakdown ORDER BY dimension, total DESC, key`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := &VerificationRecordAnalytics{ByPlatform: []VerificationRecordBreakdown{}, ByCountry: []VerificationRecordBreakdown{}, ByType: []VerificationRecordBreakdown{}, Financial: []VerificationRecordFinancialTotal{}}
	for rows.Next() {
		var dimension, key string
		var item VerificationRecordBreakdown
		if err := rows.Scan(&dimension, &key, &item.Total, &item.Success, &item.SuccessRate); err != nil {
			return nil, err
		}
		item.Key = key
		switch dimension {
		case "platform":
			result.ByPlatform = append(result.ByPlatform, item)
		case "country":
			result.ByCountry = append(result.ByCountry, item)
		case "type":
			result.ByType = append(result.ByType, item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	financialQuery := verificationRecordsCTE + `, filtered AS (SELECT * FROM records` + where + `)
	SELECT currency, COALESCE(SUM(sale_amount),0)::float8, COALESCE(SUM(provider_cost),0)::float8,
	 COALESCE(SUM(captured_amount),0)::float8, COALESCE(SUM(refunded_amount),0)::float8,
	 COALESCE(BOOL_OR(provider_cost_estimated OR settlement_estimated),FALSE)
	FROM filtered GROUP BY currency ORDER BY currency`
	rows, err = s.db.QueryContext(ctx, financialQuery, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var item VerificationRecordFinancialTotal
		if err := rows.Scan(&item.Currency, &item.SaleAmount, &item.ProviderCost, &item.CapturedAmount, &item.RefundedAmount, &item.Estimated); err != nil {
			return nil, err
		}
		item.NetRevenue = item.CapturedAmount - item.RefundedAmount
		item.EstimatedProfit = item.NetRevenue - item.ProviderCost
		if admin {
			result.Financial = append(result.Financial, item)
		}
	}
	return result, rows.Err()
}

func (s *VerificationRecordService) ListOptions(ctx context.Context, options VerificationRecordOptionListOptions) (*VerificationRecordOptionPage, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("verification record database unavailable")
	}
	options.Kind = strings.ToLower(strings.TrimSpace(options.Kind))
	options.Type = strings.ToLower(strings.TrimSpace(options.Type))
	options.Query = strings.TrimSpace(options.Query)
	if options.Kind != "platform" && options.Kind != "country" {
		return nil, fmt.Errorf("%w: unsupported option kind", ErrInvalidVerificationRecordFilter)
	}
	if options.Type != "" && options.Type != "sms" && options.Type != "email" {
		return nil, fmt.Errorf("%w: unsupported verification type", ErrInvalidVerificationRecordFilter)
	}
	if options.Page <= 0 {
		options.Page = 1
	}
	if options.PageSize <= 0 {
		options.PageSize = 20
	}
	if options.PageSize > 50 {
		options.PageSize = 50
	}
	if options.Kind == "country" && options.Type == "email" {
		return &VerificationRecordOptionPage{Items: []VerificationRecordOption{}, Page: options.Page, PageSize: options.PageSize, Pages: 1}, nil
	}

	baseQuery := `SELECT value,label,label_en,icon FROM (`
	if options.Kind == "country" {
		baseQuery += `SELECT iso2 AS value, COALESCE(NULLIF(name_zh,''),NULLIF(name_en,''),iso2) AS label, COALESCE(NULLIF(name_en,''),NULLIF(name_zh,''),iso2) AS label_en, ''::text AS icon FROM sms_countries WHERE enabled`
	} else {
		parts := make([]string, 0, 2)
		if options.Type == "" || options.Type == "sms" {
			parts = append(parts, `SELECT code AS value,name AS label,name AS label_en,icon FROM sms_services WHERE enabled`)
		}
		if options.Type == "" || options.Type == "email" {
			parts = append(parts, `SELECT code AS value,name AS label,name AS label_en,''::text AS icon FROM email_services WHERE enabled`)
		}
		baseQuery += strings.Join(parts, ` UNION ALL `)
	}
	baseQuery += `) catalog`
	args := []any{}
	if options.Query != "" {
		args = append(args, "%"+options.Query+"%")
		baseQuery += ` WHERE value ILIKE $1 OR label ILIKE $1 OR label_en ILIKE $1`
	}
	groupedQuery := `SELECT value,MAX(label) AS label,MAX(label_en) AS label_en,MAX(icon) AS icon FROM (` + baseQuery + `) filtered GROUP BY value`
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+groupedQuery+`) options`, args...).Scan(&total); err != nil {
		return nil, err
	}
	listArgs := append([]any{}, args...)
	limitPlaceholder := fmt.Sprintf("$%d", len(listArgs)+1)
	listArgs = append(listArgs, options.PageSize)
	offsetPlaceholder := fmt.Sprintf("$%d", len(listArgs)+1)
	listArgs = append(listArgs, (options.Page-1)*options.PageSize)
	rows, err := s.db.QueryContext(ctx, groupedQuery+` ORDER BY label,value LIMIT `+limitPlaceholder+` OFFSET `+offsetPlaceholder, listArgs...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]VerificationRecordOption, 0, options.PageSize)
	for rows.Next() {
		var item VerificationRecordOption
		if err := rows.Scan(&item.Value, &item.Label, &item.LabelEN, &item.Icon); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	pages := int((total + int64(options.PageSize) - 1) / int64(options.PageSize))
	if pages < 1 {
		pages = 1
	}
	return &VerificationRecordOptionPage{Items: items, Total: total, Page: options.Page, PageSize: options.PageSize, Pages: pages}, nil
}

func buildVerificationRecordWhere(options VerificationRecordListOptions, admin bool) (string, []any) {
	conditions := make([]string, 0, 6)
	args := make([]any, 0, 6)
	add := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}
	if !admin {
		add("user_id = $%d", options.UserID)
	}
	if options.Type != "" {
		add("verification_type = $%d", options.Type)
	}
	if options.Outcome != "" {
		add("outcome = $%d", options.Outcome)
	}
	if options.ServiceCode != "" {
		add("service_code = $%d", options.ServiceCode)
	}
	if options.Region != "" {
		add("region = $%d", options.Region)
	}
	if options.Keyword != "" {
		args = append(args, "%"+options.Keyword+"%")
		placeholder := len(args)
		conditions = append(conditions, fmt.Sprintf("(order_no ILIKE $%d OR target ILIKE $%d OR service_code ILIKE $%d OR user_email ILIKE $%d)", placeholder, placeholder, placeholder, placeholder))
	}
	if options.CreatedFrom != nil {
		add("created_at >= $%d", *options.CreatedFrom)
	}
	if options.CreatedTo != nil {
		add("created_at < $%d", *options.CreatedTo)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func isVerificationOutcome(value string) bool {
	switch value {
	case "processing", "success", "failed", "refunded", "cancelled", "expired":
		return true
	default:
		return false
	}
}

func verificationIntPtr(value int) *int             { return &value }
func verificationInt64Ptr(value int64) *int64       { return &value }
func verificationFloat64Ptr(value float64) *float64 { return &value }
