package repository

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"slices"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type enterpriseIdentityRepository struct {
	db        *sql.DB
	encryptor service.SecretEncryptor
}

func NewEnterpriseIdentityRepository(db *sql.DB, encryptor service.SecretEncryptor) service.EnterpriseIdentityRepository {
	return &enterpriseIdentityRepository{db: db, encryptor: encryptor}
}

func (r *enterpriseIdentityRepository) beginScoped(ctx context.Context, workspaceID, actorID int64, permission string) (*sql.Tx, *service.WorkspaceAccess, error) {
	tx, access, err := (&workspaceWebhookRepository{db: r.db}).beginScoped(ctx, actorID, workspaceID, permission)
	if err != nil {
		return nil, nil, err
	}
	if access.Workspace.Type != service.WorkspaceTypeOrganization {
		_ = tx.Rollback()
		return nil, nil, service.ErrWorkspaceForbidden
	}
	return tx, access, nil
}

// Audit, event and dispatch outbox are part of the same business transaction.
// Callers pass only bounded IDs and status codes, never claims or credentials.
func appendIdentityMutation(ctx context.Context, tx *sql.Tx, workspaceID, actorID int64, action, eventType, target string, targetID int64, data service.DomainEventData) error {
	if err := appendWorkspaceAudit(ctx, tx, workspaceID, actorID, nil, action, target, targetID, map[string]any(data)); err != nil {
		return err
	}
	if eventType == "" {
		return nil
	}
	event, err := service.NewDomainEvent(eventType, workspaceID, 0, actorID, target, fmt.Sprint(targetID), data)
	if err != nil {
		return err
	}
	return insertDomainEventTx(ctx, tx, event, "")
}

func enterpriseIdentityError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrWorkspaceNotFound
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch pqErr.Code {
		case "23505":
			if strings.Contains(pqErr.Constraint, "workspace_domains") || strings.Contains(pqErr.Constraint, "normalized_domain") {
				return service.ErrDomainAlreadyClaimed.WithCause(err)
			}
			return service.ErrWorkspaceConflict.WithCause(err)
		case "23503", "23514", "40001", "40P01":
			return service.ErrWorkspaceConflict.WithCause(err)
		}
	}
	return err
}

const enterpriseDomainColumns = `id,workspace_id,domain,normalized_domain,status,verification_method,verified_at,last_checked_at,last_error_code,created_by_user_id,created_at,updated_at`

func scanEnterpriseDomain(scanner interface{ Scan(...any) error }) (*service.EnterpriseDomain, error) {
	d := &service.EnterpriseDomain{}
	var lastError sql.NullString
	err := scanner.Scan(&d.ID, &d.WorkspaceID, &d.Domain, &d.NormalizedDomain, &d.Status, &d.VerificationMethod, &d.VerifiedAt, &d.LastCheckedAt, &lastError, &d.CreatedByUserID, &d.CreatedAt, &d.UpdatedAt)
	if lastError.Valid {
		d.LastErrorCode = lastError.String
	}
	return d, enterpriseIdentityError(err)
}

func (r *enterpriseIdentityRepository) ListDomains(ctx context.Context, workspaceID int64, p pagination.PaginationParams) ([]service.EnterpriseDomain, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM workspace_domains WHERE workspace_id=$1`, workspaceID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+enterpriseDomainColumns+` FROM workspace_domains WHERE workspace_id=$1 ORDER BY id DESC LIMIT $2 OFFSET $3`, workspaceID, p.Limit(), p.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.EnterpriseDomain, 0)
	for rows.Next() {
		item, scanErr := scanEnterpriseDomain(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

func (r *enterpriseIdentityRepository) CreateDomain(ctx context.Context, workspaceID, actorID int64, canonical, _ string, tokenHash []byte) (*service.EnterpriseDomain, error) {
	if len(tokenHash) != 32 {
		return nil, service.ErrEnterpriseIdentityInvalid
	}
	tx, _, err := r.beginScoped(ctx, workspaceID, actorID, "identity.manage")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	item, err := scanEnterpriseDomain(tx.QueryRowContext(ctx, `INSERT INTO workspace_domains(workspace_id,domain,normalized_domain,verification_token_hash,created_by_user_id) VALUES($1,$2,$2,$3,$4) ON CONFLICT(workspace_id,normalized_domain) DO UPDATE SET status='pending',verification_token_hash=EXCLUDED.verification_token_hash,verified_at=NULL,last_checked_at=NULL,last_error_code=NULL,updated_at=now() WHERE workspace_domains.status='revoked' RETURNING `+enterpriseDomainColumns, workspaceID, canonical, tokenHash, actorID))
	if errors.Is(err, service.ErrWorkspaceNotFound) {
		return nil, service.ErrDomainAlreadyClaimed
	}
	if err != nil {
		return nil, err
	}
	if err = appendIdentityMutation(ctx, tx, workspaceID, actorID, "identity.domain_created", service.EventWorkspaceDomainCreated, "domain", item.ID, service.DomainEventData{"status": item.Status}); err != nil {
		return nil, err
	}
	return item, enterpriseIdentityError(tx.Commit())
}

func (r *enterpriseIdentityRepository) GetDomain(ctx context.Context, workspaceID, actorID int64, domainID int64) (*service.EnterpriseDomain, []byte, error) {
	tx, _, err := r.beginScoped(ctx, workspaceID, actorID, "identity.read")
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	d := &service.EnterpriseDomain{}
	var lastError sql.NullString
	var tokenHash []byte
	err = tx.QueryRowContext(ctx, `SELECT `+enterpriseDomainColumns+`,verification_token_hash FROM workspace_domains WHERE workspace_id=$1 AND id=$2`, workspaceID, domainID).Scan(&d.ID, &d.WorkspaceID, &d.Domain, &d.NormalizedDomain, &d.Status, &d.VerificationMethod, &d.VerifiedAt, &d.LastCheckedAt, &lastError, &d.CreatedByUserID, &d.CreatedAt, &d.UpdatedAt, &tokenHash)
	if lastError.Valid {
		d.LastErrorCode = lastError.String
	}
	if err != nil {
		return nil, nil, enterpriseIdentityError(err)
	}
	return d, tokenHash, enterpriseIdentityError(tx.Commit())
}

func (r *enterpriseIdentityRepository) RegenerateDomain(ctx context.Context, workspaceID, actorID int64, domainID int64, tokenHash []byte) (*service.EnterpriseDomain, error) {
	if len(tokenHash) != 32 {
		return nil, service.ErrEnterpriseIdentityInvalid
	}
	tx, _, err := r.beginScoped(ctx, workspaceID, actorID, "identity.manage")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	item, err := scanEnterpriseDomain(tx.QueryRowContext(ctx, `UPDATE workspace_domains SET verification_token_hash=$3,status='pending',verified_at=NULL,last_checked_at=NULL,last_error_code=NULL,updated_at=now() WHERE workspace_id=$1 AND id=$2 AND status<>'revoked' RETURNING `+enterpriseDomainColumns, workspaceID, domainID, tokenHash))
	if err != nil {
		return nil, err
	}
	if err = appendIdentityMutation(ctx, tx, workspaceID, actorID, "identity.domain_token_regenerated", service.EventWorkspaceDomainRegenerated, "domain", domainID, service.DomainEventData{"status": item.Status}); err != nil {
		return nil, err
	}
	return item, enterpriseIdentityError(tx.Commit())
}

func (r *enterpriseIdentityRepository) MarkDomainChecked(ctx context.Context, workspaceID, actorID int64, domainID int64, expectedHash []byte, errorCode string, verified bool) (*service.EnterpriseDomain, error) {
	if len(expectedHash) != 32 {
		return nil, service.ErrEnterpriseIdentityInvalid
	}
	tx, _, err := r.beginScoped(ctx, workspaceID, actorID, "identity.manage")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	status := "failed"
	if verified {
		status = "verified"
	}
	var lastError any = errorCode
	if errorCode == "" {
		lastError = nil
	}
	// A transient resolver failure never loses a verified claim. Comparing the
	// original generation prevents a delayed lookup from verifying a new token.
	item, err := scanEnterpriseDomain(tx.QueryRowContext(ctx, `UPDATE workspace_domains SET status=CASE WHEN $4='DNS_LOOKUP_FAILED' THEN status ELSE $3 END,verified_at=CASE WHEN $4='DNS_LOOKUP_FAILED' THEN verified_at WHEN $3='verified' THEN COALESCE(verified_at,now()) ELSE NULL END,last_checked_at=now(),last_error_code=$4,updated_at=now() WHERE workspace_id=$1 AND id=$2 AND verification_token_hash=$5 AND status<>'revoked' RETURNING `+enterpriseDomainColumns, workspaceID, domainID, status, lastError, expectedHash))
	if err != nil {
		return nil, err
	}
	eventType := ""
	if verified && errorCode == "" {
		eventType = service.EventWorkspaceDomainVerified
	}
	if err = appendIdentityMutation(ctx, tx, workspaceID, actorID, "identity.domain_checked", eventType, "domain", domainID, service.DomainEventData{"status": item.Status, "reason_code": errorCode}); err != nil {
		return nil, err
	}
	return item, enterpriseIdentityError(tx.Commit())
}

func (r *enterpriseIdentityRepository) RevokeDomain(ctx context.Context, workspaceID, actorID, domainID int64) (*service.EnterpriseDomain, error) {
	tx, _, err := r.beginScoped(ctx, workspaceID, actorID, "identity.manage")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	item, err := scanEnterpriseDomain(tx.QueryRowContext(ctx, `UPDATE workspace_domains SET status='revoked',verified_at=NULL,updated_at=now() WHERE workspace_id=$1 AND id=$2 RETURNING `+enterpriseDomainColumns, workspaceID, domainID))
	if err != nil {
		return nil, err
	}
	if err = appendIdentityMutation(ctx, tx, workspaceID, actorID, "identity.domain_revoked", service.EventWorkspaceDomainRevoked, "domain", domainID, service.DomainEventData{"status": item.Status}); err != nil {
		return nil, err
	}
	return item, enterpriseIdentityError(tx.Commit())
}

const enterpriseProviderColumns = `id,workspace_id,type,provider_key,name,status,is_default,COALESCE(issuer_url,''),COALESCE(client_id,''),(NULLIF(encrypted_client_secret,'') IS NOT NULL),COALESCE(scopes,'{}'::text[]),authorization_endpoint,token_endpoint,jwks_uri,userinfo_endpoint,COALESCE(discovery_enabled,false),claim_mapping,jit_config,created_by_user_id,created_at,updated_at,disabled_at,revision,COALESCE(token_auth_method,''),last_validated_at,last_validation_code,saml_config,COALESCE(saml_public_id,'')`

func scanEnterpriseProvider(scanner interface{ Scan(...any) error }, secret ...*sql.NullString) (*service.EnterpriseIdentityProvider, string, error) {
	p := &service.EnterpriseIdentityProvider{}
	var scopes pq.StringArray
	var authEndpoint, tokenEndpoint, jwksURI, userinfoEndpoint sql.NullString
	var lastValidationCode sql.NullString
	var claimJSON, jitJSON, samlJSON []byte
	var hasSecret bool
	args := []any{&p.ID, &p.WorkspaceID, &p.Type, &p.ProviderKey, &p.Name, &p.Status, &p.IsDefault, &p.IssuerURL, &p.ClientID, &hasSecret, &scopes, &authEndpoint, &tokenEndpoint, &jwksURI, &userinfoEndpoint, &p.DiscoveryEnabled, &claimJSON, &jitJSON, &p.CreatedByUserID, &p.CreatedAt, &p.UpdatedAt, &p.DisabledAt, &p.Revision, &p.TokenAuthMethod, &p.LastValidatedAt, &lastValidationCode, &samlJSON, &p.PublicID}
	for _, s := range secret {
		args = append(args, s)
	}
	err := scanner.Scan(args...)
	if err != nil {
		return nil, "", enterpriseIdentityError(err)
	}
	if len(samlJSON) > 0 {
		if err := json.Unmarshal(samlJSON, &p.SAML); err != nil {
			return nil, "", err
		}
		p.SAML.MetadataXML = ""
	}
	p.HasClientSecret = hasSecret
	if lastValidationCode.Valid {
		p.LastValidationCode = lastValidationCode.String
	}
	p.Scopes = append([]string(nil), scopes...)
	if authEndpoint.Valid {
		p.AuthorizationEndpoint = authEndpoint.String
	}
	if tokenEndpoint.Valid {
		p.TokenEndpoint = tokenEndpoint.String
	}
	if jwksURI.Valid {
		p.JWKSURI = jwksURI.String
	}
	if userinfoEndpoint.Valid {
		p.UserinfoEndpoint = userinfoEndpoint.String
	}
	if len(claimJSON) > 0 {
		if err := json.Unmarshal(claimJSON, &p.ClaimMapping); err != nil {
			return nil, "", err
		}
	}
	if len(jitJSON) > 0 {
		if err := json.Unmarshal(jitJSON, &p.JITConfig); err != nil {
			return nil, "", err
		}
	}
	return p, "", nil
}

func providerJSON(input service.EnterpriseIdentityProviderInput) ([]byte, []byte, error) {
	claimJSON, err := json.Marshal(input.ClaimMapping)
	if err != nil {
		return nil, nil, err
	}
	jitJSON, err := json.Marshal(input.JITConfig)
	if err != nil {
		return nil, nil, err
	}
	return claimJSON, jitJSON, nil
}

func (r *enterpriseIdentityRepository) ListProviders(ctx context.Context, workspaceID, actorID int64, p pagination.PaginationParams) ([]service.EnterpriseIdentityProvider, int64, error) {
	tx, _, err := r.beginScoped(ctx, workspaceID, actorID, "identity.read")
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	var total int64
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM workspace_identity_providers WHERE workspace_id=$1`, workspaceID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+enterpriseProviderColumns+` FROM workspace_identity_providers WHERE workspace_id=$1 ORDER BY id DESC LIMIT $2 OFFSET $3`, workspaceID, p.Limit(), p.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.EnterpriseIdentityProvider, 0)
	for rows.Next() {
		item, _, scanErr := scanEnterpriseProvider(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, *item)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	_ = rows.Close()
	return items, total, enterpriseIdentityError(tx.Commit())
}

