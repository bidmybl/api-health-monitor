FROM golang:1.27.3-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /app/api \
    ./cmd/api

FROM alpine:3.22

RUN apk --no-cache add ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app
WORKDIR /app
COPY --from=builder --chown=app:app /app/api ./api
USER app:app

EXPOSE 8090
ENTRYPOINT ["/app/api"]