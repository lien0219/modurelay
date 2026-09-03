#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo_root"

check_application_security_opt() {
  file=$1
  service=$2
  count=$(
    awk -v service="$service" '
      $0 == "  " service ":" {
        in_application = 1
        next
      }
      in_application && $0 ~ /^  [A-Za-z0-9_-]+:$/ {
        in_application = 0
      }
      in_application && $0 == "    security_opt:" {
        in_security_opt = 1
        next
      }
      in_application && in_security_opt && $0 == "      - no-new-privileges:true" {
        count++
      }
      END { print count + 0 }
    ' "$file"
  )

  if [ "$count" -ne 1 ]; then
    printf '%s must enable no-new-privileges exactly once for the %s service\n' "$file" "$service" >&2
    exit 1
  fi
}

for compose_file in \
  deploy/docker-compose.yml \
  deploy/docker-compose.local.yml \
  deploy/docker-compose.standalone.yml \
  deploy/docker-compose.dev.yml
do
  check_application_security_opt "$compose_file" sub2api
done

check_application_security_opt deploy/docker-compose.prod.yml modurelay

prod_policy_count=$(grep -Fxc '      REDIS_MAXMEMORY_POLICY: "${REDIS_MAXMEMORY_POLICY:-noeviction}"' deploy/docker-compose.prod.yml || true)
prod_command_count=$(grep -Fxc '        --maxmemory-policy "$${REDIS_MAXMEMORY_POLICY:-noeviction}" \' deploy/docker-compose.prod.yml || true)
if [ "$prod_policy_count" -ne 1 ] || [ "$prod_command_count" -ne 1 ]; then
  printf '%s\n' 'deploy/docker-compose.prod.yml must default Redis to noeviction in both the container environment and command' >&2
  exit 1
fi

printf 'docker compose security test passed\n'