func (r *enterpriseIdentityRepository) CreateProvider(ctx context.Context, workspaceID, actorID int64, input service.EnterpriseIdentityProviderInput, ciphertext string, scopes []string) (*service.EnterpriseIdentityProvider, error) {
	protocol := enterpriseProtocol(input.Type)
	if protocol != "oidc" && protocol != "saml" {
		return nil, service.ErrEnterpriseIdentityInvalid
	}
	method := input.TokenAuthMethod
	if method == "" {
		method = "client_secret_basic"
	}
	var samlConfig []byte
	var publicID string
	var err error
	if protocol == "saml" {
		if !validSAMLRepositoryInput(input) || ciphertext == "" {
			return nil, service.ErrEnterpriseIdentityInvalid
		}
		samlConfig, err = sanitizedSAMLConfig(input.SAML)
		if err != nil {
			return nil, err
		}
		publicID, _, err = service.NewSecureToken(32)
		if err != nil {
			return nil, err
		}
	} else if input.SAML != nil || !validEnterpriseClientAuthentication(method, ciphertext != "") {
		return nil, service.ErrEnterpriseIdentityInvalid
	}
	claimJSON, jitJSON, err := providerJSON(input)
	if err != nil {
		return nil, err
	}
	tx, _, err := r.beginScoped(ctx, workspaceID, actorID, "identity.manage")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if input.IsDefault {
		if _, err = tx.ExecContext(ctx, `UPDATE workspace_identity_providers SET is_default=false,revision=revision+1,last_validated_at=NULL,last_validation_code=NULL,updated_at=now() WHERE workspace_id=$1 AND is_default`, workspaceID); err != nil {
			return nil, enterpriseIdentityError(err)
		}
	}
	discovery := input.DiscoveryEnabled == nil || *input.DiscoveryEnabled
	var item *service.EnterpriseIdentityProvider
	if protocol == "saml" {
		item, _, err = scanEnterpriseProvider(tx.QueryRowContext(ctx, `INSERT INTO workspace_identity_providers(workspace_id,type,provider_key,name,issuer_url,client_id,encrypted_client_secret,token_auth_method,scopes,discovery_enabled,is_default,claim_mapping,jit_config,created_by_user_id,saml_config,encrypted_saml_sp_keys,saml_public_id) VALUES($1,'saml',$2,$3,NULL,NULL,NULL,NULL,NULL,NULL,$4,$5,$6,$7,$8,$9,$10) RETURNING `+enterpriseProviderColumns, workspaceID, input.ProviderKey, input.Name, input.IsDefault, claimJSON, jitJSON, actorID, samlConfig, ciphertext, publicID))
	} else {
		item, _, err = scanEnterpriseProvider(tx.QueryRowContext(ctx, `INSERT INTO workspace_identity_providers(workspace_id,provider_key,name,issuer_url,client_id,encrypted_client_secret,scopes,is_default,discovery_enabled,claim_mapping,jit_config,created_by_user_id,token_auth_method,authorization_endpoint,token_endpoint,jwks_uri,userinfo_endpoint) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9,$10,$11,$12,$13,NULLIF($14,''),NULLIF($15,''),NULLIF($16,''),NULLIF($17,'')) RETURNING `+enterpriseProviderColumns, workspaceID, input.ProviderKey, input.Name, input.IssuerURL, input.ClientID, ciphertext, pq.Array(scopes), input.IsDefault, discovery, claimJSON, jitJSON, actorID, method, input.AuthorizationEndpoint, input.TokenEndpoint, input.JWKSURI, input.UserinfoEndpoint))
	}
	if err != nil {
		return nil, err
	}
	if err = appendIdentityMutation(ctx, tx, workspaceID, actorID, "identity.provider_created", service.EventIdentityProviderCreated, "identity_provider", item.ID, service.DomainEventData{"status": item.Status, "provider_id": item.ID, "provider_revision": item.Revision, "category": protocol}); err != nil {
		return nil, err
	}
	return item, enterpriseIdentityError(tx.Commit())
}

func getEnterpriseProvider(ctx context.Context, q workspaceSQL, workspaceID, providerID int64, lock bool) (*service.EnterpriseIdentityProvider, string, error) {
	var secret sql.NullString
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	p, _, err := scanEnterpriseProvider(q.QueryRowContext(ctx, `SELECT `+enterpriseProviderColumns+`,CASE WHEN type='saml' THEN encrypted_saml_sp_keys ELSE encrypted_client_secret END FROM workspace_identity_providers WHERE workspace_id=$1 AND id=$2`+suffix, workspaceID, providerID), &secret)
	return p, secret.String, err
}

