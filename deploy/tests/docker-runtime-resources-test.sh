#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo_root"

fail() {
  printf 'docker runtime resources test failed: %s\n' "$1" >&2
  exit 1
}

assert_line() {
  file=$1
  line=$2
  grep -Fqx "$line" "$file" || fail "$file is missing: $line"
}

assert_count() {
  file=$1
  line=$2
  expected=$3
  actual=$(grep -Fxc "$line" "$file" || true)
  [ "$actual" -eq "$expected" ] || fail "$file has $actual occurrences of '$line', expected $expected"
}

assert_absent() {
  file=$1
  text=$2
  if grep -Fq "$text" "$file"; then
    fail "$file must not contain: $text"
  fi
}

test -s backend/resources/model-pricing/model_prices_and_context_window.json || \
  fail 'fallback pricing data is missing or empty'

assert_line Dockerfile.goreleaser 'COPY --chown=sub2api:sub2api backend/resources /app/resources'
assert_line deploy/Dockerfile 'COPY --from=backend-builder --chown=sub2api:sub2api /app/backend/resources /app/resources'
assert_line .goreleaser.yaml '      - -X main.DeploymentMode=binary'
assert_line .goreleaser.simple.yaml '      - -X main.DeploymentMode=binary'
assert_absent .goreleaser.yaml 'dockers:'
assert_absent .goreleaser.simple.yaml 'dockers:'
assert_absent .github/workflows/release.yml 'packages: write'
assert_absent .github/workflows/release.yml '/sub2api:'
assert_line .github/workflows/publish-main-image.yml '          if [ "${{ github.event_name }}" = "workflow_dispatch" ] && [ "${{ github.ref }}" != "refs/heads/main" ]; then'
assert_line .github/workflows/publish-main-image.yml '    uses: ./.github/workflows/release.yml'
assert_line .github/workflows/release.yml '  workflow_call:'
assert_line .github/workflows/release.yml '      - name: Create release tag'

printf 'docker runtime resources test passed\n'
