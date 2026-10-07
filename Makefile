# story: e01s01
.PHONY: gen run build test lint preflight

gen:
	go tool templ generate

run: gen
	go run ./cmd/upareal

build: gen
	CGO_ENABLED=0 go build -o bin/upareal ./cmd/upareal

test:
	go test -race ./...

lint:
	golangci-lint run

# Generated code must be committed: fail if gen changed or added *_templ.go files.
preflight: gen
	@if [ -n "$$(git status --porcelain -- '*_templ.go')" ]; then \
		echo "preflight: generated templ code is stale or uncommitted:"; \
		git status --porcelain -- '*_templ.go'; exit 1; fi
	$(MAKE) lint test build