func (r *enterpriseIdentityRepository) GetProvider(ctx context.Context, workspaceID, actorID, providerID int64) (*service.EnterpriseIdentityProvider, string, error) {
	if actorID == 0 {
		return getEnterpriseProvider(ctx, r.db, workspaceID, providerID, false)
	}
	tx, _, err := r.beginScoped(ctx, workspaceID, actorID, "identity.read")
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = tx.Rollback() }()
	p, ciphertext, err := getEnterpriseProvider(ctx, tx, workspaceID, providerID, false)
	if err != nil {
		return nil, "", err
	}
	return p, ciphertext, enterpriseIdentityError(tx.Commit())
}

func (r *enterpriseIdentityRepository) UpdateProvider(ctx context.Context, workspaceID, actorID, providerID int64, input service.EnterpriseIdentityProviderInput, ciphertext *string, scopes []string) (*service.EnterpriseIdentityProvider, error) {
	claimJSON, jitJSON, err := providerJSON(input)
	if err != nil {
		return nil, err
	}
	tx, _, err := r.beginScoped(ctx, workspaceID, actorID, "identity.manage")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	current, currentCiphertext, err := getEnterpriseProvider(ctx, tx, workspaceID, providerID, true)
	if err != nil {
		return nil, err
	}
	if enterpriseProtocol(input.Type) != current.Type {
		return nil, service.ErrWorkspaceConflict
	}
	if input.Revision != current.Revision {
		return nil, service.ErrWorkspaceConflict
	}
	// Subjects are scoped to the issuer/client namespace. Reusing a provider
	// ID with another namespace must never adopt its existing global users.
	namespaceChanged := input.IssuerURL != current.IssuerURL || input.ClientID != current.ClientID
	if current.Type == "saml" {
		if !validSAMLRepositoryInput(input) || current.SAML == nil || ciphertext != nil {
			return nil, service.ErrEnterpriseIdentityInvalid
		}
		namespaceChanged = input.SAML.IDPEntityID != current.SAML.IDPEntityID || input.SAML.SubjectAttribute != current.SAML.SubjectAttribute || input.SAML.AllowUnspecifiedNameID != current.SAML.AllowUnspecifiedNameID
	}
	if namespaceChanged {
		var bound bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_user_identities WHERE workspace_id=$1 AND provider_id=$2)`, workspaceID, providerID).Scan(&bound); err != nil {
			return nil, err
		}
		if bound {
			return nil, service.ErrWorkspaceConflict
		}
	}
	if input.IsDefault && current.Status != "active" {
		return nil, service.ErrOIDCProviderDisabled
	}
	if current.Type == "saml" {
		config := *input.SAML
		config.SPCertificate, config.NextSPCertificate = current.SAML.SPCertificate, current.SAML.NextSPCertificate
		configJSON, err := sanitizedSAMLConfig(&config)
		if err != nil {
			return nil, err
		}
		if input.IsDefault {
			if _, err = tx.ExecContext(ctx, `UPDATE workspace_identity_providers SET is_default=false,revision=revision+1,last_validated_at=NULL,last_validation_code=NULL,updated_at=now() WHERE workspace_id=$1 AND id<>$2 AND is_default`, workspaceID, providerID); err != nil {
				return nil, enterpriseIdentityError(err)
			}
		}
		item, _, err := scanEnterpriseProvider(tx.QueryRowContext(ctx, `UPDATE workspace_identity_providers SET provider_key=$3,name=$4,is_default=$5,claim_mapping=$6,jit_config=$7,saml_config=$8,revision=revision+1,last_validated_at=NULL,last_validation_code=NULL,updated_at=now() WHERE workspace_id=$1 AND id=$2 RETURNING `+enterpriseProviderColumns, workspaceID, providerID, input.ProviderKey, input.Name, input.IsDefault, claimJSON, jitJSON, configJSON))
		if err != nil {
			return nil, err
		}
		if err = appendIdentityMutation(ctx, tx, workspaceID, actorID, "identity.provider_updated", service.EventIdentityProviderUpdated, "identity_provider", providerID, service.DomainEventData{"status": item.Status, "provider_id": item.ID, "provider_revision": item.Revision, "category": "saml"}); err != nil {
			return nil, err
		}
		if input.SAML.IDPEntityID != current.SAML.IDPEntityID || input.SAML.SSOURL != current.SAML.SSOURL || input.SAML.MetadataURL != current.SAML.MetadataURL || !slices.Equal(input.SAML.SigningCertificates, current.SAML.SigningCertificates) {
			if err = appendIdentityMutation(ctx, tx, workspaceID, actorID, "identity.saml_metadata_updated", service.EventSAMLMetadataUpdated, "identity_provider", providerID, service.DomainEventData{"provider_id": item.ID, "provider_revision": item.Revision, "category": "saml"}); err != nil {
				return nil, err
			}
		}
		return item, enterpriseIdentityError(tx.Commit())
	}
	var secret any
	replaceSecret := ciphertext != nil
	if ciphertext != nil {
		secret = *ciphertext
	}
	switch input.SecretAction {
	case "", "preserve":
		if input.SecretAction == "preserve" && ciphertext != nil {
			return nil, service.ErrEnterpriseIdentityInvalid
		}
	case "remove":
		secret, replaceSecret = "", true
	case "replace":
		if ciphertext == nil || *ciphertext == "" {
			return nil, service.ErrEnterpriseIdentityInvalid
		}
	default:
		return nil, service.ErrEnterpriseIdentityInvalid
	}
	method := input.TokenAuthMethod
	if method == "" {
		method = current.TokenAuthMethod
	}
	hasSecret := currentCiphertext != ""
	if replaceSecret {
		hasSecret = secret != nil && secret != ""
	}
	if !validEnterpriseClientAuthentication(method, hasSecret) {
		return nil, service.ErrEnterpriseIdentityInvalid
	}
	if input.IsDefault {
		if _, err = tx.ExecContext(ctx, `UPDATE workspace_identity_providers SET is_default=false,revision=revision+1,last_validated_at=NULL,last_validation_code=NULL,updated_at=now() WHERE workspace_id=$1 AND id<>$2 AND is_default`, workspaceID, providerID); err != nil {
			return nil, enterpriseIdentityError(err)
		}
	}
	discovery := input.DiscoveryEnabled == nil || *input.DiscoveryEnabled
	item, _, err := scanEnterpriseProvider(tx.QueryRowContext(ctx, `UPDATE workspace_identity_providers SET provider_key=$3,name=$4,issuer_url=$5,client_id=$6,encrypted_client_secret=CASE WHEN $8 THEN NULLIF($7,'') ELSE encrypted_client_secret END,scopes=$9,is_default=$10,discovery_enabled=$11,claim_mapping=$12,jit_config=$13,token_auth_method=$14,authorization_endpoint=NULLIF($15,''),token_endpoint=NULLIF($16,''),jwks_uri=NULLIF($17,''),userinfo_endpoint=NULLIF($18,''),revision=revision+1,last_validated_at=NULL,last_validation_code=NULL,updated_at=now() WHERE workspace_id=$1 AND id=$2 RETURNING `+enterpriseProviderColumns, workspaceID, providerID, input.ProviderKey, input.Name, input.IssuerURL, input.ClientID, secret, replaceSecret, pq.Array(scopes), input.IsDefault, discovery, claimJSON, jitJSON, method, input.AuthorizationEndpoint, input.TokenEndpoint, input.JWKSURI, input.UserinfoEndpoint))
	if err != nil {
		return nil, err
	}
	if err = appendIdentityMutation(ctx, tx, workspaceID, actorID, "identity.provider_updated", service.EventIdentityProviderUpdated, "identity_provider", providerID, service.DomainEventData{"status": item.Status, "provider_id": item.ID, "provider_revision": item.Revision}); err != nil {
		return nil, err
	}
	return item, enterpriseIdentityError(tx.Commit())
}

func validEnterpriseClientAuthentication(method string, hasSecret bool) bool {
	switch method {
	case "none":
		return !hasSecret
	case "client_secret_basic", "client_secret_post":
		return hasSecret
	default:
		return false
	}
}

func (r *enterpriseIdentityRepository) DisableProvider(ctx context.Context, workspaceID, actorID int64, providerID int64) error {
	tx, _, err := r.beginScoped(ctx, workspaceID, actorID, "identity.manage")
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, _, err = getEnterpriseProvider(ctx, tx, workspaceID, providerID, true); err != nil {
		return err
	}
	var lockout bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_security_policies WHERE workspace_id=$1 AND require_sso) AND NOT EXISTS(SELECT 1 FROM workspace_identity_providers WHERE workspace_id=$1 AND id<>$2 AND status='active')`, workspaceID, providerID).Scan(&lockout); err != nil {
		return err
	}
	if lockout {
		return service.ErrWorkspaceConflict
	}
	if _, err = tx.ExecContext(ctx, `UPDATE workspace_identity_providers SET status='disabled',revision=revision+1,disabled_at=now(),is_default=false,last_validated_at=NULL,last_validation_code=NULL,updated_at=now() WHERE workspace_id=$1 AND id=$2`, workspaceID, providerID); err != nil {
		return enterpriseIdentityError(err)
	}
	if err = appendIdentityMutation(ctx, tx, workspaceID, actorID, "identity.provider_disabled", service.EventIdentityProviderDisabled, "identity_provider", providerID, service.DomainEventData{"status": "disabled", "provider_id": providerID}); err != nil {
		return err
	}
	return enterpriseIdentityError(tx.Commit())
}

