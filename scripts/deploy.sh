#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${PSCPT_APP_DIR:-/home/fitrah/apps/pscpt}"
REF="${1:-${PSCPT_DEPLOY_REF:-main}}"
LOCAL_HEALTH="${PSCPT_LOCAL_HEALTH:-http://127.0.0.1:8091/api/health}"

compose() {
  if docker compose version >/dev/null 2>&1; then
    docker compose "$@"
  else
    docker-compose "$@"
  fi
}

cd "$APP_DIR"

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
