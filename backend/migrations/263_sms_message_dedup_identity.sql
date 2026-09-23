-- Provider-aware message identity. Text alone is not unique for rental
-- orders: the same text may arrive from different senders/services/timestamps.
ALTER TABLE sms_messages ADD COLUMN IF NOT EXISTS dedupe_hash TEXT;

-- Preserve existing rows without inventing provider metadata. The legacy
-- identity is deliberately bounded to the metadata already stored on each
-- row, matching persistSMSMessage's provider-aware identity exactly.
WITH legacy AS (
    SELECT id, order_id, message_text, sender, message_type, service_code, other_sms, provider_received_at,
           ROW_NUMBER() OVER (
               PARTITION BY order_id, message_text, sender, message_type, service_code, other_sms, provider_received_at
               ORDER BY id
           ) AS duplicate_rank
    FROM sms_messages
    WHERE dedupe_hash IS NULL OR dedupe_hash = ''
)
UPDATE sms_messages m
SET dedupe_hash = md5(concat_ws(
    chr(31),
    legacy.order_id::text,
    legacy.message_text,
    legacy.sender,
    legacy.message_type,
    legacy.service_code,
    legacy.other_sms::text,
    legacy.provider_received_at::text,
    CASE WHEN legacy.duplicate_rank = 1 THEN NULL ELSE legacy.id::text END
))
FROM legacy
WHERE m.id = legacy.id;

ALTER TABLE sms_messages ALTER COLUMN dedupe_hash SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_sms_messages_order_dedupe_hash
    ON sms_messages(order_id, dedupe_hash);
