FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.Version=${VERSION:-dev} -X main.GitCommit=${GIT_COMMIT:-unknown} -X main.BuildTime=$(date -u '+%Y-%m-%d_%H:%M:%S')" \
    -o drip \
    ./cmd/drip

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -S drip && adduser -S -G drip drip

WORKDIR /app

RUN mkdir -p /app/data/certs && \
    chown -R drip:drip /app

COPY --from=builder /app/drip /app/drip

USER drip

EXPOSE 80 443 8080 20000-40000

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/drip"]
CMD ["server", "--port", "8080"]
