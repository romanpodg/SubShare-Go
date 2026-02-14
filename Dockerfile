FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o server ./cmd/server

# Build frontend if source exists, ensure directory exists either way
RUN if [ -d frontend ] && [ -f frontend/package.json ]; then \
      apk add --no-cache nodejs npm && \
      cd frontend && npm ci && npm run build; \
    fi && mkdir -p frontend/out

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/server .
COPY --from=builder /app/frontend/out ./frontend/out/
VOLUME /app/data
EXPOSE 8080
CMD ["./server"]
