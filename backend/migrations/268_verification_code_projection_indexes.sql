-- Latest verification-code projections used by user order lists.
CREATE INDEX IF NOT EXISTS idx_sms_messages_latest_verification_code
    ON sms_messages(order_id, received_at DESC, id DESC)
    WHERE BTRIM(verification_code) <> '';

CREATE INDEX IF NOT EXISTS idx_email_messages_latest_verification_code
    ON email_messages(email_order_id, received_at DESC, id DESC)
    WHERE BTRIM(verification_code) <> '';
