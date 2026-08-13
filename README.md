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

## Environment

Default Docker values are development values. Change before production:

- `PSCPT_SESSION_SECRET`
- `MYSQL_PASSWORD`
- `MYSQL_ROOT_PASSWORD`
- `PSCPT_APP_URL`
