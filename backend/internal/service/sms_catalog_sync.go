package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type SMSCatalogSyncStatus struct {
	Provider      string     `json:"provider"`
	Status        string     `json:"status"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
	NextSyncAt    *time.Time `json:"next_sync_at,omitempty"`
	DurationMS    int64      `json:"duration_ms"`
	ServiceCount  int        `json:"service_count"`
	CountryCount  int        `json:"country_count"`
	FailureReason string     `json:"failure_reason,omitempty"`
	Stale         bool       `json:"stale"`
	Source        string     `json:"source"`
}

func (s *SMSService) SyncProviderCatalog(ctx context.Context, code string) error {
	code = strings.ToLower(strings.TrimSpace(code))
	var id int64
	var base, cred string
	if err := s.db.QueryRowContext(ctx, `SELECT id,base_url,credential_ref FROM sms_providers WHERE code=$1 AND enabled`, code).Scan(&id, &base, &cred); err != nil {
		return err
	}

	started := time.Now()
	_, _ = s.db.ExecContext(ctx, `INSERT INTO sms_provider_catalog_syncs(provider_id,status,started_at,failure_reason) VALUES($1,'running',NOW(),'') ON CONFLICT(provider_id) DO UPDATE SET status='running',started_at=NOW(),failure_reason=''`, id)

	provider := providerFor(code, base, providerAPIKey(code, cred, s.encryptor))
	catalog, ok := provider.(SMSCatalogProvider)
	if !ok {
		return s.finishCatalogSync(ctx, id, started, fmt.Errorf("provider catalog unsupported"))
	}

	_, countries, err := catalog.Catalog(ctx)
	if err != nil {
		return s.finishCatalogSync(ctx, id, started, err)
	}

	var services []SMSSvcCatalogItem
	if p, ok := provider.(SMSServiceCatalogProvider); ok {
		services, err = p.CatalogServices(ctx, countries)
	} else {
		services, err = s.ListServices(ctx)
	}
	if err != nil {
		return s.finishCatalogSync(ctx, id, started, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return s.finishCatalogSync(ctx, id, started, err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, v := range services {
		providerCode := v.ProviderCode
		if providerCode == "" {
			providerCode = v.Code
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO sms_provider_catalog_services(provider_id,provider_service_code,provider_service_name,category,observed_at) VALUES($1,$2,$3,$4,NOW()) ON CONFLICT(provider_id,provider_service_code) DO UPDATE SET provider_service_name=EXCLUDED.provider_service_name,category=EXCLUDED.category,enabled=TRUE,observed_at=NOW()`, id, providerCode, v.Name, v.Category); err != nil {
			return s.finishCatalogSync(ctx, id, started, err)
		}
	}

	for _, v := range countries {
		providerCountryID := v.ProviderCode
		if providerCountryID == "" {
			providerCountryID = v.ISO2
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO sms_provider_catalog_countries(provider_id,provider_country_id,provider_country_code,iso2,name_zh,name_en,observed_at) VALUES($1,$2,$3,$4,$5,$6,NOW()) ON CONFLICT(provider_id,provider_country_id) DO UPDATE SET provider_country_code=EXCLUDED.provider_country_code,iso2=EXCLUDED.iso2,name_zh=EXCLUDED.name_zh,name_en=EXCLUDED.name_en,enabled=TRUE,observed_at=NOW()`, id, providerCountryID, v.ProviderCode, v.ISO2, v.NameZH, v.NameEN); err != nil {
			return s.finishCatalogSync(ctx, id, started, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return s.finishCatalogSync(ctx, id, started, err)
	}
	return s.finishCatalogSync(ctx, id, started, nil)
}

func (s *SMSService) finishCatalogSync(ctx context.Context, id int64, started time.Time, syncErr error) error {
	if syncErr != nil {
		_, _ = s.db.ExecContext(ctx, `UPDATE sms_provider_catalog_syncs SET status='failed',completed_at=NOW(),duration_ms=$2,failure_reason=$3,next_sync_at=NOW()+INTERVAL '6 hours' WHERE provider_id=$1`, id, time.Since(started).Milliseconds(), syncErr.Error())
		return syncErr
	}
	_, err := s.db.ExecContext(ctx, `UPDATE sms_provider_catalog_syncs SET status='succeeded',completed_at=NOW(),last_success_at=NOW(),duration_ms=$2,service_count=(SELECT COUNT(*) FROM sms_provider_catalog_services WHERE provider_id=$1 AND enabled),country_count=(SELECT COUNT(*) FROM sms_provider_catalog_countries WHERE provider_id=$1 AND enabled),failure_reason='',next_sync_at=NOW()+INTERVAL '6 hours' WHERE provider_id=$1`, id, time.Since(started).Milliseconds())
	return err
}

func (s *SMSService) CatalogSyncStatus(ctx context.Context, code string) (SMSCatalogSyncStatus, error) {
	var out SMSCatalogSyncStatus
	var id int64
	var completed, last, next sql.NullTime
	var stale float64
	err := s.db.QueryRowContext(ctx, `SELECT p.id,p.code,COALESCE(x.status,'never'),x.started_at,x.completed_at,x.last_success_at,x.next_sync_at,COALESCE(x.duration_ms,0),COALESCE(x.service_count,0),COALESCE(x.country_count,0),COALESCE(x.failure_reason,''),COALESCE(EXTRACT(EPOCH FROM x.stale_after),43200),COALESCE(x.source,'provider') FROM sms_providers p LEFT JOIN sms_provider_catalog_syncs x ON x.provider_id=p.id WHERE p.code=$1`, strings.ToLower(strings.TrimSpace(code))).Scan(&id, &out.Provider, &out.Status, &out.StartedAt, &completed, &last, &next, &out.DurationMS, &out.ServiceCount, &out.CountryCount, &out.FailureReason, &stale, &out.Source)
	if err != nil {
		return out, err
	}
	if completed.Valid {
		t := completed.Time
		out.CompletedAt = &t
	}
	if last.Valid {
		t := last.Time
		out.LastSuccessAt = &t
	}
	if next.Valid {
		t := next.Time
		out.NextSyncAt = &t
	}
	out.Stale = out.LastSuccessAt == nil || time.Since(*out.LastSuccessAt) > time.Duration(stale)*time.Second
	return out, nil
}

func (s *SMSService) SyncDueCatalogs(ctx context.Context) {
	s.catalogSyncMu.Lock()
	if time.Since(s.catalogSyncLastCheck) < time.Minute {
		s.catalogSyncMu.Unlock()
		return
	}
	s.catalogSyncLastCheck = time.Now()
	s.catalogSyncMu.Unlock()

	rows, err := s.db.QueryContext(ctx, "SELECT p.code FROM sms_providers p LEFT JOIN sms_provider_catalog_syncs x ON x.provider_id=p.id WHERE p.enabled AND (x.next_sync_at IS NULL OR x.next_sync_at<=NOW())")
	if err != nil {
		return
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var code string
		if rows.Scan(&code) == nil {
			syncCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			_ = s.SyncProviderCatalog(syncCtx, code)
			cancel()
		}
	}
}
