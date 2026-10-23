-- Keep PDF contents separate: invoice lists and exports only read metadata.
ALTER TABLE invoice_applications ADD COLUMN IF NOT EXISTS contact VARCHAR(200) NOT NULL DEFAULT '';
ALTER TABLE invoice_applications ADD COLUMN IF NOT EXISTS requested_issue_date DATE;

CREATE TABLE IF NOT EXISTS invoice_documents (
    invoice_id BIGINT PRIMARY KEY REFERENCES invoice_applications(id) ON DELETE CASCADE,
    filename VARCHAR(200) NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 10485760),
    sha256 CHAR(64) NOT NULL,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    content BYTEA NOT NULL,
    CHECK (octet_length(content) = size_bytes)
);
