.PHONY: proto-gen build up down test lint clean

# Proto code generation (Go + TypeScript)
proto-gen:
	@echo "Generating Go proto code..."
	protoc \
		--proto_path=proto \
		--go_out=gen/go \
		--go_opt=paths=source_relative \
		proto/*.proto
	@echo "Generating TypeScript proto code..."
	protoc \
		--proto_path=proto \
		--es_out=gen/ts \
		--es_opt=target=ts \
		proto/*.proto
	@echo "Proto generation done."

# Build all Go services
build:
	@for svc in control-server auth-service safety-service telemetry-service webrtc-sfu; do \
		echo "Building $$svc..."; \
		go build -o bin/$$svc ./cmd/$$svc; \
	done

# Start full stack
up:
	docker compose -f infrastructure/compose/docker-compose.yml --env-file .env up --build

# Stop full stack
down:
	docker compose -f infrastructure/compose/docker-compose.yml --env-file .env down

# Run all tests
test:
	go test ./...

# Run safety test suite only
test-safety:
	go test ./internal/controlserver/safety/... -v -run Safety

# Run latency tests
test-latency:
	k6 run tests/integration/latency.js

# Lint
lint:
	golangci-lint run ./...

# Clean generated files
clean:
	rm -rf gen/go/* gen/ts/* bin/
