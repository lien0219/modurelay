# Enterprise SAML 2.0

Phase C1 extends the Phase B identity-provider parent and shared SSO pipeline. This document is the implementation and operational contract; validation results are recorded in the roadmap and ENTERPRISE_SAML_ACCEPTANCE.md.

## Architecture gate

The audited baseline is feature/new-feature at 81e68865463755f3fb684199b3de5a31e8572713. Migration 294 only supports OIDC. Existing assurance carries tenant/provider/revision/original authentication time but completion and enforcement are OIDC-specific. Stable workspace_user_identities, JIT, explicit account linking, role priorities, Team mappings, owner recovery, JWT/Redis refresh and transactional audit/event/outbox are reused.

Migration 295 makes protocol-specific OIDC columns nullable and adds SAML configuration plus an independent encrypted SP-key field and opaque public-provider identifier. Protocol CHECKs require genuine OIDC values for OIDC and genuine IdP entity/HTTPS SSO/certificates/encrypted key for SAML. SAML never stores placeholder issuer, client ID or OAuth authentication fields. Existing OIDC rows and historical Usage, Billing, Key and Service Account rows are preserved.

Provider type is immutable. After a subject binding exists, the subject namespace cannot be changed in place. Persistent NameID is the default stable key; transient/email NameID is rejected unless an independent stable attribute is explicitly configured. Unspecified NameID requires explicit administrator confirmation of stability.

SAML proof consists of a validated trusted signature, configured email attribute and verified Workspace domain. A new JIT user additionally requires JIT/allowed-domain approval. An existing global email requires authenticated explicit linking with recent authentication/TOTP; email equality alone never adopts an account. Owner and Billing Owner remain manually managed.

SSO assurance uses one enforcement engine: Workspace, Provider, type, revision, active state and original authentication time must match. OIDC and SAML may coexist and share one active default. Machine credentials remain outside browser SSO.

## Dependency decision

The implementation selects github.com/russellhaering/gosaml2 v0.12.0 (2026-08-05 release, Apache-2.0) and github.com/russellhaering/goxmldsig v1.6.1 (2026-08-04, Apache-2.0). Both compile against the repository's Go 1.27. The library provides metadata, signed AuthnRequests, XML signature/canonicalization and encrypted assertion decryption. XML roundtrip validation and bounded token parsing are included upstream. No XML DSig or canonicalization is implemented by ModuRelay.

The older crewjam/saml v0.5.1 candidate was reviewed but not selected: its latest tag dates to April 2025 and its default dependency uses goxmldsig v1.4.0. The selected goxmldsig version includes fixes for GO-2026-4753. Exact-version OSV queries for gosaml2 v0.12.0 and goxmldsig v1.6.1 returned no known advisories on 2026-10-07. Full dependency audit/build results are recorded at the phase gate; this is a dated result, not a permanent absence-of-vulnerability guarantee.

## Protocol and security contract

- SP-initiated HTTP-Redirect AuthnRequest and HTTP-POST ACS are supported. Requests are signed using RSA-SHA256.
- RelayState is an opaque high-entropy server-hashed token with bounded TTL, one-time consumption and an HttpOnly browser binding. The state freezes Workspace, Provider/revision, request ID, return path and authenticated linking intent.
- Response/Assertion IDs have independent atomic replay protection. Consuming RelayState alone is insufficient.
- Responses require a valid trusted Response or Assertion signature. Completely unsigned responses, signature wrapping, duplicate IDs, multiple assertions and malformed XML are rejected.
- Application validation additionally requires exact issuer, Destination, Recipient, InResponseTo, audience, bearer subject confirmation, success status, bounded issue/condition times and stable subject. Unknown/unsolicited requests cannot log in.
- Metadata and response bodies, token counts, nesting, attributes, endpoints and certificates are bounded. DTD/directives, external entities and expansion are rejected. Metadata URLs use the existing HTTPS/DNS-pinned/no-proxy/no-redirect transport.
- Multiple trusted IdP signing certificates support rollover. Expired/not-yet-valid/unsupported weak keys fail configuration validation. SP private keys are per-provider and encrypted with the persistent SecretEncryptor key; public metadata contains only certificates and SP endpoints.
- SP metadata uses an opaque public provider identifier. Entity ID and ACS derive from the retained configured enterprise callback origin, never attacker Host headers.
- Login completes through Phase B's same-origin, single-use exchange and local TOTP gate. Callback URLs never contain JWTs or raw assertions. Refresh retains original authentication/assurance time.
- Provider disable/revision changes invalidate that Workspace assurance; Global Session remains intact. Existing Owner recovery is the sole recovery path.

## Operational boundaries

Retain and back up the application encryption key and configured enterprise callback origin. Losing the key prevents decrypting SP keys and identity state. Register the exact generated Entity ID, ACS and metadata URL with the IdP. Keep old/new IdP certificates together during rollover and complete SP-key metadata changes before promotion.

IdP-initiated SSO is NOT SUPPORTED in Phase C1. SAML Single Logout is DEFERRED; local ModuRelay logout is authoritative. HTTP-POST AuthnRequest is deferred when a provider has no HTTP-Redirect endpoint. Real Entra, Okta, Google/other SAML registration, browser/operator recovery drill and load acceptance remain Phase L and NOT RUN unless separately evidenced.

## Audit and isolation

Provider create/update/disable, metadata/certificate lifecycle and common identity/mapping changes write bounded scalar audit/event/outbox records. Raw assertion, signed XML, private keys and credentials are forbidden in those payloads and logs. All control-plane mutations repeat central tenant RBAC under the Workspace lock. Shared Team mappings continue to Phase A Project Access Grants; SAML creates no separate Project ACL.

### gosaml2 decryption integration

The generated signed/encrypted IdP fixture exposed that v0.12.0 `getDecryptCert` still reads the deprecated `SPKeyStore` field instead of the `SetSPKeyStore` override. The SP supplies the identical encrypted-at-rest RSA key/certificate through both APIs; the legacy field is used only to connect the library's maintained decryption path. This has a narrow staticcheck suppression with an explanatory comment. AES-GCM/RSA-OAEP fixtures verify current, staged-next and retained-previous SP keys, wrong-key and unsigned encrypted assertions. No signature verification, canonicalization or production decryption was implemented locally.

SAML browser roundtrips require an HTTPS enterprise callback: the cross-site POST binding cookie is HttpOnly, Secure, SameSite=None and scoped to the ACS. Completion switches to the existing SameSite=Lax cookies, same-origin exchange and local TOTP flow. SP metadata may be read publicly, but browser login refuses an HTTP callback.
