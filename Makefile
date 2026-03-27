TOOLS := github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.6.0

generate:
	go run $(TOOLS) --config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml

build:
	go build ./...

test:
	go test ./...

verify: generate build test
