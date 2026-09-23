-- Harden paid email verification settlement.
--
-- Paid mailbox providers used by channel 2 do not expose a verifiable
-- provider-refund operation. New paid orders therefore use a
-- no-refund-after-inbox-delivery policy. Historical order snapshots are not
-- rewritten; runtime safety logic protects legacy paid orders by sale price.

UPDATE email_channels
SET refund_policy='no_refund_after_inbox_delivery',
    updated_at=NOW()
WHERE code='email_channel_2'
  AND sale_price > 0;
