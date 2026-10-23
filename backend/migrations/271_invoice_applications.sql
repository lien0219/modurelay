-- Invoice amounts are integer CNY cents, independent of the USD wallet balance.
CREATE TABLE invoice_applications (
    id BIGSERIAL PRIMARY KEY,
    application_no VARCHAR(40) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id),
    user_email VARCHAR(255) NOT NULL,
    user_name VARCHAR(100) NOT NULL DEFAULT '',
    amount_cents BIGINT NOT NULL CHECK (amount_cents >= 30000 AND amount_cents <= 9007199254740991),
    invoice_type VARCHAR(20) NOT NULL CHECK (invoice_type IN ('vat_normal', 'vat_special')),
    title_type VARCHAR(20) NOT NULL CHECK (title_type IN ('personal', 'enterprise')),
    title VARCHAR(200) NOT NULL,
    tax_number VARCHAR(32) NOT NULL DEFAULT '',
    email VARCHAR(254) NOT NULL,
    address VARCHAR(500) NOT NULL DEFAULT '',
    phone VARCHAR(50) NOT NULL DEFAULT '',
    bank_name VARCHAR(200) NOT NULL DEFAULT '',
    bank_account VARCHAR(100) NOT NULL DEFAULT '',
    remark VARCHAR(1000) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'issued', 'rejected', 'cancelled')),
    admin_remark VARCHAR(1000) NOT NULL DEFAULT '',
    invoice_number VARCHAR(100) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    CHECK (title_type <> 'enterprise' OR tax_number <> ''),
    CHECK (invoice_type <> 'vat_special' OR (title_type = 'enterprise' AND address <> '' AND phone <> '' AND bank_name <> '' AND bank_account <> ''))
);
CREATE INDEX idx_invoice_applications_user_created ON invoice_applications(user_id, created_at DESC, id DESC);
CREATE INDEX idx_invoice_applications_status_created ON invoice_applications(status, created_at DESC, id DESC);
INSERT INTO settings (key, value, updated_at) VALUES ('invoice_enabled', 'false', NOW()) ON CONFLICT (key) DO NOTHING;
