package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type SMSRentalServicePublicOption struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Icon       string  `json:"icon,omitempty"`
	Stock      int     `json:"stock"`
	RentDays   int     `json:"rent_days"`
	SalePrice  float64 `json:"sale_price"`
}

type SMSRentalServicePublicQuote struct {
	ID          string    `json:"quote_id"`
	ServiceCode string    `json:"service_code"`
	RentDays    int       `json:"rent_days"`
	SalePrice   float64   `json:"sale_price"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type SMSRentalRestorePublicQuote struct {
	ID          string    `json:"quote_id"`
	ServiceCode string    `json:"service_code"`
	CountryCode string    `json:"country_code"`
	PhoneNumber string    `json:"phone_number"`
	DurationDays int      `json:"duration_days"`
	SalePrice   float64   `json:"sale_price"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func smsUnknownGradeSalePrice(providerCost float64, pricing SMSPricingSettings) float64 {
	price := decimal.NewFromFloat(providerCost).
		Mul(decimal.NewFromFloat(pricing.CostMultiplier)).
		Mul(decimal.NewFromFloat(pricing.UnknownGradeMultiplier)).
		Add(decimal.NewFromFloat(pricing.UnknownGradeFixedMarkup + pricing.FixedMarkup))
	return quantize(price)
}

func (s *SMSService) loadSMSPVARentalForUser(ctx context.Context, userID int64, publicID string) (orderID int64, providerOrder, phone, countryCode, providerCode, base, credential string, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT o.id,o.provider_order_id,o.phone_number,co.iso2,p.code,p.base_url,p.credential_ref
		FROM sms_orders o
		JOIN sms_providers p ON p.id=o.provider_id
		JOIN sms_countries co ON co.id=o.country_id
		WHERE o.user_id=$1 AND o.public_id=$2::uuid AND o.product_type='rental'`,
		userID, strings.TrimSpace(publicID)).
		Scan(&orderID, &providerOrder, &phone, &countryCode, &providerCode, &base, &credential)
	if err != nil {
		return
	}
	if strings.ToLower(strings.TrimSpace(providerCode)) != "smspva" {
		err = errors.New("advanced rental service management is unavailable for this channel")
	}
	return
}

func (s *SMSService) RentalServiceOptions(ctx context.Context, userID int64, publicID string, rentDays int) ([]SMSRentalServicePublicOption, error) {
	if !s.Enabled(ctx) {
		return nil, ErrSMSFeatureDisabled
	}
	if rentDays <= 0 || rentDays > 366 {
		return nil, errors.New("invalid rental service duration")
	}
	orderID, _, _, countryCode, providerCode, base, credential, err := s.loadSMSPVARentalForUser(ctx, userID, publicID)
	if err != nil {
		return nil, err
	}
	var status string
	if err := s.db.QueryRowContext(ctx, `SELECT status FROM sms_orders WHERE id=$1`, orderID).Scan(&status); err != nil {
		return nil, err
	}
	if status != "active" {
		return nil, errors.New("rental order is not active")
	}
	provider := providerFor(providerCode, base, providerAPIKey(providerCode, credential, s.encryptor))
	smspva, ok := provider.(*smsPVAProvider)
	if !ok {
		return nil, errors.New("advanced rental service management is unavailable for this channel")
	}
	offers, err := smspva.RentalServiceOffers(ctx, countryCode, 1, "week")
	if err != nil {
		return nil, sanitizeProviderError(err)
	}
	existing := map[string]struct{}{}
	rows, err := s.db.QueryContext(ctx, `SELECT lower(COALESCE(NULLIF(provider_service_code,''),sv.code))
		FROM sms_order_services os JOIN sms_services sv ON sv.id=os.service_id
		WHERE os.order_id=$1 AND os.status='active'`, orderID)
	if err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var code string
			if rows.Scan(&code) == nil {
				existing[strings.ToLower(strings.TrimSpace(code))] = struct{}{}
			}
		}
	}
	pricing, err := s.GetPricingSettings(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SMSRentalServicePublicOption, 0, len(offers))
	for _, offer := range offers {
		if offer.Stock <= 0 {
			continue
		}
		if _, exists := existing[offer.Code]; exists {
			continue
		}
		name := offer.Name
		if name == "" {
			name = offer.Code
		}
		providerCost := offer.PriceDay * float64(rentDays)
		icon := ""
		if strings.TrimSpace(offer.ProviderIconPath) != "" {
			icon = "/api/v1/sms/providers/smspva/service-icons/" + offer.Code
		}
		out = append(out, SMSRentalServicePublicOption{
			Code:      offer.Code,
			Name:      name,
			Icon:      icon,
			Stock:     offer.Stock,
			RentDays:  rentDays,
			SalePrice: smsUnknownGradeSalePrice(providerCost, pricing),
		})
		if offer.ProviderIconPath != "" {
			_, _ = s.db.ExecContext(ctx, `UPDATE sms_provider_catalog_services c
				SET raw_metadata=COALESCE(c.raw_metadata,'{}'::jsonb) || jsonb_build_object('icon_path',$1)
				FROM sms_providers p
				WHERE c.provider_id=p.id AND p.code='smspva' AND lower(c.provider_service_code)=lower($2)`,
				offer.ProviderIconPath, offer.Code)
		}
	}
	return out, nil
}

func (s *SMSService) CreateRentalServiceQuote(ctx context.Context, userID int64, publicID, serviceCode string, rentDays int) (*SMSRentalServicePublicQuote, error) {
	serviceCode = strings.ToLower(strings.TrimSpace(serviceCode))
	if serviceCode == "" {
		return nil, errors.New("service code is required")
	}
	options, err := s.RentalServiceOptions(ctx, userID, publicID, rentDays)
	if err != nil {
		return nil, err
	}
	var selected *SMSRentalServicePublicOption
	for i := range options {
		if strings.EqualFold(options[i].Code, serviceCode) {
			selected = &options[i]
			break
		}
	}
	if selected == nil {
		return nil, ErrSMSProviderUnavailable
	}
	orderID, _, _, countryCode, providerCode, base, credential, err := s.loadSMSPVARentalForUser(ctx, userID, publicID)
	if err != nil {
		return nil, err
	}
	provider := providerFor(providerCode, base, providerAPIKey(providerCode, credential, s.encryptor))
	smspva, ok := provider.(*smsPVAProvider)
	if !ok {
		return nil, ErrSMSProviderUnavailable
	}
	offers, err := smspva.RentalServiceOffers(ctx, countryCode, 1, "week")
	if err != nil {
		return nil, sanitizeProviderError(err)
	}
	var providerCost float64
	for _, offer := range offers {
		if offer.Code == serviceCode && offer.Stock > 0 {
			providerCost = offer.PriceDay * float64(rentDays)
			break
		}
	}
	if providerCost <= 0 {
		return nil, ErrSMSProviderUnavailable
	}
	var serviceID int64
	if err := s.db.QueryRowContext(ctx, `INSERT INTO sms_services(code,name,category,enabled)
		VALUES($1,$2,'rental',TRUE)
		ON CONFLICT(code) DO UPDATE SET enabled=TRUE
		RETURNING id`, serviceCode, selected.Name).Scan(&serviceID); err != nil {
		return nil, err
	}
	id := randomID()
	expiresAt := time.Now().Add(30 * time.Second)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO sms_rental_service_quotes
		(id,user_id,order_id,service_id,provider_service_code,rent_days,provider_cost_snapshot,sale_price_snapshot,expires_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		id, userID, orderID, serviceID, serviceCode, rentDays, providerCost, selected.SalePrice, expiresAt); err != nil {
		return nil, err
	}
	return &SMSRentalServicePublicQuote{ID: id, ServiceCode: serviceCode, RentDays: rentDays, SalePrice: selected.SalePrice, ExpiresAt: expiresAt}, nil
}

func (s *SMSService) releaseRentalServiceCharge(ctx context.Context, chargeID, userID int64, amount float64, reason string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `UPDATE sms_rental_service_charges
		SET released_amount=reserved_amount,settlement_status='released',status='failed',last_error=$1,updated_at=NOW()
		WHERE id=$2 AND settlement_status='held'`, reason, chargeID)
	if err != nil {
		return err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, amount, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SMSService) AddRentalService(ctx context.Context, userID int64, publicID, quoteID, idempotencyKey string, expectedPrice *float64) (*SMSOrder, error) {
	if !s.Enabled(ctx) {
		return nil, ErrSMSFeatureDisabled
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" || len(idempotencyKey) > 128 {
		return nil, errors.New("Idempotency-Key is required")
	}
	var existingOrderID int64
	err := s.db.QueryRowContext(ctx, `SELECT order_id FROM sms_rental_service_charges WHERE user_id=$1 AND idempotency_key=$2`, userID, idempotencyKey).Scan(&existingOrderID)
	if err == nil {
		return s.GetOrder(ctx, userID, existingOrderID)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var orderID, serviceID int64
	var serviceCode string
	var rentDays int
	var providerCost, salePrice float64
	var providerOrder, phone, countryCode, providerCode, base, credential string
	err = s.db.QueryRowContext(ctx, `SELECT q.order_id,q.service_id,q.provider_service_code,q.rent_days,q.provider_cost_snapshot,q.sale_price_snapshot,
		o.provider_order_id,o.phone_number,co.iso2,p.code,p.base_url,p.credential_ref
		FROM sms_rental_service_quotes q
		JOIN sms_orders o ON o.id=q.order_id
		JOIN sms_countries co ON co.id=o.country_id
		JOIN sms_providers p ON p.id=o.provider_id
		WHERE q.id=$1 AND q.user_id=$2 AND o.public_id=$3::uuid AND q.consumed_at IS NULL AND q.expires_at>NOW() AND o.status='active'`,
		strings.TrimSpace(quoteID), userID, strings.TrimSpace(publicID)).
		Scan(&orderID, &serviceID, &serviceCode, &rentDays, &providerCost, &salePrice, &providerOrder, &phone, &countryCode, &providerCode, &base, &credential)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSMSQuoteExpired
	}
	if err != nil {
		return nil, err
	}
	if expectedPrice != nil && math.Abs(*expectedPrice-salePrice) > 0.00000001 {
		return nil, ErrSMSPriceChanged
	}
	if strings.ToLower(strings.TrimSpace(providerCode)) != "smspva" {
		return nil, ErrSMSProviderUnavailable
	}

	provider := providerFor(providerCode, base, providerAPIKey(providerCode, credential, s.encryptor))
	smspva, ok := provider.(*smsPVAProvider)
	if !ok {
		return nil, ErrSMSProviderUnavailable
	}
	offers, err := smspva.RentalServiceOffers(ctx, countryCode, 1, "week")
	if err != nil {
		return nil, sanitizeProviderError(err)
	}
	liveProviderCost := 0.0
	for _, offer := range offers {
		if offer.Code == serviceCode && offer.Stock > 0 {
			liveProviderCost = offer.PriceDay * float64(rentDays)
			break
		}
	}
	if liveProviderCost <= 0 {
		return nil, ErrSMSProviderUnavailable
	}
	if math.Abs(liveProviderCost-providerCost) > 0.00000001 {
		return nil, ErrSMSPriceChanged
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var chargeID int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO sms_rental_service_charges
		(user_id,order_id,service_id,quote_id,idempotency_key,reserved_amount,settlement_status,status)
		VALUES($1,$2,$3,$4,$5,$6,'held','pending')
		RETURNING id`, userID, orderID, serviceID, quoteID, idempotencyKey, salePrice).Scan(&chargeID); err != nil {
		return nil, err
	}
	res, err := tx.ExecContext(ctx, `UPDATE sms_rental_service_quotes SET consumed_at=NOW()
		WHERE id=$1 AND user_id=$2 AND consumed_at IS NULL AND expires_at>NOW()`, quoteID, userID)
	if err != nil {
		return nil, err
	}
	if affected, _ := res.RowsAffected(); affected != 1 {
		return nil, ErrSMSQuoteExpired
	}
	res, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance-$1,frozen_balance=COALESCE(frozen_balance,0)+$1,updated_at=NOW()
		WHERE id=$2 AND deleted_at IS NULL AND balance >= $1`, salePrice, userID)
	if err != nil {
		return nil, err
	}
	if affected, _ := res.RowsAffected(); affected != 1 {
		return nil, ErrSMSInsufficientBalance
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	callCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	added, addErr := smspva.AddRentalService(callCtx, providerOrder, phone, serviceCode, rentDays)
	cancel()
	if addErr != nil {
		if !isSMSPurchaseOutcomeAmbiguous(addErr) {
			if settleErr := s.releaseRentalServiceCharge(context.Background(), chargeID, userID, salePrice, providerErrorDiagnostic(addErr)); settleErr != nil {
				return nil, settleErr
			}
			return nil, sanitizeProviderError(addErr)
		}
		_, _ = s.db.ExecContext(context.Background(), `UPDATE sms_rental_service_charges SET status='reconciling',last_error=$1,updated_at=NOW() WHERE id=$2`, providerErrorDiagnostic(addErr), chargeID)
		return nil, ErrSMSProviderUnknown
	}
	if added == nil || strings.TrimSpace(added.ProviderOrderID) == "" {
		_, _ = s.db.ExecContext(context.Background(), `UPDATE sms_rental_service_charges SET status='reconciling',last_error='provider returned no child order id',updated_at=NOW() WHERE id=$1`, chargeID)
		return nil, ErrSMSProviderUnknown
	}

	finalCtx, finalCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer finalCancel()
	finalTx, err := s.db.BeginTx(finalCtx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = finalTx.Rollback() }()
	res, err = finalTx.ExecContext(finalCtx, `UPDATE sms_rental_service_charges
		SET provider_child_order_id=$1,captured_amount=reserved_amount,settlement_status='captured',status='completed',updated_at=NOW()
		WHERE id=$2 AND settlement_status='held'`, added.ProviderOrderID, chargeID)
	if err != nil {
		return nil, err
	}
	if affected, _ := res.RowsAffected(); affected == 1 {
		if _, err := finalTx.ExecContext(finalCtx, `UPDATE users SET frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, salePrice, userID); err != nil {
			return nil, err
		}
	}
	if _, err := finalTx.ExecContext(finalCtx, `INSERT INTO sms_order_services
		(order_id,service_id,provider_service_code,provider_order_id,provider_cost_snapshot,sale_price_snapshot,status,metadata)
		VALUES($1,$2,$3,$4,$5,$6,'active','{"added":true}'::jsonb)
		ON CONFLICT(order_id,service_id) DO UPDATE SET provider_order_id=EXCLUDED.provider_order_id,provider_cost_snapshot=EXCLUDED.provider_cost_snapshot,sale_price_snapshot=EXCLUDED.sale_price_snapshot,status='active',updated_at=NOW()`,
		orderID, serviceID, serviceCode, added.ProviderOrderID, providerCost, salePrice); err != nil {
		return nil, err
	}
	if _, err := finalTx.ExecContext(finalCtx, `INSERT INTO sms_order_events(order_id,event_type,actor,idempotency_key,payload)
		VALUES($1,'rental_add_service','user',$2,jsonb_build_object('service_code',$3,'provider_child_order_id',$4,'sale_price',$5))
		ON CONFLICT(order_id,event_type,idempotency_key) WHERE idempotency_key<>'' DO NOTHING`,
		orderID, idempotencyKey, serviceCode, added.ProviderOrderID, salePrice); err != nil {
		return nil, err
	}
	if err := finalTx.Commit(); err != nil {
		return nil, err
	}
	return s.GetOrder(ctx, userID, orderID)
}

