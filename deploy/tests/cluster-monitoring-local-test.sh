#!/usr/bin/env bash
# Local repeatable checks; never changes a deployment or sends a notification.
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root/backend"
go test -tags=unit ./internal/config ./internal/pkg/httpmetrics ./internal/server/middleware ./internal/server/routes ./internal/service ./internal/repository ./internal/handler/admin -run 'Test.*(Ops|Monitoring|Metrics|ObserveCounter|MetricDimensions|ExcludedPaths|UsageBilling|ReadReplicaKeepsBilling|LoggerRecordsInstance|LoggerMarksIngress|RequestAccess)' -count=1
cd "$repo_root/frontend"
pnpm exec vitest run src/views/admin/ops src/i18n/__tests__/localeKeyCompleteness.spec.ts
pnpm typecheck
cd "$repo_root"
bash deploy/tests/docker-compose-cluster-env-test.sh
bash deploy/tests/docker-compose-security-test.sh
