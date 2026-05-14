# watchlist-go

A project made to help learn new skills and improve existing ones.

## Stack

- **[chi](https://github.com/go-chi/chi)** — HTTP router
- **[pgx](https://github.com/jackc/pgx)** — PostgreSQL driver (pgxpool)
- **[godotenv](https://github.com/joho/godotenv)** — `.env` file loading
- **[golang-jwt/jwt](https://github.com/golang-jwt/jwt)** — JWT creation and validation
- **[discord-oauth2](https://github.com/ravener/discord-oauth2)** — Discord OAuth2 constants
- **[golang.org/x/oauth2](https://pkg.go.dev/golang.org/x/oauth2)** — OAuth2 client
- **[swaggo/http-swagger](https://github.com/swaggo/http-swagger)** — Swagger UI (dev only)

## Getting Started

### Prerequisites

- Go 1.25+
- Docker

### Configure environment

Copy `.env.example` to `.env` and fill in the values:

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string |
| `PORT` | Port to run the server on (default `8080`) |
| `DISCORD_CLIENT_ID` | From the Discord developer portal |
| `DISCORD_CLIENT_SECRET` | From the Discord developer portal |
| `DISCORD_REDIRECT_URL` | Must match what's set in Discord (`/auth/callback`) |
| `JWT_SECRET` | Any random string, at least 32 characters |
| `META_BASE_URL` | Base URL for the Stremio-compatible meta API |

### Run the database

```bash
make docker
```

### Run locally

```bash
make dev
```

See the `Makefile` for all available commands.

### API Docs

Swagger UI is available at `/swagger/` in non-production environments.

## Install git hooks

A pre-commit hook is included that strips hardcoded bearer tokens from `bruno/watchlist.yml` before committing, so the Bruno collection can be safely exported.

```bash
./scripts/install-git-hooks.sh
```
