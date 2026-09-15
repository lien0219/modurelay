package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type planCatalogRepository struct{ db *sql.DB }

func NewPlanCatalogRepository(db *sql.DB) service.PlanCatalogRepository {
	return &planCatalogRepository{db: db}
}

const planCatalogColumns = `id, name, subtitle, description, price::text, original_price::text,
currency, billing_period, badge, accent, group_name, provider, rate_multiplier::text,
daily_limit_usd::text, weekly_limit_usd::text, monthly_limit_usd::text, benefits::text,
payment_url, is_published, is_featured, sort_order, created_at, updated_at`

type planCatalogScanner interface{ Scan(...any) error }

func scanPlanCatalogItem(scanner planCatalogScanner) (*service.PlanCatalogItem, error) {
	var item service.PlanCatalogItem
	var original sql.NullString
	var benefitsJSON string
	var rate, daily, weekly, monthly sql.NullString
	if err := scanner.Scan(
		&item.ID, &item.Name, &item.Subtitle, &item.Description, &item.Price, &original,
		&item.Currency, &item.BillingPeriod, &item.Badge, &item.Accent, &item.GroupName, &item.Provider, &rate,
		&daily, &weekly, &monthly, &benefitsJSON,
		&item.PaymentURL, &item.IsPublished, &item.IsFeatured, &item.SortOrder,
		&item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if original.Valid {
		item.OriginalPrice = &original.String
	}
	if rate.Valid {
		item.RateMultiplier = rate.String
	}
	if daily.Valid {
		item.DailyLimitUSD = &daily.String
	}
	if weekly.Valid {
		item.WeeklyLimitUSD = &weekly.String
	}
	if monthly.Valid {
		item.MonthlyLimitUSD = &monthly.String
	}
	if err := json.Unmarshal([]byte(benefitsJSON), &item.Benefits); err != nil {
		return nil, fmt.Errorf("decode plan catalog benefits: %w", err)
	}
	if item.Benefits == nil {
		item.Benefits = []string{}
	}
	return &item, nil
}

func (r *planCatalogRepository) List(ctx context.Context, publishedOnly bool) ([]service.PlanCatalogItem, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil plan catalog repository")
	}
	query := `SELECT ` + planCatalogColumns + ` FROM plan_catalog_items`
	if publishedOnly {
		query += ` WHERE is_published = TRUE`
	}
	query += ` ORDER BY sort_order ASC, id ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list plan catalog items: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.PlanCatalogItem, 0)
	for rows.Next() {
		item, scanErr := scanPlanCatalogItem(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *planCatalogRepository) Get(ctx context.Context, id int64) (*service.PlanCatalogItem, error) {
	item, err := scanPlanCatalogItem(r.db.QueryRowContext(ctx, `SELECT `+planCatalogColumns+` FROM plan_catalog_items WHERE id=$1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrPlanCatalogNotFound
	}
	return item, err
}

func (r *planCatalogRepository) Create(ctx context.Context, input service.PlanCatalogInput) (*service.PlanCatalogItem, error) {
	benefits, err := json.Marshal(input.Benefits)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
INSERT INTO plan_catalog_items
(name, subtitle, description, price, original_price, currency, billing_period, badge, accent, group_name, provider, rate_multiplier, daily_limit_usd, weekly_limit_usd, monthly_limit_usd, benefits, payment_url, is_published, is_featured, sort_order)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16::jsonb,$17,$18,$19,$20)
RETURNING `+planCatalogColumns,
		input.Name, input.Subtitle, input.Description, input.Price, input.OriginalPrice,
		input.Currency, input.BillingPeriod, input.Badge, input.Accent, input.GroupName, input.Provider, input.RateMultiplier,
		input.DailyLimitUSD, input.WeeklyLimitUSD, input.MonthlyLimitUSD, string(benefits),
		input.PaymentURL, input.IsPublished, input.IsFeatured, input.SortOrder)
	return scanPlanCatalogItem(row)
}

func (r *planCatalogRepository) Update(ctx context.Context, id int64, input service.PlanCatalogInput) (*service.PlanCatalogItem, error) {
	benefits, err := json.Marshal(input.Benefits)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
UPDATE plan_catalog_items SET
name=$2, subtitle=$3, description=$4, price=$5, original_price=$6, currency=$7,
billing_period=$8, badge=$9, accent=$10, group_name=$11, provider=$12, rate_multiplier=$13,
daily_limit_usd=$14, weekly_limit_usd=$15, monthly_limit_usd=$16, benefits=$17::jsonb, payment_url=$18,
is_published=$19, is_featured=$20, sort_order=$21, updated_at=NOW()
WHERE id=$1 RETURNING `+planCatalogColumns,
		id, input.Name, input.Subtitle, input.Description, input.Price, input.OriginalPrice,
		input.Currency, input.BillingPeriod, input.Badge, input.Accent, input.GroupName, input.Provider, input.RateMultiplier,
		input.DailyLimitUSD, input.WeeklyLimitUSD, input.MonthlyLimitUSD, string(benefits),
		input.PaymentURL, input.IsPublished, input.IsFeatured, input.SortOrder)
	item, err := scanPlanCatalogItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrPlanCatalogNotFound
	}
	return item, err
}

func (r *planCatalogRepository) Delete(ctx context.Context, id int64) (bool, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM plan_catalog_items WHERE id=$1`, id)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
