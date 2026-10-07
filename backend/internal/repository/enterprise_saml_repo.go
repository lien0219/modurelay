package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

var _ service.SAMLReplayRepository = (*enterpriseIdentityRepository)(nil)
var _ service.SAMLKeyRepository = (*enterpriseIdentityRepository)(nil)

func enterpriseProtocol(protocol string) string {
	if protocol == "" {
		return "oidc"
	}
	return protocol
}

func validSAMLRepositoryInput(input service.EnterpriseIdentityProviderInput) bool {
	return input.SAML != nil && input.IssuerURL == "" && input.ClientID == "" && input.ClientSecret == nil && input.TokenAuthMethod == "" && input.AuthorizationEndpoint == "" && input.TokenEndpoint == "" && input.JWKSURI == "" && input.UserinfoEndpoint == "" && len(input.Scopes) == 0 && (input.SecretAction == "" || input.SecretAction == "preserve") && (input.DiscoveryEnabled == nil || !*input.DiscoveryEnabled)
}

func sanitizedSAMLConfig(config *service.SAMLProviderConfig) ([]byte, error) {
	if config == nil {
		return nil, service.ErrEnterpriseIdentityInvalid
	}
	copy := *config
	copy.MetadataXML = ""
	return json.Marshal(copy)
}

func (r *enterpriseIdentityRepository) GetSAMLProviderByPublicID(ctx context.Context, publicID string) (*service.EnterpriseIdentityProvider, string, error) {
	if len(publicID) != 43 {
		return nil, "", service.ErrWorkspaceNotFound
	}
	var ciphertext sql.NullString
	provider, _, err := scanEnterpriseProvider(r.db.QueryRowContext(ctx, `SELECT `+enterpriseProviderColumns+`,encrypted_saml_sp_keys FROM workspace_identity_providers WHERE saml_public_id=$1 AND type='saml' AND status='active' AND workspace_id IN (SELECT id FROM workspaces WHERE type='organization' AND status='active')`, publicID), &ciphertext)
	if err != nil {
		return nil, "", err
	}
	return provider, ciphertext.String, nil
}

func (r *enterpriseIdentityRepository) UseSAMLResponse(ctx context.Context, workspaceID, providerID int64, responseID, assertionID string, expiresAt time.Time) error {
	now := time.Now().UTC()
	if workspaceID <= 0 || providerID <= 0 || strings.TrimSpace(responseID) == "" || strings.TrimSpace(assertionID) == "" || len(responseID) > 1024 || len(assertionID) > 1024 || !expiresAt.After(now) || expiresAt.After(now.Add(24*time.Hour)) {
		return service.ErrEnterpriseIdentityInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockWorkspace(ctx, tx, workspaceID, false); err != nil {
		return err
	}
	var live bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_identity_providers p JOIN workspaces w ON w.id=p.workspace_id WHERE p.workspace_id=$1 AND p.id=$2 AND p.type='saml' AND p.status='active' AND w.type='organization' AND w.status='active')`, workspaceID, providerID).Scan(&live); err != nil {
		return err
	}
	if !live {
		return service.ErrOIDCProviderDisabled
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM workspace_identity_saml_replays WHERE id IN(SELECT id FROM workspace_identity_saml_replays WHERE expires_at<now() ORDER BY expires_at,id LIMIT 100 FOR UPDATE SKIP LOCKED)`); err != nil {
		return err
	}
	// A single transaction records both IDs, so a collision on the assertion
	// rolls back the response ID as well. IDs share a namespace: an assertion
	// must not be replayable under the response position.
	_, err = tx.ExecContext(ctx, `INSERT INTO workspace_identity_saml_replays(workspace_id,provider_id,id_hash,expires_at) VALUES($1,$2,$3,$5),($1,$2,$4,$5)`, workspaceID, providerID, service.HashEnterpriseToken(responseID), service.HashEnterpriseToken(assertionID), expiresAt)
	if err != nil {
		var constraint *pq.Error
		if errors.As(err, &constraint) && constraint.Code == "23505" {
			return service.ErrSAMLReplayDetected
		}
		return enterpriseIdentityError(err)
	}
	return enterpriseIdentityError(tx.Commit())
}

func (r *enterpriseIdentityRepository) UpdateSAMLKeys(ctx context.Context, workspaceID, actorID, providerID, revision int64, ciphertext, currentCert, nextCert string) (*service.EnterpriseIdentityProvider, error) {
	if revision <= 0 || ciphertext == "" || strings.TrimSpace(currentCert) == "" || len(currentCert) > 16384 || len(nextCert) > 16384 {
		return nil, service.ErrEnterpriseIdentityInvalid
	}
	tx, _, err := r.beginScoped(ctx, workspaceID, actorID, "identity.manage")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	provider, _, err := getEnterpriseProvider(ctx, tx, workspaceID, providerID, true)
	if err != nil {
		return nil, err
	}
	if provider.Type != "saml" || provider.Revision != revision || provider.SAML == nil {
		return nil, service.ErrWorkspaceConflict
	}
	if provider.Status != "active" {
		return nil, service.ErrOIDCProviderDisabled
	}
	config := *provider.SAML
	config.SPCertificate, config.NextSPCertificate = currentCert, nextCert
	configJSON, err := sanitizedSAMLConfig(&config)
	if err != nil {
		return nil, err
	}
	item, _, err := scanEnterpriseProvider(tx.QueryRowContext(ctx, `UPDATE workspace_identity_providers SET saml_config=$3,encrypted_saml_sp_keys=$4,revision=revision+1,last_validated_at=NULL,last_validation_code=NULL,updated_at=now() WHERE workspace_id=$1 AND id=$2 RETURNING `+enterpriseProviderColumns, workspaceID, providerID, configJSON, ciphertext))
	if err != nil {
		return nil, err
	}
	if err = appendIdentityMutation(ctx, tx, workspaceID, actorID, "identity.saml_keys_rotated", service.EventSAMLCertificateRotated, "identity_provider", providerID, service.DomainEventData{"provider_id": providerID, "provider_revision": item.Revision, "category": "saml"}); err != nil {
		return nil, err
	}
	return item, enterpriseIdentityError(tx.Commit())
}
