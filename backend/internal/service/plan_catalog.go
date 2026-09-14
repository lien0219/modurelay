package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

var ErrPlanCatalogNotFound = infraerrors.NotFound("PLAN_CATALOG_NOT_FOUND", "plan catalog item not found")

type PlanCatalogItem struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Subtitle      string    `json:"subtitle"`
	Description   string    `json:"description"`
	Price         string    `json:"price"`
	OriginalPrice *string   `json:"original_price"`
	Currency      string    `json:"currency"`
	BillingPeriod string    `json:"billing_period"`
	Badge         string    `json:"badge"`
	Accent        string    `json:"accent"`
	Benefits      []string  `json:"benefits"`
	PaymentURL    string    `json:"payment_url"`
	IsPublished   bool      `json:"is_published"`
	IsFeatured    bool      `json:"is_featured"`
	SortOrder     int       `json:"sort_order"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type PlanCatalogInput struct {
	Name          string   `json:"name"`
	Subtitle      string   `json:"subtitle"`
	Description   string   `json:"description"`
	Price         string   `json:"price"`
	OriginalPrice *string  `json:"original_price"`
	Currency      string   `json:"currency"`
	BillingPeriod string   `json:"billing_period"`
	Badge         string   `json:"badge"`
	Accent        string   `json:"accent"`
	Benefits      []string `json:"benefits"`
	PaymentURL    string   `json:"payment_url"`
	IsPublished   bool     `json:"is_published"`
	IsFeatured    bool     `json:"is_featured"`
	SortOrder     int      `json:"sort_order"`
}

// UnmarshalJSON accepts both JSON strings and JSON numbers for monetary fields.
// Browser number inputs commonly submit 10 rather than "10", while the
// service keeps the decimal text representation to preserve precision.
func (input *PlanCatalogInput) UnmarshalJSON(data []byte) error {
	type alias PlanCatalogInput
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}

	priceRaw := fields["price"]
	originalPriceRaw := fields["original_price"]
	delete(fields, "price")
	delete(fields, "original_price")
	normalized, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	var decoded alias
	if err := json.Unmarshal(normalized, &decoded); err != nil {
		return err
	}

	if len(priceRaw) > 0 {
		decoded.Price, err = decodePlanCatalogDecimal(priceRaw)
		if err != nil {
			return err
		}
	}
	if len(originalPriceRaw) > 0 && string(originalPriceRaw) != "null" {
		value, decodeErr := decodePlanCatalogDecimal(originalPriceRaw)
		if decodeErr != nil {
			return decodeErr
		}
		decoded.OriginalPrice = &value
	}

	*input = PlanCatalogInput(decoded)
	return nil
}

func decodePlanCatalogDecimal(raw json.RawMessage) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return value, nil
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return "", err
	}
	return number.String(), nil
}

type PlanCatalogRepository interface {
	List(ctx context.Context, publishedOnly bool) ([]PlanCatalogItem, error)
	Get(ctx context.Context, id int64) (*PlanCatalogItem, error)
	Create(ctx context.Context, input PlanCatalogInput) (*PlanCatalogItem, error)
	Update(ctx context.Context, id int64, input PlanCatalogInput) (*PlanCatalogItem, error)
	Delete(ctx context.Context, id int64) (bool, error)
}

type PlanCatalogService struct {
	repo        PlanCatalogRepository
	settingRepo SettingRepository
}

func NewPlanCatalogService(repo PlanCatalogRepository, settingRepo SettingRepository) *PlanCatalogService {
	return &PlanCatalogService{repo: repo, settingRepo: settingRepo}
}

func (s *PlanCatalogService) ListPublished(ctx context.Context) ([]PlanCatalogItem, error) {
	if s == nil || s.repo == nil || s.settingRepo == nil {
		return nil, errors.New("plan catalog service is unavailable")
	}
	enabled, err := s.settingRepo.GetValue(ctx, SettingKeyPlanCatalogEnabled)
	if err != nil && !errors.Is(err, ErrSettingNotFound) {
		return nil, err
	}
	if enabled != "true" {
		return nil, infraerrors.NotFound("PLAN_CATALOG_DISABLED", "plan catalog is disabled")
	}
	return s.repo.List(ctx, true)
}

func (s *PlanCatalogService) ListAdmin(ctx context.Context) ([]PlanCatalogItem, error) {
	return s.repo.List(ctx, false)
}

func (s *PlanCatalogService) Create(ctx context.Context, input PlanCatalogInput) (*PlanCatalogItem, error) {
	normalized, err := validatePlanCatalogInput(input)
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, normalized)
}

func (s *PlanCatalogService) Update(ctx context.Context, id int64, input PlanCatalogInput) (*PlanCatalogItem, error) {
	if id <= 0 {
		return nil, infraerrors.BadRequest("PLAN_CATALOG_INVALID_ID", "invalid plan catalog item id")
	}
	normalized, err := validatePlanCatalogInput(input)
	if err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, id, normalized)
}

func (s *PlanCatalogService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return infraerrors.BadRequest("PLAN_CATALOG_INVALID_ID", "invalid plan catalog item id")
	}
	deleted, err := s.repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrPlanCatalogNotFound
	}
	return nil
}

func validatePlanCatalogInput(input PlanCatalogInput) (PlanCatalogInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Subtitle = strings.TrimSpace(input.Subtitle)
	input.Description = strings.TrimSpace(input.Description)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.BillingPeriod = strings.ToLower(strings.TrimSpace(input.BillingPeriod))
	input.Badge = strings.TrimSpace(input.Badge)
	input.Accent = strings.ToLower(strings.TrimSpace(input.Accent))
	input.PaymentURL = strings.TrimSpace(input.PaymentURL)
	if input.Name == "" || len([]rune(input.Name)) > 80 {
		return input, infraerrors.BadRequest("PLAN_CATALOG_INVALID_NAME", "name is required and must not exceed 80 characters")
	}
	if len([]rune(input.Subtitle)) > 160 || len([]rune(input.Description)) > 600 || len([]rune(input.Badge)) > 40 {
		return input, infraerrors.BadRequest("PLAN_CATALOG_TEXT_TOO_LONG", "plan catalog text exceeds the allowed length")
	}
	price, err := decimal.NewFromString(strings.TrimSpace(input.Price))
	if err != nil || price.IsNegative() || price.Exponent() < -4 {
		return input, infraerrors.BadRequest("PLAN_CATALOG_INVALID_PRICE", "price must be a non-negative number with at most 4 decimal places")
	}
	input.Price = price.String()
	if input.OriginalPrice != nil && strings.TrimSpace(*input.OriginalPrice) != "" {
		original, parseErr := decimal.NewFromString(strings.TrimSpace(*input.OriginalPrice))
		if parseErr != nil || original.LessThan(price) || original.Exponent() < -4 {
			return input, infraerrors.BadRequest("PLAN_CATALOG_INVALID_ORIGINAL_PRICE", "original price must be greater than or equal to price")
		}
		value := original.String()
		input.OriginalPrice = &value
	} else {
		input.OriginalPrice = nil
	}
	if !containsString([]string{"CNY", "USD", "EUR", "HKD"}, input.Currency) {
		return input, infraerrors.BadRequest("PLAN_CATALOG_INVALID_CURRENCY", "unsupported currency")
	}
	if !containsString([]string{"monthly", "quarterly", "yearly", "one_time", "custom"}, input.BillingPeriod) {
		return input, infraerrors.BadRequest("PLAN_CATALOG_INVALID_PERIOD", "unsupported billing period")
	}
	if input.Accent == "" {
		input.Accent = "indigo"
	}
	if !containsString([]string{"indigo", "emerald", "amber", "rose", "slate"}, input.Accent) {
		return input, infraerrors.BadRequest("PLAN_CATALOG_INVALID_ACCENT", "unsupported accent")
	}
	if input.SortOrder < -100000 || input.SortOrder > 100000 {
		return input, infraerrors.BadRequest("PLAN_CATALOG_INVALID_SORT", "sort order is out of range")
	}
	if len(input.Benefits) == 0 || len(input.Benefits) > 30 {
		return input, infraerrors.BadRequest("PLAN_CATALOG_INVALID_BENEFITS", "provide between 1 and 30 benefits")
	}
	benefits := make([]string, 0, len(input.Benefits))
	for _, benefit := range input.Benefits {
		benefit = strings.TrimSpace(benefit)
		if benefit == "" || len([]rune(benefit)) > 160 {
			return input, infraerrors.BadRequest("PLAN_CATALOG_INVALID_BENEFIT", "each benefit must be 1 to 160 characters")
		}
		benefits = append(benefits, benefit)
	}
	input.Benefits = benefits
	if !isSafePlanPaymentURL(input.PaymentURL) {
		return input, infraerrors.BadRequest("PLAN_CATALOG_INVALID_PAYMENT_URL", "payment URL must be an absolute HTTP or HTTPS URL")
	}
	return input, nil
}

func isSafePlanPaymentURL(raw string) bool {
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Host == "" || u.User != nil {
		return false
	}
	return u.Scheme == "https" || u.Scheme == "http"
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