func (r *enterpriseIdentityRepository) RecordProviderValidation(ctx context.Context, workspaceID, actorID, providerID, revision int64, code string) error {
	switch code {
	case "SUCCESS", "DISCOVERY_FAILED", "ISSUER_MISMATCH", "ENDPOINT_INVALID", "CONFIGURATION_INVALID", "VALIDATION_FAILED", "PROVIDER_DISABLED":
	default:
		return service.ErrEnterpriseIdentityInvalid
	}
	if revision <= 0 {
		return service.ErrEnterpriseIdentityInvalid
	}
	tx, _, err := r.beginScoped(ctx, workspaceID, actorID, "identity.manage")
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	provider, _, err := getEnterpriseProvider(ctx, tx, workspaceID, providerID, true)
	if err != nil {
		return err
	}
	if provider.Revision != revision {
		return service.ErrWorkspaceConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE workspace_identity_providers SET last_validated_at=now(),last_validation_code=$4 WHERE workspace_id=$1 AND id=$2 AND revision=$3`, workspaceID, providerID, revision, code)
	if err != nil {
		return enterpriseIdentityError(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return service.ErrWorkspaceConflict
	}
	if err = appendIdentityMutation(ctx, tx, workspaceID, actorID, "identity.provider_validation", service.EventIdentityProviderUpdated, "identity_provider", providerID, service.DomainEventData{"provider_id": providerID, "provider_revision": revision, "reason_code": code}); err != nil {
		return err
	}
	return enterpriseIdentityError(tx.Commit())
}

func (r *enterpriseIdentityRepository) ActiveProviderCount(ctx context.Context, workspaceID, _ int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM workspace_identity_providers WHERE workspace_id=$1 AND status='active'`, workspaceID).Scan(&count)
	return count, err
}

func (r *enterpriseIdentityRepository) HasVerifiedDomain(ctx context.Context, workspaceID int64, normalizedDomain string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_domains WHERE workspace_id=$1 AND normalized_domain=$2 AND status='verified')`, workspaceID, normalizedDomain).Scan(&exists)
	return exists, err
}

func (r *enterpriseIdentityRepository) GetPolicy(ctx context.Context, workspaceID, actorID int64) (*service.WorkspaceIdentityPolicy, error) {
	const query = `SELECT w.id,COALESCE(p.require_sso,false),p.sso_grace_until,COALESCE(p.revision,1),p.updated_by_user_id,COALESCE(p.updated_at,w.updated_at) FROM workspaces w LEFT JOIN workspace_security_policies p ON p.workspace_id=w.id WHERE w.id=$1`
	if actorID == 0 {
		return scanIdentityPolicy(r.db.QueryRowContext(ctx, query, workspaceID))
	}
	tx, _, err := r.beginScoped(ctx, workspaceID, actorID, "identity.read")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	item, err := scanIdentityPolicy(tx.QueryRowContext(ctx, query, workspaceID))
	if err != nil {
		return nil, err
	}
	return item, enterpriseIdentityError(tx.Commit())
}

func scanIdentityPolicy(scanner interface{ Scan(...any) error }) (*service.WorkspaceIdentityPolicy, error) {
	p := &service.WorkspaceIdentityPolicy{}
	err := scanner.Scan(&p.WorkspaceID, &p.RequireSSO, &p.SSOGraceUntil, &p.Revision, &p.UpdatedBy, &p.UpdatedAt)
	return p, enterpriseIdentityError(err)
}

func (r *enterpriseIdentityRepository) UpdatePolicy(ctx context.Context, workspaceID, actorID int64, requireSSO bool, graceUntil *time.Time) (*service.WorkspaceIdentityPolicy, error) {
	tx, _, err := r.beginScoped(ctx, workspaceID, actorID, "workspace_sso.update")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if requireSSO {
		// Configuration validation runs outside the transaction. Recheck the
		// requesting Owner's exact assurance after acquiring the same workspace
		// lock used by provider edits, so a revision change cannot race enabling.
		assurance, ok := service.AuthenticationAssuranceFromContext(ctx)
		if !ok || assurance.WorkspaceID != workspaceID || (assurance.AuthMethod != "oidc" && assurance.AuthMethod != "saml") || !assurance.Valid(time.Now()) {
			return nil, service.ErrSSORequired
		}
		var ready bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_domains WHERE workspace_id=$1 AND status='verified') AND EXISTS(SELECT 1 FROM workspace_user_identities i JOIN workspace_identity_providers p ON p.workspace_id=i.workspace_id AND p.id=i.provider_id AND p.status='active' JOIN workspace_members m ON m.workspace_id=i.workspace_id AND m.user_id=i.user_id AND m.role='owner' AND m.status='active' JOIN users u ON u.id=m.user_id AND u.status='active' AND u.deleted_at IS NULL WHERE i.workspace_id=$1 AND i.provider_id=$2 AND p.revision=$3 AND i.user_id=$4 AND p.type=$5)`, workspaceID, assurance.ProviderID, assurance.ProviderRevision, actorID, assurance.AuthMethod).Scan(&ready); err != nil {
			return nil, err
		}
		if !ready {
			return nil, service.ErrWorkspaceConflict
		}
	}
	item, err := writeIdentityPolicy(ctx, tx, workspaceID, actorID, requireSSO, graceUntil)
	if err != nil {
		return nil, err
	}
	eventType := service.EventSSOEnforcementDisabled
	if requireSSO {
		eventType = service.EventSSOEnforcementEnabled
	}
	if err = appendIdentityMutation(ctx, tx, workspaceID, actorID, "identity.policy_updated", eventType, "workspace", workspaceID, service.DomainEventData{"policy_revision": item.Revision, "require_sso": requireSSO}); err != nil {
		return nil, err
	}
	return item, enterpriseIdentityError(tx.Commit())
}

func writeIdentityPolicy(ctx context.Context, tx *sql.Tx, workspaceID, actorID int64, requireSSO bool, graceUntil *time.Time) (*service.WorkspaceIdentityPolicy, error) {
	return scanIdentityPolicy(tx.QueryRowContext(ctx, `INSERT INTO workspace_security_policies(workspace_id,require_sso,sso_grace_until,revision,updated_by_user_id) VALUES($1,$2,$3,2,$4) ON CONFLICT (workspace_id) DO UPDATE SET require_sso=EXCLUDED.require_sso,sso_grace_until=EXCLUDED.sso_grace_until,revision=workspace_security_policies.revision+1,updated_by_user_id=EXCLUDED.updated_by_user_id,updated_at=now() RETURNING workspace_id,require_sso,sso_grace_until,revision,updated_by_user_id,updated_at`, workspaceID, requireSSO, graceUntil, actorID))
}

