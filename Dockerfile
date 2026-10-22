FROM golang:1.23-alpine AS builder

ENV GOTOOLCHAIN=auto

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/api

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /api /app/api
COPY web /app/web
COPY migrations /app/migrations

ENV PORT=3001
ENV WEB_DIR=/app/web
ENV DATABASE_URL=postgres://solution:solution@postgres:5432/solution?sslmode=disable
ENV LOG_LEVEL=info

EXPOSE 3001

CMD ["/app/api"]
