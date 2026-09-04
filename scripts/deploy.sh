#!/usr/bin/env bash
set -euo pipefail

REF="${1:-${PSCPT_DEPLOY_REF:-main}}"

DEPLOY_ENV="${PSCPT_DEPLOY_ENV:-}"
if [ -z "$DEPLOY_ENV" ]; then
  case "$REF" in
    prod-* | refs/tags/prod-*) DEPLOY_ENV="prod" ;;
    *) DEPLOY_ENV="staging" ;;
  esac
fi

APP_DIR="${PSCPT_APP_DIR:-/opt/alva/apps/${DEPLOY_ENV}/pscpt}"
case "$DEPLOY_ENV" in
  prod)
    DEFAULT_APP_PORT="8001"
    DEFAULT_DB_PORT="3309"
    ;;
  *)
    DEFAULT_APP_PORT="8091"
    DEFAULT_DB_PORT="3308"
    ;;
esac

export ALVA_ENV="$DEPLOY_ENV"
export COMPOSE_PROJECT_NAME="${PSCPT_COMPOSE_PROJECT:-pscpt_${DEPLOY_ENV}}"
export PSCPT_HOST_BIND="${PSCPT_HOST_BIND:-127.0.0.1}"
export PSCPT_HOST_PORT="${PSCPT_HOST_PORT:-$DEFAULT_APP_PORT}"
export PSCPT_DB_HOST_BIND="${PSCPT_DB_HOST_BIND:-127.0.0.1}"
export PSCPT_DB_HOST_PORT="${PSCPT_DB_HOST_PORT:-$DEFAULT_DB_PORT}"

LOCAL_HEALTH="${PSCPT_LOCAL_HEALTH:-http://${PSCPT_HOST_BIND}:${PSCPT_HOST_PORT}/api/health}"

compose() {
  if docker compose version >/dev/null 2>&1; then
    docker compose "$@"
  else
    docker-compose "$@"
  fi
}

cd "$APP_DIR"

printf 'pscpt deploy: env %s\n' "$DEPLOY_ENV"
printf 'pscpt deploy: fetch refs\n'
git fetch --prune --tags origin

case "$REF" in
  prod-* | refs/tags/prod-*)
    REF="${REF#refs/tags/}"
    printf 'pscpt deploy: checkout tag %s\n' "$REF"
    git checkout --force --detach "$REF"
    ;;
  *)
    printf 'pscpt deploy: reset branch origin/%s\n' "$REF"
    git checkout --force "$REF"
    git reset --hard "origin/$REF"
    ;;
esac

printf 'pscpt deploy: compose config\n'
compose config --quiet

printf 'pscpt deploy: build and restart\n'
compose up -d --build

printf 'pscpt deploy: health\n'
for i in $(seq 1 30); do
  if curl -fsS "$LOCAL_HEALTH"; then
    printf '\npscpt deploy: ok\n'
    exit 0
  fi
  sleep 2
done

printf 'pscpt deploy: health failed\n' >&2
exit 1