func (r *enterpriseIdentityRepository) CreateOIDCState(ctx context.Context, state *service.OIDCState) error {
	if state == nil || state.ProviderRevision <= 0 || len(state.BrowserSessionHash) != 32 || service.ValidateEnterpriseReturnTo(state.ReturnTo) != nil || (state.Intent != "login" && state.Intent != "link") || (state.Intent == "link" && (state.LinkUserID == nil || *state.LinkUserID <= 0)) || (state.Intent == "login" && state.LinkUserID != nil) {
		return service.ErrEnterpriseIdentityInvalid
	}
	protocol := enterpriseProtocol(state.Protocol)
	if protocol != "oidc" && protocol != "saml" {
		return service.ErrEnterpriseIdentityInvalid
	}
	if protocol == "saml" && (strings.TrimSpace(state.RequestID) == "" || len(state.RequestID) > 256) {
		return service.ErrEnterpriseIdentityInvalid
	}
	if protocol == "oidc" && (r.encryptor == nil || len(state.NonceHash) != 32 || state.PKCEVerifier == "" || state.Nonce == "" || state.RequestID != "") {
		return service.ErrEnterpriseIdentityInvalid
	}
	hash, err := hex.DecodeString(state.Hash)
	if err != nil || len(hash) != 32 {
		return service.ErrEnterpriseIdentityInvalid
	}
	var nonceHash, ciphertext, nonceCiphertext, requestID any
	if protocol == "oidc" {
		nonceHash = state.NonceHash
		ciphertext, err = r.encryptor.Encrypt(state.PKCEVerifier)
		if err != nil {
			return err
		}
		nonceCiphertext, err = r.encryptor.Encrypt(state.Nonce)
		if err != nil {
			return err
		}
	} else {
		requestID = state.RequestID
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockWorkspace(ctx, tx, state.WorkspaceID, false); err != nil {
		return err
	}
	if err = cleanupExpiredIdentityAuthentication(ctx, tx); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO workspace_identity_auth_states(state_hash,workspace_id,provider_id,provider_revision,browser_session_hash,nonce_hash,pkce_verifier_ciphertext,nonce_ciphertext,return_to,intent,link_user_id,expires_at,protocol,request_id) SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10::varchar,$11,$12,$13::varchar,$14 FROM workspace_identity_providers p JOIN workspaces w ON w.id=p.workspace_id WHERE p.workspace_id=$2 AND p.id=$3 AND p.revision=$4 AND p.status='active' AND p.type=$13::varchar AND w.status='active' AND w.type='organization' AND ($10::varchar='login' OR EXISTS(SELECT 1 FROM workspace_members m JOIN users u ON u.id=m.user_id WHERE m.workspace_id=$2 AND m.user_id=$11 AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL))`, hash, state.WorkspaceID, state.ProviderID, state.ProviderRevision, state.BrowserSessionHash, nonceHash, ciphertext, nonceCiphertext, state.ReturnTo, state.Intent, state.LinkUserID, state.ExpiresAt, protocol, requestID)
	if err == nil {
		var count int64
		count, err = result.RowsAffected()
		if err == nil && count != 1 {
			return service.ErrOIDCStateSessionMismatch
		}
	}
	if err != nil {
		return enterpriseIdentityError(err)
	}
	return enterpriseIdentityError(tx.Commit())
}

// Expired credentials retain a one-day troubleshooting window. Each creation
// removes at most 100 expired rows per table without waiting for other workers.
func cleanupExpiredIdentityAuthentication(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM workspace_identity_auth_states WHERE id IN(SELECT id FROM workspace_identity_auth_states WHERE expires_at<now()-interval '1 day' ORDER BY expires_at,id LIMIT 100 FOR UPDATE SKIP LOCKED)`); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM workspace_identity_login_completions WHERE token_hash IN(SELECT token_hash FROM workspace_identity_login_completions WHERE expires_at<now()-interval '1 day' ORDER BY expires_at,token_hash LIMIT 100 FOR UPDATE SKIP LOCKED)`)
	return err
}

func (r *enterpriseIdentityRepository) ConsumeOIDCState(ctx context.Context, stateHash string, browserHash []byte, now time.Time) (*service.OIDCState, error) {
	hash, err := hex.DecodeString(stateHash)
	if err != nil || len(hash) != 32 || len(browserHash) != 32 {
		return nil, service.ErrOIDCStateNotFound
	}
	row := r.db.QueryRowContext(ctx, `UPDATE workspace_identity_auth_states s SET consumed_at=$2 WHERE state_hash=$1 AND browser_session_hash=$3 AND consumed_at IS NULL AND expires_at>$2 AND EXISTS(SELECT 1 FROM workspace_identity_providers p JOIN workspaces w ON w.id=p.workspace_id WHERE p.workspace_id=s.workspace_id AND p.id=s.provider_id AND p.revision=s.provider_revision AND p.status='active' AND p.type=s.protocol AND w.status='active' AND w.type='organization') RETURNING workspace_id,provider_id,provider_revision,browser_session_hash,nonce_hash,pkce_verifier_ciphertext,nonce_ciphertext,return_to,intent,link_user_id,expires_at,created_at,protocol,request_id`, hash, now, browserHash)
	var state service.OIDCState
	var ciphertext, nonceCiphertext, requestID sql.NullString
	if err := row.Scan(&state.WorkspaceID, &state.ProviderID, &state.ProviderRevision, &state.BrowserSessionHash, &state.NonceHash, &ciphertext, &nonceCiphertext, &state.ReturnTo, &state.Intent, &state.LinkUserID, &state.ExpiresAt, &state.CreatedAt, &state.Protocol, &requestID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrOIDCStateNotFound
		}
		return nil, enterpriseIdentityError(err)
	}
	state.Hash = stateHash
	state.RequestID = requestID.String
	if state.Protocol == "saml" {
		return &state, nil
	}
	if state.Protocol != "oidc" {
		return nil, service.ErrEnterpriseIdentityInvalid
	}
	if r.encryptor == nil {
		return nil, service.ErrEnterpriseIdentityInvalid
	}
	state.PKCEVerifier, err = r.encryptor.Decrypt(ciphertext.String)
	if err != nil {
		return nil, service.ErrEnterpriseIdentityInvalid
	}
	state.Nonce, err = r.encryptor.Decrypt(nonceCiphertext.String)
	if err != nil {
		return nil, service.ErrEnterpriseIdentityInvalid
	}
	return &state, nil
}

func (r *enterpriseIdentityRepository) FindIdentity(ctx context.Context, workspaceID, providerID int64, subject string) (*service.OIDCIdentity, error) {
	i := &service.OIDCIdentity{}
	var email sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT id,workspace_id,provider_id,user_id,subject,email_at_link,email_verified,COALESCE(last_seen_at,created_at) FROM workspace_user_identities WHERE workspace_id=$1 AND provider_id=$2 AND subject=$3`, workspaceID, providerID, subject).Scan(&i.ID, &i.WorkspaceID, &i.ProviderID, &i.UserID, &i.Subject, &email, &i.EmailVerified, &i.LastSeenAt)
	if email.Valid {
		i.EmailAtLink = email.String
	}
	return i, enterpriseIdentityError(err)
}

func (r *enterpriseIdentityRepository) BindIdentity(ctx context.Context, binding service.OIDCIdentityBinding) error {
	// Legacy compatibility only refreshes an existing, exact stable binding.
	// New links must use CompleteIdentityLogin's authenticated server-bound
	// intent so this helper cannot bypass linking or atomic provisioning.
	result, err := r.db.ExecContext(ctx, `UPDATE workspace_user_identities i SET email_verified=$5,last_seen_at=now(),updated_at=now() WHERE i.workspace_id=$1 AND i.provider_id=$2 AND i.user_id=$3 AND i.subject=$4 AND EXISTS(SELECT 1 FROM workspace_members m JOIN users u ON u.id=m.user_id JOIN workspaces w ON w.id=m.workspace_id JOIN workspace_identity_providers p ON p.workspace_id=w.id AND p.id=$2 WHERE m.workspace_id=$1 AND m.user_id=$3 AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL AND w.status='active' AND p.status='active')`, binding.WorkspaceID, binding.ProviderID, binding.UserID, binding.Subject, binding.EmailVerified)
	if err != nil {
		return enterpriseIdentityError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrOIDCAccountLinkRequired
	}
	return nil
}

