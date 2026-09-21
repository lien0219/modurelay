package service

import (
	"context"
	"database/sql"
	"strings"
)

// mergeRentalServiceStatuses polls provider child-order ids created when an
// existing SMSPVA rental receives an additional service. Initial create_multi
// rentals share one provider order id and therefore do not produce extra polls.
func (s *SMSService) mergeRentalServiceStatuses(ctx context.Context, provider SMSProvider, orderID int64, primaryProviderOrder string, base *SMSStatusResult) (*SMSStatusResult, error) {
	if base == nil {
		base = &SMSStatusResult{Status: "active"}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT os.provider_order_id,COALESCE(NULLIF(os.provider_service_code,''),sv.code)
		FROM sms_order_services os
		JOIN sms_services sv ON sv.id=os.service_id
		WHERE os.order_id=$1 AND os.status='active' AND os.provider_order_id<>'' AND os.provider_order_id<>$2
		ORDER BY os.created_at,os.service_id`, orderID, strings.TrimSpace(primaryProviderOrder))
	if err != nil {
		if err == sql.ErrNoRows {
			return base, nil
		}
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	baseMetadata := make([]map[string]any, 0, len(base.Messages))
	for index := range base.Messages {
		sender, receivedAt, messageType, serviceCode, otherSMS, ok := smsMessageMetadata(base.Metadata, index)
		meta := map[string]any{"message_type": messageType, "service_code": serviceCode, "other_sms": otherSMS}
		if ok {
			if sender != "" {
				meta["sender"] = sender
			}
			if receivedAt != nil {
				meta["provider_received_at"] = *receivedAt
			}
		}
		baseMetadata = append(baseMetadata, meta)
	}

	seenOrders := map[string]struct{}{}
	for rows.Next() {
		var childOrderID, serviceCode string
		if err := rows.Scan(&childOrderID, &serviceCode); err != nil {
			return nil, err
		}
		childOrderID = strings.TrimSpace(childOrderID)
		if childOrderID == "" {
			continue
		}
		if _, exists := seenOrders[childOrderID]; exists {
			continue
		}
		seenOrders[childOrderID] = struct{}{}

		child, err := provider.GetRentalStatus(ctx, childOrderID)
		if err != nil || child == nil {
			// One add-on service being temporarily unavailable must not erase a
			// healthy primary rental. The normal worker will poll it again later.
			continue
		}
		for index, message := range child.Messages {
			message = strings.TrimSpace(message)
			if message == "" {
				continue
			}
			base.Messages = append(base.Messages, message)
			sender, receivedAt, messageType, upstreamService, otherSMS, ok := smsMessageMetadata(child.Metadata, index)
			if strings.TrimSpace(upstreamService) == "" {
				upstreamService = serviceCode
			}
			meta := map[string]any{"message_type": messageType, "service_code": upstreamService, "other_sms": otherSMS}
			if !ok {
				meta["message_type"] = "service"
				meta["other_sms"] = false
			}
			if sender != "" {
				meta["sender"] = sender
			}
			if receivedAt != nil {
				meta["provider_received_at"] = *receivedAt
			}
			baseMetadata = append(baseMetadata, meta)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(baseMetadata) > 0 {
		if base.Metadata == nil {
			base.Metadata = map[string]any{}
		}
		base.Metadata["messages"] = baseMetadata
	}
	return base, nil
}