func (s *SMSService) CreateRentalRestoreQuote(ctx context.Context, userID int64, publicID string) (*SMSRentalRestorePublicQuote, error) {
	if !s.Enabled(ctx) {
		return nil, ErrSMSFeatureDisabled
	}
	var sourceOrderID int64
	var providerHistoryID, localServiceCode, localCountryCode, providerCode, base, credential string
	if err := s.db.QueryRowContext(ctx, `SELECT o.id,o.provider_order_id,sv.code,co.iso2,p.code,p.base_url,p.credential_ref
		FROM sms_orders o
		JOIN sms_services sv ON sv.id=o.service_id
		JOIN sms_countries co ON co.id=o.country_id
		JOIN sms_providers p ON p.id=o.provider_id
		WHERE o.user_id=$1 AND o.public_id=$2::uuid AND o.product_type='rental'
		  AND o.status IN ('cancelled','expired','failed','refunded','completed')`,
		userID, strings.TrimSpace(publicID)).
		Scan(&sourceOrderID, &providerHistoryID, &localServiceCode, &localCountryCode, &providerCode, &base, &credential); err != nil {
		return nil, err
	}
	if strings.ToLower(strings.TrimSpace(providerCode)) != "smspva" || strings.TrimSpace(providerHistoryID) == "" {
		return nil, ErrSMSProviderUnavailable
	}
	provider := providerFor(providerCode, base, providerAPIKey(providerCode, credential, s.encryptor))
	advanced, ok := provider.(SMSRentalAdvancedProvider)
	if !ok {
		return nil, ErrSMSProviderUnavailable
	}

	canRestore := false
	for skip := 0; skip < 1000 && !canRestore; skip += 100 {
		history, err := advanced.RentalHistory(ctx, skip, 100)
		if err != nil {
			return nil, sanitizeProviderError(err)
		}
		if len(history) == 0 {
			break
		}
		for _, item := range history {
			if item.ProviderOrderID == providerHistoryID {
				canRestore = item.CanRestore
				break
			}
		}
		if len(history) < 100 {
			break
		}
	}
	if !canRestore {
		return nil, errors.New("provider does not allow this rental to be restored")
	}

	upstream, err := advanced.PrecalcRentalRestore(ctx, providerHistoryID)
	if err != nil {
		return nil, sanitizeProviderError(err)
	}
	if !strings.EqualFold(upstream.ServiceCode, localServiceCode) || !strings.EqualFold(upstream.CountryCode, localCountryCode) {
		return nil, errors.New("provider restore identity does not match the original rental")
	}
	pricing, err := s.GetPricingSettings(ctx)
	if err != nil {
		return nil, err
	}
	salePrice := smsUnknownGradeSalePrice(upstream.ProviderCost, pricing)
	id := randomID()
	expiresAt := time.Now().Add(30 * time.Second)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO sms_rental_restore_quotes
		(id,user_id,source_order_id,provider_history_order_id,provider_cost_snapshot,sale_price_snapshot,service_code,country_code,phone_number,duration_days,expires_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		id, userID, sourceOrderID, providerHistoryID, upstream.ProviderCost, salePrice, upstream.ServiceCode, upstream.CountryCode, upstream.PhoneNumber, upstream.ProlongTo, expiresAt); err != nil {
		return nil, err
	}
	return &SMSRentalRestorePublicQuote{
		ID: id, ServiceCode: upstream.ServiceCode, CountryCode: upstream.CountryCode,
		PhoneNumber: upstream.PhoneNumber, DurationDays: upstream.ProlongTo,
		SalePrice: salePrice, ExpiresAt: expiresAt,
	}, nil
}

