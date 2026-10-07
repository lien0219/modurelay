# Phase C1 SAML acceptance

Baseline: `81e68865463755f3fb684199b3de5a31e8572713`, branch `feature/new-feature`.
Migration: `295_enterprise_identity_saml.sql`. Local phase commit:
`feat(saml): add enterprise SAML single sign-on` (resolve its SHA from Git history).

| Gate | Result | Evidence / boundary |
| --- | --- | --- |
| Provider/schema/OIDC compatibility | PASS | Protocol SQL integrity; mixed providers, scoped CRUD/revisions/defaults/redaction and repeat migration/history tests. |
| Real PostgreSQL identity/governance | PASS | Core/OIDC/SAML 16.979s; final SAML 7.360s; combined identity/governance 43.498s. |
| Local signed/encrypted IdP | PASS | Signed Redirect request and Response/Assertion verification; current/next/previous decryption keys, browser/state/replay correlation; 2.500s. |
| XML/metadata security | PASS | Bounded parser, XXE/directive rejection, duplicate/wrapped assertions, malformed input, HTTPS/private DNS/dial/redirect rejection and signing rollover. |
| Shared JIT/linking/mappings | PASS | Stable subjects, verified Workspace domain, no existing-email adoption, explicit linking, manual Owner/Billing protection, missing/empty groups and mixed Team attribution. |
| Browser/completion/assurance | PASS | Bounded ACS and Secure/HttpOnly/SameSite=None binding; same-origin completion/TOTP; original authentication time and protocol; revision/disable enforcement. |
| Optional attribute settings | PASS | Explicitly empty name/groups attributes remain disabled; SAML/service suite 2.277s after RED/GREEN correction. |
| Relevant backend race | PASS | Service 8.427s; handler 1.280s; repository 1.153s. |
| Frontend | PASS | i18n, lint, typecheck; 421 files / 3111 tests; production build 52.90s. Existing chunk-size warnings remain. |
| Go vet/build/pinned lint | PASS | `go vet ./...`, `go build ./...`; golangci-lint v2.13.0 reports 0 issues. |
| Dependency audit | PASS | govulncheck: 0 reachable and 0 imported-package vulnerabilities; 11 advisories in required modules not called by this application. Production pnpm audit exception checker validated. |
| Independent code/security review | PASS | Immutable v4 review snapshot; F0-F3 resolved, spec and code-quality approval. |
| Full backend default/unit/integration suites | PRE-EXISTING | Exact final commands reproduce only the three archived PgDumper missing-`sh` failures. Unit also reproduces the archived Ollama stale-callback failure. A parallel integration run briefly exposed a Phase B SSO timing failure; the required serial rerun passed that test, so it is not classified as a C1 regression. |
| External Entra/Okta/Google SAML | NOT RUN | Phase L registration/provider interoperability. |
| Rendered browser/operator/load/deployment | NOT RUN | Phase L. Acceptance runtime at port 18081 remains unchanged. |

## Baseline comparison

Archived exact baseline `.cache/phase-c/baseline` ran with `GOMAXPROCS=4`:
`go test ./... -count=1 -timeout=10m`, `go test -tags=unit ./... -count=1 -timeout=10m`,
and `go test -tags=integration ./... -count=1 -timeout=10m`.
Default/integration baseline fail three PgDumper tests because Windows lacks `sh`.
Unit baseline additionally fails `TestOllamaProbeCallback_StaleLongDoesNotOverrideNewShort`.
Classify current failures only after identical final commands.

## Security and operations

SAML uses genuine protocol configuration. Unsigned assertions, transient/email-only
permanent subjects, unsolicited responses, stale/mismatched requests and replayed IDs
fail closed. Existing Global Users require explicit linking. Public metadata never
returns private keys; encrypted SP key rotation retains previous decryption keys for
15 minutes. Audit/events contain bounded scalars. Retain the application encryption
key and exact HTTPS callback configuration.

IdP-initiated SSO, SLO and POST-only AuthnRequest providers are deferred explicitly.
Local ModuRelay logout remains authoritative. No Gateway admission, billing or historical
Usage/Key/Service Account data was rewritten.
