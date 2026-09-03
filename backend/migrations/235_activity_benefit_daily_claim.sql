ALTER TABLE activity_benefit_claims
    ADD COLUMN IF NOT EXISTS claim_day DATE;

-- Preserve every historical financial record. Only the first claim for each
-- legacy user/day is marked so the new uniqueness rule can be introduced even
-- if the previous configuration allowed multiple claims that day.
WITH ranked_claims AS (
    SELECT id,
           (created_at AT TIME ZONE 'Asia/Shanghai')::date AS local_claim_day,
           ROW_NUMBER() OVER (
               PARTITION BY activity_id, user_id, (created_at AT TIME ZONE 'Asia/Shanghai')::date
               ORDER BY created_at, id
           ) AS claim_rank
    FROM activity_benefit_claims
)
UPDATE activity_benefit_claims AS claims
SET claim_day = ranked_claims.local_claim_day
FROM ranked_claims
WHERE claims.id = ranked_claims.id
  AND claims.claim_day IS NULL
  AND ranked_claims.claim_rank = 1;

CREATE UNIQUE INDEX IF NOT EXISTS activity_benefit_claims_user_day_unique
    ON activity_benefit_claims(activity_id, user_id, claim_day)
    WHERE claim_day IS NOT NULL;

COMMENT ON COLUMN activity_benefit_claims.claim_day IS
    'Benefit claim calendar date in Asia/Shanghai. Legacy duplicate claims may remain NULL.';
