#!/usr/bin/env bash
set -Eeuo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.prod.yml}"
SERVICE_NAME="${SERVICE_NAME:-modurelay}"
CONTAINER_NAME="${CONTAINER_NAME:-modurelay}"
HEALTH_TIMEOUT_SECONDS="${HEALTH_TIMEOUT_SECONDS:-180}"
STATE_FILE="${STATE_FILE:-.deployed-image}"
TARGET_IMAGE="${1:-}"

log() {
  printf '[modurelay-deploy] %s\n' "$*"
}

fail() {
  printf '[modurelay-deploy] ERROR: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

wait_for_health() {
  local deadline=$((SECONDS + HEALTH_TIMEOUT_SECONDS))
  local state=""

  while (( SECONDS < deadline )); do
    state="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$CONTAINER_NAME" 2>/dev/null || true)"

    case "$state" in
      healthy|running)
        return 0
        ;;
      unhealthy|exited|dead)
        return 1
        ;;
    esac

    sleep 3
  done

  return 1
}

require_command docker
docker compose version >/dev/null 2>&1 || fail "Docker Compose v2 is required"

[ -n "$TARGET_IMAGE" ] || fail "usage: $0 ghcr.io/lien0219/modurelay:sha-<commit>"
[[ "$TARGET_IMAGE" == ghcr.io/lien0219/modurelay:* ]] || fail "unexpected image repository: $TARGET_IMAGE"
[ -f "$COMPOSE_FILE" ] || fail "missing $COMPOSE_FILE"
[ -f .env ] || fail "missing server-side .env"

mkdir -p data postgres_data redis_data
chmod 700 .env 2>/dev/null || true

log "validating production compose configuration"
MODURELAY_IMAGE="$TARGET_IMAGE" docker compose --env-file .env -f "$COMPOSE_FILE" config --quiet

previous_image=""
if docker inspect "$CONTAINER_NAME" >/dev/null 2>&1; then
  previous_image="$(docker inspect --format '{{.Config.Image}}' "$CONTAINER_NAME")"
elif [ -f "$STATE_FILE" ]; then
  previous_image="$(cat "$STATE_FILE")"
fi

log "pulling immutable image $TARGET_IMAGE"
docker pull "$TARGET_IMAGE"

log "recreating only the $SERVICE_NAME service"
MODURELAY_IMAGE="$TARGET_IMAGE" docker compose \
  --env-file .env \
  -f "$COMPOSE_FILE" \
  up -d --no-deps --force-recreate "$SERVICE_NAME"

if wait_for_health; then
  printf '%s\n' "$TARGET_IMAGE" > "$STATE_FILE"
  log "deployment healthy: $TARGET_IMAGE"
  docker image prune -f >/dev/null 2>&1 || true
  exit 0
fi

log "new container did not become healthy"
docker logs --tail 200 "$CONTAINER_NAME" >&2 || true

if [ -n "$previous_image" ] && [ "$previous_image" != "$TARGET_IMAGE" ]; then
  log "rolling back to $previous_image"
  docker pull "$previous_image" >/dev/null 2>&1 || true
  MODURELAY_IMAGE="$previous_image" docker compose \
    --env-file .env \
    -f "$COMPOSE_FILE" \
    up -d --no-deps --force-recreate "$SERVICE_NAME"

  if wait_for_health; then
    printf '%s\n' "$previous_image" > "$STATE_FILE"
    fail "deployment failed; rollback completed successfully"
  fi

  docker logs --tail 200 "$CONTAINER_NAME" >&2 || true
  fail "deployment failed and rollback did not become healthy"
fi

fail "deployment failed and no previous image was available for rollback"
