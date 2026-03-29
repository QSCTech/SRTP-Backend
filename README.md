## Stack

- Go 1.25.0
- Gin
- GORM
- PostgreSQL
- Zap
- OpenAPI 3.0.3
- oapi-codegen
- Docker / Docker Compose

## Repository Layout

```text
.
├── api/openapi/                # OpenAPI source of truth
│   ├── openapi.yaml
│   └── oapi-codegen.yaml
├── cmd/server/                 # application entrypoint
│   └── main.go
├── internal/
│   ├── api/                    # transport layer
│   │   ├── gen/                # generated server/types, do not edit
│   │   ├── handler.go
│   │   └── router.go
│   ├── config/                 # env loading and validation
│   ├── database/               # DB bootstrap / pool config
│   ├── logger/                 # zap setup
│   ├── middleware/             # gin middleware
│   ├── repository/             # persistence layer
│   └── service/                # business layer
├── models/                     # gorm models
├── pkg/response/               # shared response helpers
├── pkg/utils/                  # small shared helpers
├── .env.example
├── Dockerfile
├── docker-compose.yml
└── Makefile
```

## Architecture

Request flow:

```text
HTTP -> handler -> service -> repository -> PostgreSQL
```

Responsibilities:

- `internal/api`: request/response adaptation only
- `internal/service`: business rules and orchestration
- `internal/repository`: database access via GORM
- `models`: persistence models
- `api/openapi`: API contract and generated interface source

## Quick Start

### 1. Clone and install dependencies

```bash
git clone https://github.com/QSCTech/SRTP-Backend.git
cd SRTP-backend
go mod tidy
```

### 2. Create local env

```bash
cp .env.example .env
```

### 3. Start PostgreSQL only

```bash
docker compose up -d postgres
```

### 4. Run the service locally

```bash
go run ./cmd/server
```

Default health endpoints:

- `GET http://localhost:8080/healthz`
- `GET http://localhost:8080/readyz`

## Full Docker Run

```bash
docker compose up --build
```

Notes:
- When the app runs on the host machine, use `DB_HOST=localhost`
- When the app runs inside Compose, use `DB_HOST=postgres`

## Environment Variables

Loaded from `.env` via `internal/config/config.go`.

Common values:

```env
APP_ENV=development
HTTP_PORT=8080
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=srtp
DB_SSLMODE=disable
DB_TIMEZONE=Asia/Shanghai
DB_MAX_IDLE_CONNS=10
DB_MAX_OPEN_CONNS=50
DB_CONN_MAX_LIFETIME_MIN=30
DB_CONN_MAX_IDLE_TIME_MIN=10
LOG_LEVEL=info
```

## Database

Current bootstrap is in `internal/database/postgres.go`.

It:
- opens a GORM PostgreSQL connection
- configures the connection pool
- pings the database on startup

Current app startup in `cmd/server/main.go` also runs:

```go
gormDB.AutoMigrate(&models.User{})
```

So the sample `users` table is created automatically.

Default local DB credentials from Compose:

- host: `localhost`
- port: `5432`
- user: `postgres`
- password: `postgres`
- database: `srtp`

## OpenAPI Workflow

OpenAPI is the source of truth for HTTP contracts.

Files:
- spec: `api/openapi/openapi.yaml`
- generator config: `api/openapi/oapi-codegen.yaml`
- generated output: `internal/api/gen/api.gen.go`

Do not edit generated code directly.

### Regenerate code

```bash
make generate
```

### Validate the scaffold

```bash
make verify
```

Equivalent Make targets:

- `make generate` — regenerate from OpenAPI
- `make build` — compile all packages
- `make test` — run tests
- `make verify` — generate + build + test

## Current Endpoints

- `GET /healthz`
- `GET /readyz`
- `POST /api/v1/users`
- `GET /api/v1/users/{id}`

## Adding a New Endpoint

Recommended sequence:

1. Update `api/openapi/openapi.yaml`
2. Run `make generate`
3. Implement the generated interface in `internal/api/handler.go` or split handlers if needed
4. Add or update service logic in `internal/service`
5. Add or update repository logic in `internal/repository`
6. Add or update GORM models in `models`
7. Run `make verify`

## Conventions

- Treat OpenAPI as the contract source; avoid hand-written route drift
- Do not put business logic in handlers
- Do not access the DB directly from handlers
- Do not edit `internal/api/gen/api.gen.go`
- Keep shared helpers minimal; avoid turning `pkg/utils` into a dump folder

## Example Smoke Tests

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice"}'
curl http://localhost:8080/api/v1/users/1
```

## Current Scope

The repo is still in scaffold stage. The current vertical slice exists mainly to validate:

- project structure
- PostgreSQL integration
- OpenAPI code generation
- handler/service/repository layering
- Docker-based local development

The next real feature areas from `introduction.md` are expected to be:
- lobby / room listing
- profile
- room creation
- room management