func (s *SMSService) RestoreRentalOrder(ctx context.Context, userID int64, sourcePublicID, quoteID, idempotencyKey string, expectedPrice *float64) (*SMSOrder, error) {
	if !s.Enabled(ctx) {
		return nil, ErrSMSFeatureDisabled
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" || len(idempotencyKey) > 128 {
		return nil, errors.New("Idempotency-Key is required")
	}
	var existingID int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM sms_orders WHERE user_id=$1 AND idempotency_key=$2`, userID, idempotencyKey).Scan(&existingID)
	if err == nil {
		return s.GetOrder(ctx, userID, existingID)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var sourceOrderID, channelID, providerID, serviceID, countryID int64
	var providerHistoryID, serviceCode, countryCode, providerCode, base, credential string
	var providerCost, salePrice float64
	var durationDays int
	err = s.db.QueryRowContext(ctx, `SELECT q.source_order_id,o.channel_id,o.provider_id,o.service_id,o.country_id,
		q.provider_history_order_id,q.service_code,q.country_code,p.code,p.base_url,p.credential_ref,
		q.provider_cost_snapshot,q.sale_price_snapshot,q.duration_days
		FROM sms_rental_restore_quotes q
		JOIN sms_orders o ON o.id=q.source_order_id
		JOIN sms_providers p ON p.id=o.provider_id
		WHERE q.id=$1 AND q.user_id=$2 AND o.public_id=$3::uuid AND q.consumed_at IS NULL AND q.expires_at>NOW()`,
		strings.TrimSpace(quoteID), userID, strings.TrimSpace(sourcePublicID)).
		Scan(&sourceOrderID, &channelID, &providerID, &serviceID, &countryID, &providerHistoryID, &serviceCode, &countryCode, &providerCode, &base, &credential, &providerCost, &salePrice, &durationDays)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSMSQuoteExpired
	}
	if err != nil {
		return nil, err
	}
	if expectedPrice != nil && math.Abs(*expectedPrice-salePrice) > 0.00000001 {
		return nil, ErrSMSPriceChanged
	}
	provider := providerFor(providerCode, base, providerAPIKey(providerCode, credential, s.encryptor))
	advanced, ok := provider.(SMSRentalAdvancedProvider)
	if !ok {
		return nil, ErrSMSProviderUnavailable
	}
	live, err := advanced.PrecalcRentalRestore(ctx, providerHistoryID)
	if err != nil {
		return nil, sanitizeProviderError(err)
	}
	if math.Abs(live.ProviderCost-providerCost) > 0.00000001 || !strings.EqualFold(live.ServiceCode, serviceCode) || !strings.EqualFold(live.CountryCode, countryCode) {
		return nil, ErrSMSPriceChanged
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var orderID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO sms_orders
		(user_id,channel_id,provider_id,service_id,country_id,product_type,status,provider_cost_snapshot,sale_price_snapshot,idempotency_key,reserved_amount,settlement_status,reconciliation_action,reconcile_after,metadata)
		VALUES($1,$2,$3,$4,$5,'rental','reconciling',$6,$7,$8,$7,'held','purchase',NOW()+INTERVAL '5 seconds',jsonb_build_object('restored_from_order_id',$9,'provider_history_order_id',$10))
		RETURNING id`,
		userID, channelID, providerID, serviceID, countryID, providerCost, salePrice, idempotencyKey, sourceOrderID, providerHistoryID).Scan(&orderID)
	if err != nil {
		return nil, err
	}
	res, err := tx.ExecContext(ctx, `UPDATE sms_rental_restore_quotes SET consumed_at=NOW() WHERE id=$1 AND user_id=$2 AND consumed_at IS NULL AND expires_at>NOW()`, quoteID, userID)
	if err != nil {
		return nil, err
	}
	if affected, _ := res.RowsAffected(); affected != 1 {
		return nil, ErrSMSQuoteExpired
	}
	res, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance-$1,frozen_balance=COALESCE(frozen_balance,0)+$1,updated_at=NOW() WHERE id=$2 AND deleted_at IS NULL AND balance >= $1`, salePrice, userID)
	if err != nil {
		return nil, err
	}
	if affected, _ := res.RowsAffected(); affected != 1 {
		return nil, ErrSMSInsufficientBalance
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	callCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	restoredID, restoreErr := advanced.RestoreRental(callCtx, providerHistoryID)
	cancel()
	if restoreErr != nil {
		if !isSMSPurchaseOutcomeAmbiguous(restoreErr) {
			if settleErr := s.failSMSPurchase(context.Background(), orderID, userID, providerErrorDiagnostic(restoreErr)); settleErr != nil {
				return nil, settleErr
			}
			return nil, sanitizeProviderError(restoreErr)
		}
		_, _ = s.db.ExecContext(context.Background(), `UPDATE sms_orders SET status='reconciling',reconciliation_action='purchase',last_provider_error=$1,reconcile_after=NOW()+INTERVAL '5 seconds',updated_at=NOW() WHERE id=$2`, providerErrorDiagnostic(restoreErr), orderID)
		return s.GetOrder(context.Background(), userID, orderID)
	}
	if strings.TrimSpace(restoredID) == "" {
		_, _ = s.db.ExecContext(context.Background(), `UPDATE sms_orders SET status='reconciling',reconciliation_action='purchase',last_provider_error='provider restore returned no order id',reconcile_after=NOW()+INTERVAL '5 seconds',updated_at=NOW() WHERE id=$1`, orderID)
		return s.GetOrder(context.Background(), userID, orderID)
	}

	var phone string
	var expiresAt *time.Time
	if inspector, ok := provider.(SMSRentalOrderInspector); ok {
		if state, inspectErr := inspector.RentalOrder(context.Background(), restoredID); inspectErr == nil && state != nil {
			phone = state.PhoneNumber
			if state.Until > 0 {
				t := time.Unix(state.Until, 0)
				expiresAt = &t
			}
		}
	}
	if err := s.activateSMSOrder(context.Background(), orderID, userID, restoredID, phone, expiresAt, providerCost, ""); err != nil {
		_, _ = s.db.ExecContext(context.Background(), `UPDATE sms_orders SET provider_order_id=$1,status='reconciling',reconciliation_action='purchase',last_provider_error='provider restore succeeded; settlement retry required',reconcile_after=NOW()+INTERVAL '5 seconds',updated_at=NOW() WHERE id=$2`, restoredID, orderID)
		return nil, ErrSMSProviderUnknown
	}
	_, _ = s.db.ExecContext(context.Background(), `INSERT INTO sms_order_services
		(order_id,service_id,provider_service_code,provider_order_id,provider_cost_snapshot,sale_price_snapshot,status,metadata)
		VALUES($1,$2,$3,$4,$5,$6,'active',jsonb_build_object('primary',true,'restored_from_order_id',$7))
		ON CONFLICT(order_id,service_id) DO NOTHING`,
		orderID, serviceID, serviceCode, restoredID, providerCost, salePrice, sourceOrderID)
	if durationDays > 0 && expiresAt == nil {
		_, _ = s.db.ExecContext(context.Background(), `UPDATE sms_orders SET expires_at=NOW()+($1 * INTERVAL '1 day') WHERE id=$2 AND expires_at IS NULL`, durationDays, orderID)
	}
	return s.GetOrder(context.Background(), userID, orderID)
}

func (s *SMSService) cleanupExpiredRentalQuotes(ctx context.Context) {
	_, _ = s.db.ExecContext(ctx, `DELETE FROM sms_rental_service_quotes WHERE expires_at < NOW()-INTERVAL '1 hour'`)
	_, _ = s.db.ExecContext(ctx, `DELETE FROM sms_rental_restore_quotes WHERE expires_at < NOW()-INTERVAL '1 hour'`)
}

func formatRentalServiceChargeDiagnostic(serviceCode string, rentDays int, price float64) string {
	return fmt.Sprintf("service=%s days=%d sale_price=%.8f", serviceCode, rentDays, price)
}
