# watchlist-go

The backend API for a movie & TV watchlist app. Sign in with Discord, track what
you're watching, browse and search releases, and get notified when something new
drops.

## Features

- Discord OAuth2 login with JWT sessions
- Personal watchlist with watch status, playback progress, and episode counts
- Discover and search
- Notifications for new releases
- Upcoming-releases calendar
- Profile stats and a yearly "wrapped" summary

## Stack

- **[chi](https://github.com/go-chi/chi)**
- **[pgx](https://github.com/jackc/pgx)**
- **[godotenv](https://github.com/joho/godotenv)**
- **[golang-jwt/jwt](https://github.com/golang-jwt/jwt)**
- **[discord-oauth2](https://github.com/ravener/discord-oauth2)**
- **[golang.org/x/oauth2](https://pkg.go.dev/golang.org/x/oauth2)**
- **[swaggo/http-swagger](https://github.com/swaggo/http-swagger)**

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
| `ENV` | Set to `production` to disable Swagger and use secure cookies |
| `JWT_SECRET` | Any random string, at least 32 characters |
| `DISCORD_CLIENT_ID` | From the Discord developer portal |
| `DISCORD_CLIENT_SECRET` | From the Discord developer portal |
| `DISCORD_REDIRECT_URL` | Must match what's set in Discord (`/auth/callback`) |
| `FRONTEND_URL` | Frontend origin, used for CORS and the post-login redirect (default `http://localhost:3000`) |

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

Swagger UI is available at `localhost:8080/swagger/index.html` in non-production environments.

## Install git hooks

A pre-commit hook is included that strips hardcoded bearer tokens from
`bruno/watchlist.yml` before committing, so the Bruno collection can be safely exported.

```bash
./scripts/install-git-hooks.sh
```
