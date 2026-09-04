# PSCPT

PS Correlation Platform: internal web app for PS-month correlation analysis.

## Stack

- Backend: plain Go `net/http`, MySQL driver only
- Frontend: React + Vite, built static for offline/local use
- Database: MySQL 8.4
- Runtime: Docker Compose

## Run

```bash
docker-compose up -d --build
```

Open:

```text
http://localhost:8091
```

Health check:

```bash
curl http://localhost:8091/api/health
```

## Current Features

- Landing page
- Register
- Login
- Logout
- Forgot password with local dev reset link
- Reset password API
- Change password
- Authenticated dashboard shell
- Initial PS-month analysis preview API
- Source intake with two lanes for sales performance: `.xlsx` upload and Emica/Odoo API sync

## Environment

Runtime secrets are loaded from `../secrets/pscpt.env` and `../secrets/emica.env`.
Production needs at least:

- `PSCPT_SESSION_SECRET`
- `PSCPT_DSN`
- `PSCPT_APP_URL`
- `MYSQL_PASSWORD`
- `MYSQL_ROOT_PASSWORD`
- `EMICA_BASE_URL`
- `EMICA_ODOO_DB`
- `EMICA_ODOO_USERNAME`
- `EMICA_ODOO_API_KEY` or `EMICA_ODOO_PASSWORD`
- `EMICA_TIMEOUT`

The Emica sync calls Odoo's external API flow with `authenticate` and `execute_kw`/`search_read`, matching the official Odoo integration pattern.

## CI/CD

Branch and release rules:

- Pull requests and pushes to `main` run CI.
- Merge to `main` deploys staging after CI succeeds.
- Production deploys only from tags matching `prod-*`.
- Production should use GitHub Environment approval before the SSH deploy step.

Production tag format:

```text
prod-YYYY-MM-DD-N
```

Example:

```bash
git checkout main
git pull origin main
git tag prod-2026-09-04-1
git push origin prod-2026-09-04-1
```

Required GitHub secrets:

```text
PSCPT_STAGING_DEPLOY_HOST
PSCPT_STAGING_DEPLOY_PORT
PSCPT_STAGING_DEPLOY_USER
PSCPT_STAGING_DEPLOY_SSH_KEY
PSCPT_PROD_DEPLOY_HOST
PSCPT_PROD_DEPLOY_PORT
PSCPT_PROD_DEPLOY_USER
PSCPT_PROD_DEPLOY_SSH_KEY
```

The server-side deploy command expects the app repository at `/home/fitrah/apps/pscpt`.
Override it on the server with `PSCPT_APP_DIR` if needed.
