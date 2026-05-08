# watchlist-go

A project made to help learn new skills and improve existing ones.

## Stack

- **[chi](https://github.com/go-chi/chi)** — HTTP router
- **[pgx](https://github.com/jackc/pgx)** — PostgreSQL driver
- **[godotenv](https://github.com/joho/godotenv)** — `.env` file loading
- **[golang-jwt/jwt](https://github.com/golang-jwt/jwt)** — JWT creation and validation
- **[discord-oauth2](https://github.com/ravener/discord-oauth2)** — Discord OAuth2 constants
- **[golang.org/x/oauth2](https://pkg.go.dev/golang.org/x/oauth2)** — OAuth2 client

## Getting Started

### Prerequisites

- Go 1.25+
- Docker

### Run the database

```bash
docker compose up -d
```

This starts a PostgreSQL instance on port `5432` and automatically runs migrations from `./migrations`.

### Configure environment

Copy and fill out the `.env.example` file

### Run the server

```bash
go run ./cmd/server
```

The server starts on port `8080` by default.

## Install git hooks

A pre-commit hook that removes hardcoded bearer JWT tokens from `bruno/watchlist.yml` before a commit is created is included for easily and safely exporting the bruno collection.

```bash
./scripts/install-git-hooks.sh
```

## Project Structure

```
cmd/server/       # Entry point
internal/
  auth/           # Discord OAuth2 and JWT logic
  db/             # Database connection
  handlers/       # Request/response logic
  middleware/     # Auth middleware
  models/         # Shared data models
  server/         # Router setup
migrations/       # SQL migration files
```

## API

| Method | Path             | Auth     | Description                          |
| ------ | ---------------- | -------- | ------------------------------------ |
| GET    | `/health`        |          | Health check                         |
| GET    | `/auth/login`    |          | Redirect to Discord OAuth2 login     |
| GET    | `/auth/callback` |          | Discord OAuth2 callback, returns JWT |
| GET    | `/auth/me`       | Required | Returns the authenticated user       |
