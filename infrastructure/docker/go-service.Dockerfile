ARG SERVICE_NAME

FROM golang:1.23-alpine AS builder
# ARG must be re-declared after FROM to be available in this stage
ARG SERVICE_NAME
WORKDIR /app

# Allow go to update go.sum (no go.sum pre-committed in Sprint 1)
ENV GOFLAGS=-mod=mod

# Install protoc + pin protoc-gen-go to match go.mod protobuf version
RUN apk add --no-cache protobuf protobuf-dev && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.34.2

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

# Generate proto code
RUN mkdir -p gen/go && \
    protoc --proto_path=proto --go_out=gen/go --go_opt=paths=source_relative proto/*.proto

# Build the specific service
RUN go build -o /service ./cmd/${SERVICE_NAME}

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /service /service
EXPOSE 8080
ENTRYPOINT ["/service"]