func (r *enterpriseIdentityRepository) EnsureOIDCMembership(ctx context.Context, workspaceID, userID, providerID int64, role string) error {
	if !validOIDCRoleForRepository(role) {
		role = service.WorkspaceRoleViewer
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockWorkspace(ctx, tx, workspaceID, true); err != nil {
		return err
	}
	access, err := workspaceAccess(ctx, tx, userID, workspaceID, 0)
	if err != nil {
		return err
	}
	if access.Workspace.Status != "active" || access.Member.Status != "active" {
		return service.ErrWorkspaceForbidden
	}
	provider, _, err := getEnterpriseProvider(ctx, tx, workspaceID, providerID, true)
	if err != nil {
		return err
	}
	if provider.Status != "active" {
		return service.ErrOIDCProviderDisabled
	}
	// Existing manual/SCIM/other-provider assignments and Owner survive. This
	// compatibility helper never recreates a removed or suspended membership.
	if err = upsertProviderMemberSource(ctx, tx, workspaceID, access.Member.ID, providerID, provider.Type, role); err != nil {
		return enterpriseIdentityError(err)
	}
	if err = reconcileWorkspaceMemberSources(ctx, tx, workspaceID, access.Member.ID); err != nil {
		return err
	}
	return enterpriseIdentityError(tx.Commit())
}

func validOIDCRoleForRepository(role string) bool {
	return role == service.WorkspaceRoleViewer || role == service.WorkspaceRoleDeveloper || role == service.WorkspaceRoleAdmin || role == service.WorkspaceRoleBilling
}

func getOIDCMappings(ctx context.Context, q workspaceSQL, workspaceID, providerID int64) (*service.OIDCMappings, error) {
	items := &service.OIDCMappings{Roles: []service.OIDCRoleMapping{}, Teams: []service.OIDCTeamMapping{}}
	rows, err := q.QueryContext(ctx, `SELECT claim_value,role,priority FROM workspace_identity_role_mappings WHERE workspace_id=$1 AND provider_id=$2 AND enabled ORDER BY priority DESC,role,claim_value LIMIT 201`, workspaceID, providerID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item service.OIDCRoleMapping
		if err = rows.Scan(&item.ClaimValue, &item.Role, &item.Priority); err != nil {
			_ = rows.Close()
			return nil, err
		}
		items.Roles = append(items.Roles, item)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	if len(items.Roles) > 200 {
		return nil, service.ErrEnterpriseIdentityInvalid
	}
	rows, err = q.QueryContext(ctx, `SELECT claim_value,team_id FROM workspace_identity_team_mappings WHERE workspace_id=$1 AND provider_id=$2 AND enabled ORDER BY team_id,claim_value LIMIT 201`, workspaceID, providerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var item service.OIDCTeamMapping
		if err = rows.Scan(&item.ClaimValue, &item.TeamID); err != nil {
			return nil, err
		}
		items.Teams = append(items.Teams, item)
	}
	if len(items.Teams) > 200 {
		return nil, service.ErrEnterpriseIdentityInvalid
	}
	return items, rows.Err()
}

func (r *enterpriseIdentityRepository) GetMappings(ctx context.Context, workspaceID, providerID int64) (*service.OIDCMappings, error) {
	if _, _, err := getEnterpriseProvider(ctx, r.db, workspaceID, providerID, false); err != nil {
		return nil, err
	}
	return getOIDCMappings(ctx, r.db, workspaceID, providerID)
}

func (r *enterpriseIdentityRepository) ReplaceMappings(ctx context.Context, workspaceID, actorID, providerID int64, input service.OIDCMappings) error {
	if len(input.Roles) > 200 || len(input.Teams) > 200 {
		return service.ErrEnterpriseIdentityInvalid
	}
	for i := range input.Roles {
		input.Roles[i].ClaimValue = strings.TrimSpace(input.Roles[i].ClaimValue)
	}
	for i := range input.Teams {
		input.Teams[i].ClaimValue = strings.TrimSpace(input.Teams[i].ClaimValue)
	}
	tx, _, err := r.beginScoped(ctx, workspaceID, actorID, "identity.manage")
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	provider, _, err := getEnterpriseProvider(ctx, tx, workspaceID, providerID, true)
	if err != nil {
		return err
	}
	seenRoles, seenTeams := map[string]bool{}, map[string]bool{}
	for _, item := range input.Roles {
		key := item.ClaimValue + "\x00" + item.Role
		if strings.TrimSpace(item.ClaimValue) == "" || len(item.ClaimValue) > 512 || !validOIDCRoleForRepository(item.Role) || item.Priority < -100000 || item.Priority > 100000 || seenRoles[key] {
			return service.ErrEnterpriseIdentityInvalid
		}
		seenRoles[key] = true
	}
	for _, item := range input.Teams {
		key := item.ClaimValue + "\x00" + fmt.Sprint(item.TeamID)
		if strings.TrimSpace(item.ClaimValue) == "" || len(item.ClaimValue) > 512 || item.TeamID <= 0 || seenTeams[key] {
			return service.ErrEnterpriseIdentityInvalid
		}
		seenTeams[key] = true
		var id int64
		if err = tx.QueryRowContext(ctx, `SELECT id FROM workspace_teams WHERE workspace_id=$1 AND id=$2 AND status='active'`, workspaceID, item.TeamID).Scan(&id); err != nil {
			return enterpriseIdentityError(err)
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM workspace_identity_role_mappings WHERE workspace_id=$1 AND provider_id=$2`, workspaceID, providerID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM workspace_identity_team_mappings WHERE workspace_id=$1 AND provider_id=$2`, workspaceID, providerID); err != nil {
		return err
	}
	for _, item := range input.Roles {
		if _, err = tx.ExecContext(ctx, `INSERT INTO workspace_identity_role_mappings(workspace_id,provider_id,claim_value,role,priority,created_by_user_id) VALUES($1,$2,$3,$4,$5,$6)`, workspaceID, providerID, item.ClaimValue, item.Role, item.Priority, actorID); err != nil {
			return enterpriseIdentityError(err)
		}
	}
	for _, item := range input.Teams {
		if _, err = tx.ExecContext(ctx, `INSERT INTO workspace_identity_team_mappings(workspace_id,provider_id,claim_value,team_id,created_by_user_id) VALUES($1,$2,$3,$4,$5)`, workspaceID, providerID, item.ClaimValue, item.TeamID, actorID); err != nil {
			return enterpriseIdentityError(err)
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE workspace_identity_providers SET revision=revision+1,last_validated_at=NULL,last_validation_code=NULL,updated_at=now() WHERE workspace_id=$1 AND id=$2`, workspaceID, providerID); err != nil {
		return err
	}
	if err = appendIdentityMutation(ctx, tx, workspaceID, actorID, "identity.mappings_updated", service.EventOIDCMappingsUpdated, "identity_provider", providerID, service.DomainEventData{"category": provider.Type, "provider_id": providerID, "role_count": len(input.Roles), "team_count": len(input.Teams)}); err != nil {
		return err
	}
	return enterpriseIdentityError(tx.Commit())
}

func (r *enterpriseIdentityRepository) DiscoverSSO(ctx context.Context, domain string) ([]service.SSODiscoveryProvider, error) {
	// This lookup is based only on a public verified domain. It never searches
	// users or exposes membership, policies, client IDs or provider endpoints.
	rows, err := r.db.QueryContext(ctx, `SELECT p.workspace_id,p.id,p.name,p.is_default,p.type,COALESCE(p.saml_public_id,'') FROM workspace_domains d JOIN workspaces w ON w.id=d.workspace_id AND w.type='organization' AND w.status='active' JOIN workspace_identity_providers p ON p.workspace_id=w.id AND p.status='active' WHERE d.normalized_domain=$1 AND d.status='verified' ORDER BY p.is_default DESC,p.id LIMIT 20`, domain)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []service.SSODiscoveryProvider{}
	for rows.Next() {
		var item service.SSODiscoveryProvider
		if err = rows.Scan(&item.WorkspaceID, &item.ProviderID, &item.Name, &item.IsDefault, &item.Type, &item.SAMLPublicID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *enterpriseIdentityRepository) HasUserIdentity(ctx context.Context, workspaceID, userID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_user_identities i JOIN workspace_identity_providers p ON p.workspace_id=i.workspace_id AND p.id=i.provider_id AND p.status='active' WHERE i.workspace_id=$1 AND i.user_id=$2)`, workspaceID, userID).Scan(&exists)
	return exists, err
}

// lockOIDCEmailIdentity uses the same advisory keys and ordering as ordinary
// registration. This prevents a concurrent registration or another tenant's
// JIT from creating a second global inbox identity.
func lockOIDCEmailIdentity(ctx context.Context, tx *sql.Tx, email string) error {
	for _, key := range normalizeLockKeys(normalizedEmailUniquenessLockKey(email), emailAliasUniquenessLockKey(email)) {
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, advisoryLockHash(key)); err != nil {
			return err
		}
	}
	return nil
}

func oidcEmailAlreadyExists(ctx context.Context, tx *sql.Tx, email string) (bool, error) {
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE lower(trim(email))=$1 AND deleted_at IS NULL)`, email).Scan(&exists); err != nil || exists {
		return exists, err
	}
	identity := service.NormalizeEmailForAliasDedup(email)
	for _, probe := range service.EmailAliasDedupProbes(email) {
		local := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(probe.Local)
		rows, err := tx.QueryContext(ctx, `SELECT email FROM users WHERE deleted_at IS NULL AND REPLACE(LOWER(TRIM(email)),'.','') LIKE $1 ESCAPE '\' LIMIT 1001`, local+"%@"+probe.Domain)
		if err != nil {
			return false, err
		}
		count := 0
		for rows.Next() {
			var candidate string
			if err = rows.Scan(&candidate); err != nil {
				_ = rows.Close()
				return false, err
			}
			count++
			if count > 1000 || service.NormalizeEmailForAliasDedup(candidate) == identity {
				_ = rows.Close()
				return true, nil
			}
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return false, err
		}
	}
	return false, nil
}

func (r *enterpriseIdentityRepository) CompleteIdentityLogin(ctx context.Context, input service.OIDCProvisionInput) (int64, error) {
	claims := input.Claims
	protocol := enterpriseProtocol(input.Protocol)
	if protocol != "oidc" && protocol != "saml" {
		return 0, service.ErrEnterpriseIdentityInvalid
	}
	if input.WorkspaceID <= 0 || input.ProviderID <= 0 || input.ProviderRevision <= 0 || claims == nil || strings.TrimSpace(claims.Subject) == "" || len(claims.Subject) > 1024 || len(claims.Email) > 320 || (input.LinkUserID != nil && *input.LinkUserID <= 0) {
		return 0, service.ErrEnterpriseIdentityInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockWorkspace(ctx, tx, input.WorkspaceID, true); err != nil {
		return 0, err
	}
	var workspaceType, workspaceStatus string
	if err = tx.QueryRowContext(ctx, `SELECT type,status FROM workspaces WHERE id=$1`, input.WorkspaceID).Scan(&workspaceType, &workspaceStatus); err != nil {
		return 0, enterpriseIdentityError(err)
	}
	if workspaceType != service.WorkspaceTypeOrganization || workspaceStatus != "active" {
		return 0, service.ErrWorkspaceForbidden
	}
	provider, _, err := getEnterpriseProvider(ctx, tx, input.WorkspaceID, input.ProviderID, true)
	if err != nil {
		return 0, err
	}
	if provider.Status != "active" {
		return 0, service.ErrOIDCProviderDisabled
	}
	if provider.Revision != input.ProviderRevision || provider.Type != protocol {
		return 0, service.ErrOIDCStateSessionMismatch
	}
	var userID, identityID int64
	err = tx.QueryRowContext(ctx, `SELECT id,user_id FROM workspace_user_identities WHERE workspace_id=$1 AND provider_id=$2 AND subject=$3 FOR UPDATE`, input.WorkspaceID, input.ProviderID, claims.Subject).Scan(&identityID, &userID)
	newIdentity, newUser := errors.Is(err, sql.ErrNoRows), false
	if err != nil && !newIdentity {
		return 0, enterpriseIdentityError(err)
	}
	if !newIdentity && input.LinkUserID != nil && userID != *input.LinkUserID {
		return 0, service.ErrOIDCAccountLinkRequired
	}
	if newIdentity {
		if input.LinkUserID != nil {
			// Only the authenticated user bound to the server-side state can be
			// linked. Existing tenant membership is required and never recreated.
			userID = *input.LinkUserID
		} else {
			email := strings.ToLower(strings.TrimSpace(claims.Email))
			parsed, parseErr := mail.ParseAddress(email)
			if parseErr != nil || parsed.Address != email || len(email) > 255 || !claims.EmailVerified || !provider.JITConfig.Enabled || input.PasswordHash == "" {
				return 0, service.ErrOIDCAccountLinkRequired
			}
			domainAt := strings.LastIndex(email, "@")
			if domainAt < 1 {
				return 0, service.ErrOIDCAccountLinkRequired
			}
			domain, domainErr := service.NormalizeEnterpriseDomain(email[domainAt+1:])
			if domainErr != nil {
				return 0, service.ErrOIDCAccountLinkRequired
			}
			var verified bool
			if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_domains WHERE workspace_id=$1 AND normalized_domain=$2 AND status='verified')`, input.WorkspaceID, domain).Scan(&verified); err != nil {
				return 0, err
			}
			if !verified || !provider.JITConfig.Allows(email, true, []string{domain}, true) {
				return 0, service.ErrOIDCAccountLinkRequired
			}
			if err = lockOIDCEmailIdentity(ctx, tx, email); err != nil {
				return 0, err
			}
			exists, checkErr := oidcEmailAlreadyExists(ctx, tx, email)
			if checkErr != nil {
				return 0, checkErr
			}
			if exists {
				return 0, service.ErrOIDCAccountLinkRequired
			}
			// The users insert trigger creates the personal workspace/default
			// project on this transaction, with no registration grants or keys.
			if err = tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,role,status,signup_source) VALUES($1,$2,'user','active',$3) RETURNING id`, email, input.PasswordHash, protocol).Scan(&userID); err != nil {
				return 0, enterpriseIdentityError(err)
			}
			newUser = true
		}
	}
	var userStatus, userEmail string
	var userDeleted sql.NullTime
	if err = tx.QueryRowContext(ctx, `SELECT status,deleted_at,email FROM users WHERE id=$1`, userID).Scan(&userStatus, &userDeleted, &userEmail); err != nil {
		return 0, enterpriseIdentityError(err)
	}
	if userStatus != "active" || userDeleted.Valid {
		return 0, service.ErrUserNotActive
	}
	if newIdentity && input.LinkUserID != nil && (!claims.EmailVerified || strings.TrimSpace(claims.Email) == "" || !strings.EqualFold(strings.TrimSpace(userEmail), strings.TrimSpace(claims.Email))) {
		return 0, service.ErrOIDCAccountLinkRequired
	}
	defaultRole := provider.JITConfig.DefaultRole
	if !validOIDCRoleForRepository(defaultRole) {
		defaultRole = service.WorkspaceRoleViewer
	}
	if newUser {
		if _, err = tx.ExecContext(ctx, `INSERT INTO workspace_members(workspace_id,user_id,role,status,membership_source,membership_provider_id) VALUES($1,$2,$3,'active',$5,$4)`, input.WorkspaceID, userID, defaultRole, input.ProviderID, protocol); err != nil {
			return 0, enterpriseIdentityError(err)
		}
	}
	var memberID int64
	var role, memberStatus, source string
	var sourceProvider sql.NullInt64
	var administrativelySuspended bool
	if err = tx.QueryRowContext(ctx, `SELECT id,role,status,membership_source,membership_provider_id,administratively_suspended FROM workspace_members WHERE workspace_id=$1 AND user_id=$2 FOR UPDATE`, input.WorkspaceID, userID).Scan(&memberID, &role, &memberStatus, &source, &sourceProvider, &administrativelySuspended); err != nil {
		return 0, enterpriseIdentityError(err)
	}
	var existingActiveSource bool
	if memberStatus != "active" {
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_membership_sources WHERE workspace_id=$1 AND member_id=$2 AND active)`, input.WorkspaceID, memberID).Scan(&existingActiveSource); err != nil {
			return 0, err
		}
	}
	if administrativelySuspended || existingActiveSource || (newIdentity || input.LinkUserID != nil) && memberStatus != "active" {
		return 0, service.ErrWorkspaceForbidden
	}
	var currentSourceRole string
	sourceErr := tx.QueryRowContext(ctx, `SELECT role FROM workspace_membership_sources WHERE workspace_id=$1 AND member_id=$2 AND provider_id=$3 AND source_type=$4`, input.WorkspaceID, memberID, input.ProviderID, protocol).Scan(&currentSourceRole)
	if sourceErr == sql.ErrNoRows {
		currentSourceRole = defaultRole
	} else if sourceErr != nil {
		return 0, sourceErr
	}
	if err = upsertProviderMemberSource(ctx, tx, input.WorkspaceID, memberID, input.ProviderID, protocol, currentSourceRole); err != nil {
		return 0, err
	}
	if err = reconcileWorkspaceMemberSources(ctx, tx, input.WorkspaceID, memberID); err != nil {
		return 0, err
	}
	if newIdentity {
		if err = tx.QueryRowContext(ctx, `INSERT INTO workspace_user_identities(workspace_id,provider_id,user_id,subject,email_at_link,email_verified,display_name,last_seen_at) VALUES($1,$2,$3,$4,NULLIF($5,''),$6,NULLIF($7,''),now()) RETURNING id`, input.WorkspaceID, input.ProviderID, userID, claims.Subject, claims.Email, claims.EmailVerified, boundedOIDCName(claims.Name)).Scan(&identityID); err != nil {
			return 0, enterpriseIdentityError(err)
		}
		action, event := "identity.linked", service.EventOIDCIdentityLinked
		if newUser {
			action, event = "identity.jit_provisioned", service.EventOIDCJITProvisioned
		}
		if err = appendIdentityMutation(ctx, tx, input.WorkspaceID, userID, action, event, "identity", identityID, service.DomainEventData{"user_id": userID, "member_id": memberID, "role": role, "category": protocol}); err != nil {
			return 0, err
		}
	} else {
		if _, err = tx.ExecContext(ctx, `UPDATE workspace_user_identities SET last_seen_at=now(),email_verified=$4,display_name=NULLIF($5,''),updated_at=now() WHERE workspace_id=$1 AND provider_id=$2 AND id=$3`, input.WorkspaceID, input.ProviderID, identityID, claims.EmailVerified, boundedOIDCName(claims.Name)); err != nil {
			return 0, err
		}
	}
	// Missing/distributed/overage claims preserve prior managed grants. A
	// complete empty claim is authoritative and resets only this provider.
	if claims.GroupsPresent && claims.GroupsComplete {
		if len(claims.Groups) > 200 {
			return 0, service.ErrEnterpriseIdentityInvalid
		}
		mappings, mappingErr := getOIDCMappings(ctx, tx, input.WorkspaceID, input.ProviderID)
		if mappingErr != nil {
			return 0, mappingErr
		}
		changed := false
		nextRole, _ := service.MapOIDCRole(claims.Groups, mappings.Roles, defaultRole, false)
		if err = upsertProviderMemberSource(ctx, tx, input.WorkspaceID, memberID, input.ProviderID, protocol, nextRole); err != nil {
			return 0, err
		}
		if err = reconcileWorkspaceMemberSources(ctx, tx, input.WorkspaceID, memberID); err != nil {
			return 0, err
		}
		var effectiveRole string
		if err = tx.QueryRowContext(ctx, `SELECT role FROM workspace_members WHERE workspace_id=$1 AND id=$2`, input.WorkspaceID, memberID).Scan(&effectiveRole); err != nil {
			return 0, err
		}
		changed = role != effectiveRole
		role = effectiveRole
		teamChanged, teamErr := reconcileOIDCTeams(ctx, tx, input.WorkspaceID, input.ProviderID, memberID, claims.Groups)
		if teamErr != nil {
			return 0, teamErr
		}
		data := service.DomainEventData{"user_id": userID, "member_id": memberID, "role": role, "provider_id": input.ProviderID, "source_provider_id": input.ProviderID, "category": protocol}
		if changed {
			if err = appendIdentityMutation(ctx, tx, input.WorkspaceID, userID, "identity.role_reconciled", service.EventOIDCRoleReconciled, "identity", identityID, data); err != nil {
				return 0, err
			}
		}
		if teamChanged {
			if err = appendIdentityMutation(ctx, tx, input.WorkspaceID, userID, "identity.teams_reconciled", service.EventOIDCTeamsReconciled, "identity", identityID, data); err != nil {
				return 0, err
			}
		}
	}
	return userID, enterpriseIdentityError(tx.Commit())
}

func boundedOIDCName(value string) string {
	// Display names have no role in identity or access decisions.
	runes := []rune(value)
	if len(runes) > 512 {
		runes = runes[:512]
	}
	return string(runes)
}

func reconcileOIDCTeams(ctx context.Context, tx *sql.Tx, workspaceID, providerID, memberID int64, groups []string) (bool, error) {
	if groups == nil {
		groups = []string{}
	}
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT m.team_id FROM workspace_identity_team_mappings m JOIN workspace_teams t ON t.workspace_id=m.workspace_id AND t.id=m.team_id AND t.status='active' WHERE m.workspace_id=$1 AND m.provider_id=$2 AND m.enabled AND m.claim_value=ANY($3) ORDER BY m.team_id LIMIT 201`, workspaceID, providerID, pq.Array(groups))
	if err != nil {
		return false, err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			_ = rows.Close()
			return false, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return false, err
	}
	if len(ids) > 200 {
		return false, service.ErrEnterpriseIdentityInvalid
	}
	changed := false
	result, err := tx.ExecContext(ctx, `DELETE FROM workspace_identity_team_grants WHERE workspace_id=$1 AND provider_id=$2 AND workspace_member_id=$3 AND NOT (team_id=ANY($4))`, workspaceID, providerID, memberID, pq.Array(ids))
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	changed = count > 0
	for _, id := range ids {
		result, err = tx.ExecContext(ctx, `INSERT INTO workspace_identity_team_grants(workspace_id,provider_id,team_id,workspace_member_id) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, workspaceID, providerID, id, memberID)
		if err != nil {
			return false, err
		}
		count, err = result.RowsAffected()
		if err != nil {
			return false, err
		}
		changed = changed || count > 0
		if _, err = tx.ExecContext(ctx, `INSERT INTO workspace_team_membership_sources(workspace_id,member_id,team_id,source_type,provider_id) SELECT $1,$3,$2,type,$4 FROM workspace_identity_providers WHERE workspace_id=$1 AND id=$4 ON CONFLICT(workspace_id,member_id,team_id,provider_id) DO UPDATE SET active=true`, workspaceID, id, memberID, providerID); err != nil {
			return false, err
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM workspace_team_membership_sources WHERE workspace_id=$1 AND member_id=$2 AND provider_id=$3 AND source_type IN('oidc','saml') AND NOT(team_id=ANY($4))`, workspaceID, memberID, providerID, pq.Array(ids)); err != nil {
		return false, err
	}
	if err = reconcileWorkspaceTeamSources(ctx, tx, workspaceID, memberID); err != nil {
		return false, err
	}

	return changed, nil
}

