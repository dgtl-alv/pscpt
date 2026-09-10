#!/usr/bin/env bash
set -euo pipefail

DEPLOY_ENV="${PSCPT_DEPLOY_ENV:?PSCPT_DEPLOY_ENV is required}"
APP_DIR="${PSCPT_APP_DIR:?PSCPT_APP_DIR is required}"
REF="${PSCPT_DEPLOY_REF:?PSCPT_DEPLOY_REF is required}"
TAG="${PSCPT_DEPLOY_TAG:-}"

case "$DEPLOY_ENV:$APP_DIR" in
  staging:/opt/alva/apps/staging/pscpt) DEFAULT_APP_PORT=8091; DEFAULT_DB_PORT=3308 ;;
  prod:/opt/alva/apps/prod/pscpt) DEFAULT_APP_PORT=8001; DEFAULT_DB_PORT=3309 ;;
  *) printf 'invalid deploy environment or path\n' >&2; exit 2 ;;
esac
[[ "$REF" =~ ^[0-9a-f]{40}$ ]] || { printf 'deploy ref must be a full commit SHA\n' >&2; exit 2; }
if [[ "$DEPLOY_ENV" == prod ]]; then
  [[ "$TAG" =~ ^prod-[0-9]{4}-[0-9]{2}-[0-9]{2}-[1-9][0-9]*$ ]] || { printf 'invalid production tag\n' >&2; exit 2; }
fi

export ALVA_ENV="$DEPLOY_ENV" COMPOSE_PROJECT_NAME="pscpt_${DEPLOY_ENV}"
export PSCPT_HOST_BIND="${PSCPT_HOST_BIND:-127.0.0.1}" PSCPT_HOST_PORT="${PSCPT_HOST_PORT:-$DEFAULT_APP_PORT}"
export PSCPT_DB_HOST_BIND="${PSCPT_DB_HOST_BIND:-127.0.0.1}" PSCPT_DB_HOST_PORT="${PSCPT_DB_HOST_PORT:-$DEFAULT_DB_PORT}"
HEALTH="http://${PSCPT_HOST_BIND}:${PSCPT_HOST_PORT}/api/health"
STATE_DIR="$APP_DIR/.deploy-state"

compose() { docker compose "$@"; }
health() { curl --fail --silent --show-error --max-time 5 "$HEALTH" >/dev/null; }
checkout_sha() {
  local sha="$1"
  git fetch --no-tags origin "$sha"
  git cat-file -e "$sha^{commit}"
  git checkout --force --detach "$sha"
}

cd "$APP_DIR"
command -v git >/dev/null && command -v docker >/dev/null && command -v curl >/dev/null
compose version >/dev/null
mkdir -p "$STATE_DIR"
PREVIOUS_SHA="$(cat "$STATE_DIR/active-sha" 2>/dev/null || true)"
PREVIOUS_TAG="$(cat "$STATE_DIR/active-production-tag" 2>/dev/null || true)"
checkout_sha "$REF"
if [[ "$DEPLOY_ENV" == prod ]]; then
  git fetch --no-tags origin "refs/tags/$TAG:refs/tags/$TAG"
  [[ "$(git rev-parse HEAD)" == "$REF" ]]
  [[ "$(git rev-list -n 1 "refs/tags/$TAG")" == "$REF" ]] || { printf 'tag does not resolve to deploy SHA\n' >&2; exit 2; }
fi
compose config --quiet
compose up -d --build

for _ in $(seq 1 30); do
  if health; then
    printf '%s\n' "$REF" > "$STATE_DIR/active-sha"
    [[ "$DEPLOY_ENV" == prod ]] && printf '%s\n' "$TAG" > "$STATE_DIR/active-production-tag"
    printf 'pscpt deploy: healthy at %s\n' "$REF"
    exit 0
  fi
  sleep 2
done

printf 'pscpt deploy: health failed; rolling back\n' >&2
if [[ "$PREVIOUS_SHA" =~ ^[0-9a-f]{40}$ ]]; then
  checkout_sha "$PREVIOUS_SHA"
  compose config --quiet
  compose up -d --build
  for _ in $(seq 1 30); do health && break; sleep 2; done
  health || { printf 'rollback health check failed\n' >&2; exit 1; }
  printf '%s\n' "$PREVIOUS_SHA" > "$STATE_DIR/active-sha"
  [[ -n "$PREVIOUS_TAG" ]] && printf '%s\n' "$PREVIOUS_TAG" > "$STATE_DIR/active-production-tag"
  printf 'pscpt rollback: restored %s\n' "$PREVIOUS_SHA" >&2
fi
exit 1
