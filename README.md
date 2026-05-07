# watchlist-go

A project made to help learn new skills and improve existing ones. 

## Stack

- **[chi](https://github.com/go-chi/chi)** — HTTP router
- **[pgx](https://github.com/jackc/pgx)** — PostgreSQL driver
- **[godotenv](https://github.com/joho/godotenv)** — `.env` file loading

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

## Project Structure

```
cmd/server/       # Entry point
internal/
  db/             # Database connection
  handlers/       # Request/response logic
  server/         # Router setup
migrations/       # SQL migration files
```

## API

| Method | Path      | Description  |
|--------|-----------|--------------|
| GET    | `/health` | Health check |