func (r *enterpriseIdentityRepository) BreakGlass(ctx context.Context, workspaceID, actorID int64, reason string) (*service.WorkspaceIdentityPolicy, error) {
	if strings.TrimSpace(reason) == "" || len(reason) > 500 {
		return nil, service.ErrEnterpriseIdentityInvalid
	}
	tx, access, err := r.beginScoped(ctx, workspaceID, actorID, "workspace_sso.update")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if access.Workspace.Status != "active" || access.Member.Role != service.WorkspaceRoleOwner || access.Member.Status != "active" {
		return nil, service.ErrWorkspaceForbidden
	}
	var requireSSO bool
	if err = tx.QueryRowContext(ctx, `SELECT require_sso FROM workspace_security_policies WHERE workspace_id=$1 FOR UPDATE`, workspaceID).Scan(&requireSSO); err != nil {
		return nil, enterpriseIdentityError(err)
	}
	if !requireSSO {
		return nil, service.ErrWorkspaceConflict
	}
	var lastUsed time.Time
	err = tx.QueryRowContext(ctx, `INSERT INTO workspace_identity_break_glass_limits(workspace_id,last_used_at,actor_user_id) VALUES($1,now(),$2) ON CONFLICT(workspace_id) DO UPDATE SET last_used_at=now(),actor_user_id=EXCLUDED.actor_user_id WHERE workspace_identity_break_glass_limits.last_used_at<=now()-interval '15 minutes' RETURNING last_used_at`, workspaceID, actorID).Scan(&lastUsed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrWorkspaceConflict
	}
	if err != nil {
		return nil, err
	}
	item, err := writeIdentityPolicy(ctx, tx, workspaceID, actorID, false, nil)
	if err != nil {
		return nil, err
	}
	if err = appendWorkspaceAudit(ctx, tx, workspaceID, actorID, nil, "identity.break_glass", "workspace", workspaceID, map[string]any{"reason": strings.TrimSpace(reason), "policy_revision": item.Revision}); err != nil {
		return nil, err
	}
	event, err := service.NewDomainEvent(service.EventSSOBreakGlassUsed, workspaceID, 0, actorID, "workspace", fmt.Sprint(workspaceID), service.DomainEventData{"reason_code": "owner_recovery", "policy_revision": item.Revision})
	if err != nil {
		return nil, err
	}
	if err = insertDomainEventTx(ctx, tx, event, ""); err != nil {
		return nil, err
	}
	return item, enterpriseIdentityError(tx.Commit())
}

