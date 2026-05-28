TOOLS := github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.6.0

.PHONY: generate build test verify run docker-up docker-down docker-logs db-up db db-down ps

generate:
	go run $(TOOLS) --config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml

build:
	go build ./...

test:
	go test ./...

verify: generate build test

run:
	go run ./cmd/server

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f app

db-up:
	docker compose up -d postgres

db:
	docker compose exec postgres psql -U postgres -d srtp

db-down:
	docker compose stop postgres
 
ps:
	docker compose ps