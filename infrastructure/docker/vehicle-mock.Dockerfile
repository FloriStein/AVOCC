FROM golang:1.23-alpine AS builder
WORKDIR /app

ENV GOFLAGS=-mod=mod

RUN apk add --no-cache protobuf protobuf-dev && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.34.2

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN protoc --proto_path=proto \
    --go_out=. \
    --go_opt=module=avoc \
    proto/*.proto

RUN go build -o /vehicle-mock ./cmd/vehicle-mock

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /vehicle-mock /vehicle-mock
ENTRYPOINT ["/vehicle-mock"]
