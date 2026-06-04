.PHONY: proto-gen proto-gen-ts build up down test test-safety test-integration test-latency test-k6 lint clean

# Proto code generation (Go + TypeScript)
proto-gen:
	@echo "Generating Go proto code..."
	protoc \
		--proto_path=proto \
		--go_out=. \
		--go_opt=module=avoc \
		proto/*.proto
	@echo "Generating TypeScript proto code..."
	protoc \
		--proto_path=proto \
		--es_out=gen/ts \
		--es_opt=target=ts \
		proto/*.proto
	@echo "Proto generation done."

# Generate TypeScript proto code (local dev — uses Docker to avoid local protoc requirement)
proto-gen-ts:
	docker run --rm \
		-v $(PWD):/app -w /app/frontend \
		--entrypoint sh node:22-alpine -c \
		"apk add --no-cache protobuf && npm ci && \
		mkdir -p src/gen && \
		PATH=\$$PATH:./node_modules/.bin protoc \
			--proto_path=../proto --es_out=src/gen --es_opt=target=ts ../proto/*.proto && \
		echo 'TypeScript proto generation done.'"

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

# Run all Go tests (unit + integration if stack is up)
test:
	go test ./...

# Run safety test suite only (CI safety gate — must stay 19/19 green)
test-safety:
	go test ./tests/unit/... -v -run Safety

# Start test stack, run Go integration tests, stop stack
test-integration:
	@echo "Starting test stack..."
	docker compose -f tests/docker-compose.test.yml up --build -d
	@echo "Waiting for services..."
	@sleep 5
	@echo "Running integration tests..."
	go test ./tests/integration/... -v -timeout 120s; \
	EXIT=$$?; \
	docker compose -f tests/docker-compose.test.yml down; \
	exit $$EXIT

# Go benchmark: ACK-Roundtrip latency (requires test stack running)
# Build-Fail when p99 > 100ms (ADR-006/010)
test-latency:
	@echo "Starting test stack for latency measurement..."
	docker compose -f tests/docker-compose.test.yml up --build -d
	@sleep 5
	@echo "Running Go latency benchmark..."
	go test ./tests/performance/... -bench=BenchmarkControlACKRoundtrip \
		-benchtime=10s -run=^$$ -v | tee /tmp/bench_out.txt; \
	EXIT=$$?; \
	docker compose -f tests/docker-compose.test.yml down; \
	exit $$EXIT

# k6 load test (requires k6 or Docker)
test-k6:
	@echo "Starting test stack for k6..."
	docker compose -f tests/docker-compose.test.yml up --build -d
	@sleep 5
	docker run --rm --network host grafana/k6 run - < tests/performance/latency.js; \
	EXIT=$$?; \
	docker compose -f tests/docker-compose.test.yml down; \
	exit $$EXIT

# Lint
lint:
	golangci-lint run ./...

# Clean generated files
clean:
	rm -rf gen/go/* gen/ts/* bin/
