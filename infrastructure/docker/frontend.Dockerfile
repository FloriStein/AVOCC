FROM node:22-alpine AS builder
WORKDIR /app

# Install protoc for TypeScript proto code generation (ADR-012b/013)
RUN apk add --no-cache protobuf

COPY frontend/package*.json ./
RUN npm ci

# protoc-gen-es plugin is in node_modules/.bin after npm ci
COPY frontend/ .
COPY proto/ ./proto/

# Generate TypeScript Protobuf classes (gitignored, build-time only)
RUN mkdir -p src/gen && \
    PATH=$PATH:./node_modules/.bin protoc \
        --proto_path=./proto \
        --es_out=src/gen \
        --es_opt=target=ts \
        ./proto/*.proto

RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY infrastructure/docker/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
