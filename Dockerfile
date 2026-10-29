FROM golang:1.27.2-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CGO_ENABLED=0 делает бинарник статическим (идеально для Alpine)
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/exe ./cmd/api/

FROM alpine:3.22
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/exe .
COPY migrations ./migrations

EXPOSE 8090
ENTRYPOINT [ "/app/exe" ]