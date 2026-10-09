# story: e01s01 e03s01 BUG-002
.PHONY: gen run build test lint preflight

gen:
	go tool templ generate
	go tool sqlc generate

run: gen
	go run ./cmd/upareal

build: gen
	CGO_ENABLED=0 go build -o bin/upareal ./cmd/upareal

# A far time zone (UTC+14): its local date differs from São Paulo's 17h a day,
# so tests that confuse "local today" with "São Paulo today" fail (BUG-002).
test:
	TZ=Pacific/Kiritimati go test -race ./...

lint:
	golangci-lint run

# Generated code must be committed: fail if gen changed or added templ or sqlc output.
preflight: gen
	@if [ -n "$$(git status --porcelain -- '*_templ.go' '*/internal/store/*')" ]; then \
		echo "preflight: generated code is stale or uncommitted:"; \
		git status --porcelain -- '*_templ.go' '*/internal/store/*'; exit 1; fi
	$(MAKE) lint test build
