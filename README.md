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

Default Docker values are development values. Change before production:

- `PSCPT_SESSION_SECRET`
- `MYSQL_PASSWORD`
- `MYSQL_ROOT_PASSWORD`
- `PSCPT_APP_URL`
- `EMICA_BASE_URL`
- `EMICA_ODOO_DB`
- `EMICA_ODOO_USERNAME`
- `EMICA_ODOO_API_KEY` or `EMICA_ODOO_PASSWORD`
- `EMICA_TIMEOUT`

The Emica sync calls Odoo's external API flow with `authenticate` and `execute_kw`/`search_read`, matching the official Odoo integration pattern.