func (r *enterpriseIdentityRepository) CreateLoginCompletion(ctx context.Context, login *service.OIDCLoginResult, tokenHash, browserHash []byte, expiresAt time.Time) error {
	if login == nil || login.User == nil || login.User.ID <= 0 || len(tokenHash) != 32 || len(browserHash) != 32 || service.ValidateEnterpriseReturnTo(login.ReturnTo) != nil || login.Assurance.AuthenticatedAt.IsZero() || login.Assurance.WorkspaceID != login.Workspace || login.Assurance.ProviderID != login.ProviderID || login.Assurance.ProviderRevision <= 0 || (login.Assurance.AuthMethod != "oidc" && login.Assurance.AuthMethod != "saml") {
		return service.ErrEnterpriseIdentityInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockWorkspace(ctx, tx, login.Workspace, false); err != nil {
		return err
	}
	provider, _, err := getEnterpriseProvider(ctx, tx, login.Workspace, login.ProviderID, false)
	if err != nil {
		return err
	}
	if provider.Status != "active" {
		return service.ErrOIDCProviderDisabled
	}
	if provider.Revision != login.Assurance.ProviderRevision || provider.Type != login.Assurance.AuthMethod {
		return service.ErrOIDCStateSessionMismatch
	}
	if err = cleanupExpiredIdentityAuthentication(ctx, tx); err != nil {
		return err
	}
	// Binding the current revision prevents config edits or disable/enable from
	// turning an old completion into fresh assurance.
	result, err := tx.ExecContext(ctx, `INSERT INTO workspace_identity_login_completions(token_hash,browser_session_hash,workspace_id,provider_id,provider_revision,user_id,return_to,authenticated_at,expires_at,auth_method) SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10 FROM workspaces w JOIN workspace_members m ON m.workspace_id=w.id AND m.user_id=$6 AND m.status='active' JOIN users u ON u.id=m.user_id AND u.status='active' AND u.deleted_at IS NULL WHERE w.id=$3 AND w.type='organization' AND w.status='active'`, tokenHash, browserHash, login.Workspace, login.ProviderID, provider.Revision, login.User.ID, login.ReturnTo, login.Assurance.AuthenticatedAt, expiresAt, provider.Type)
	if err != nil {
		return enterpriseIdentityError(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return service.ErrOIDCStateSessionMismatch
	}
	return enterpriseIdentityError(tx.Commit())
}

func (r *enterpriseIdentityRepository) ConsumeLoginCompletion(ctx context.Context, tokenHash, browserHash []byte, now time.Time) (*service.OIDCLoginResult, int64, error) {
	if len(tokenHash) != 32 || len(browserHash) != 32 {
		return nil, 0, service.ErrOIDCStateSessionMismatch
	}
	return scanIdentityLoginCompletion(r.db.QueryRowContext(ctx, `UPDATE workspace_identity_login_completions c SET consumed_at=$3 WHERE `+enterpriseCompletionIsLive+` RETURNING `+enterpriseCompletionProjection, tokenHash, browserHash, now))
}

const enterpriseCompletionIsLive = `c.token_hash=$1 AND c.browser_session_hash=$2 AND c.consumed_at IS NULL AND c.expires_at>$3 AND EXISTS(SELECT 1 FROM workspace_identity_providers p JOIN workspaces w ON w.id=p.workspace_id AND w.type='organization' AND w.status='active' JOIN workspace_members m ON m.workspace_id=w.id AND m.user_id=c.user_id AND m.status='active' JOIN users u ON u.id=m.user_id AND u.status='active' AND u.deleted_at IS NULL WHERE p.workspace_id=c.workspace_id AND p.id=c.provider_id AND p.revision=c.provider_revision AND p.type=c.auth_method AND p.status='active')`
const enterpriseCompletionProjection = `c.workspace_id,c.provider_id,c.provider_revision,c.user_id,c.return_to,c.authenticated_at,c.auth_method`

func (r *enterpriseIdentityRepository) PreviewLoginCompletion(ctx context.Context, tokenHash, browserHash []byte, now time.Time) (*service.OIDCLoginResult, int64, error) {
	if len(tokenHash) != 32 || len(browserHash) != 32 {
		return nil, 0, service.ErrOIDCStateSessionMismatch
	}
	return scanIdentityLoginCompletion(r.db.QueryRowContext(ctx, `SELECT `+enterpriseCompletionProjection+` FROM workspace_identity_login_completions c WHERE `+enterpriseCompletionIsLive, tokenHash, browserHash, now))
}

func scanIdentityLoginCompletion(scanner workspaceScanner) (*service.OIDCLoginResult, int64, error) {
	login := &service.OIDCLoginResult{}
	var userID, revision int64
	err := scanner.Scan(&login.Workspace, &login.ProviderID, &revision, &userID, &login.ReturnTo, &login.Assurance.AuthenticatedAt, &login.Assurance.AuthMethod)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, service.ErrOIDCStateSessionMismatch
	}
	if err != nil {
		return nil, 0, err
	}
	login.Assurance.WorkspaceID, login.Assurance.ProviderID, login.Assurance.ProviderRevision = login.Workspace, login.ProviderID, revision
	return login, userID, nil
}
