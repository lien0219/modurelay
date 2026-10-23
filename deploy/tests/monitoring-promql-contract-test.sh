#!/usr/bin/env bash
set -euo pipefail

# Runs the actual query builder against a pinned official Prometheus parser and
# execution engine without adding any dependency to the application module.
# The first run downloads the separate test module's dependencies via Go.
monitoring_repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
monitoring_fixture_dir="$monitoring_repo_root/deploy/tests/fixtures/monitoring-promql"
monitoring_temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/modurelay-promql-contract.XXXXXX")
trap 'rm -rf -- "$monitoring_temp_dir"' EXIT

for monitoring_source in ops_monitoring_models.go ops_monitoring_promql.go; do
  sed 's/^package service$/package main/' \
    "$monitoring_repo_root/backend/internal/service/$monitoring_source" \
    > "$monitoring_temp_dir/$monitoring_source"
done
cp "$monitoring_fixture_dir/main.go" "$monitoring_fixture_dir/semantics_test.go" "$monitoring_temp_dir/"
cat > "$monitoring_temp_dir/go.mod" <<'MODULE'
module modurelay-monitoring-promql-contract

go 1.27.3

require (
    github.com/prometheus/prometheus v0.315.1
    gopkg.in/yaml.v3 v3.0.1
)
MODULE

cd -- "$monitoring_temp_dir"
export GOWORK=off
go mod tidy
go run . "$monitoring_repo_root/deploy/prometheus.monitoring.rules.yml"
go test -run '^TestMonitoringPromQLSemantics$' -count=1
