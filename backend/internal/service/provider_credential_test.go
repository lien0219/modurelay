package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/config"
)

type providerCredentialTestEncryptor struct{}

func (providerCredentialTestEncryptor) Encrypt(value string) (string, error) {
	return "cipher:" + value, nil
}

func (providerCredentialTestEncryptor) Decrypt(value string) (string, error) {
	if len(value) < len("cipher:") || value[:len("cipher:")] != "cipher:" {
		return "", sql.ErrNoRows
	}
	return value[len("cipher:"):], nil
}

func fixedProviderCredentialConfig(configured bool) *config.Config {
	return &config.Config{Totp: config.TotpConfig{EncryptionKeyConfigured: configured}}
}

func TestSMSAdminProviderCredentialRejectsEphemeralEncryptionKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`SELECT code,credential_ref FROM sms_providers WHERE id=\$1`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"code", "credential_ref"}).AddRow("5sim", ""))
	svc := NewSMSService(db, NewSettingService(nil, fixedProviderCredentialConfig(false)), providerCredentialTestEncryptor{})

	err = svc.UpdateProvider(context.Background(), 1, false, "", "provider-secret")
	if err != ErrProviderCredentialEncryptionKeyNotConfigured {
		t.Fatalf("UpdateProvider error = %v, want fixed-key guard", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSMSAdminProviderCredentialEncryptsWithFixedKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`SELECT code,credential_ref FROM sms_providers WHERE id=\$1`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"code", "credential_ref"}).AddRow("5sim", ""))
	mock.ExpectExec(`UPDATE sms_providers SET enabled=\$1,base_url=COALESCE\(NULLIF\(\$2,''\),base_url\),credential_ref=COALESCE\(NULLIF\(\$3,''\),credential_ref\),updated_at=NOW\(\) WHERE id=\$4`).
		WithArgs(false, "", "enc:cipher:provider-secret", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	svc := NewSMSService(db, NewSettingService(nil, fixedProviderCredentialConfig(true)), providerCredentialTestEncryptor{})

	if err := svc.UpdateProvider(context.Background(), 1, false, "", "provider-secret"); err != nil {
		t.Fatalf("UpdateProvider: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSMSAdminProviderCredentialKeepsEnvReferenceWithoutFixedKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`SELECT code,credential_ref FROM sms_providers WHERE id=\$1`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"code", "credential_ref"}).AddRow("5sim", ""))
	mock.ExpectExec(`UPDATE sms_providers SET enabled=\$1,base_url=COALESCE\(NULLIF\(\$2,''\),base_url\),credential_ref=COALESCE\(NULLIF\(\$3,''\),credential_ref\),updated_at=NOW\(\) WHERE id=\$4`).
		WithArgs(false, "", "env:FIVE_SIM_API_KEY", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	svc := NewSMSService(db, NewSettingService(nil, fixedProviderCredentialConfig(false)), providerCredentialTestEncryptor{})

	if err := svc.UpdateProvider(context.Background(), 1, false, "", "env:FIVE_SIM_API_KEY"); err != nil {
		t.Fatalf("UpdateProvider env reference: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEmailAdminProviderCredentialRejectsEphemeralEncryptionKey(t *testing.T) {
	svc := NewEmailVerificationService(nil, nil, providerCredentialTestEncryptor{}, fixedProviderCredentialConfig(false))
	if err := svc.AdminUpdateProviderConfig(context.Background(), 1, false, "", "provider-secret", nil); err != ErrProviderCredentialEncryptionKeyNotConfigured {
		t.Fatalf("AdminUpdateProviderConfig error = %v, want fixed-key guard", err)
	}
}

func TestEmailAdminProviderCredentialEncryptsWithFixedKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectExec(`UPDATE email_providers SET enabled=\$1,base_url=COALESCE\(NULLIF\(\$2,''\),base_url\),credential_ref=COALESCE\(NULLIF\(\$3,''\),credential_ref\),updated_at=NOW\(\) WHERE id=\$4`).
		WithArgs(false, "", "enc:cipher:provider-secret", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	svc := NewEmailVerificationService(db, nil, providerCredentialTestEncryptor{}, fixedProviderCredentialConfig(true))

	if err := svc.AdminUpdateProviderConfig(context.Background(), 1, false, "", "provider-secret", nil); err != nil {
		t.Fatalf("AdminUpdateProviderConfig: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEmailAdminProviderCredentialKeepsEnvReferenceWithoutFixedKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectExec(`UPDATE email_providers SET enabled=\$1,base_url=COALESCE\(NULLIF\(\$2,''\),base_url\),credential_ref=COALESCE\(NULLIF\(\$3,''\),credential_ref\),updated_at=NOW\(\) WHERE id=\$4`).
		WithArgs(false, "", "env:EMAIL_PROVIDER_API_KEY", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	svc := NewEmailVerificationService(db, nil, providerCredentialTestEncryptor{}, fixedProviderCredentialConfig(false))

	if err := svc.AdminUpdateProviderConfig(context.Background(), 1, false, "", "env:EMAIL_PROVIDER_API_KEY", nil); err != nil {
		t.Fatalf("AdminUpdateProviderConfig env reference: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEmailAdminProviderUpdateRejectsUnknownProvider(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectExec(`UPDATE email_providers SET enabled=\$1,base_url=COALESCE\(NULLIF\(\$2,''\),base_url\),credential_ref=COALESCE\(NULLIF\(\$3,''\),credential_ref\),updated_at=NOW\(\) WHERE id=\$4`).
		WithArgs(false, "", "env:EMAIL_PROVIDER_API_KEY", int64(404)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	svc := NewEmailVerificationService(db, nil, providerCredentialTestEncryptor{}, fixedProviderCredentialConfig(false))

	if err := svc.AdminUpdateProviderConfig(context.Background(), 404, false, "", "env:EMAIL_PROVIDER_API_KEY", nil); err != ErrEmailProviderNotFound {
		t.Fatalf("AdminUpdateProviderConfig error = %v, want ErrEmailProviderNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
