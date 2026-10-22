#!/bin/bash
set -e

cd "$(dirname "$0")"

if ! docker compose ps postgres 2>/dev/null | grep -q running; then
  echo "Starting PostgreSQL..."
  docker compose up -d postgres
  echo "Waiting for PostgreSQL..."
  for i in $(seq 1 30); do
    if docker compose exec -T postgres pg_isready -U solution >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done
fi

export DATABASE_URL="${DATABASE_URL:-postgres://solution:solution@localhost:5432/solution?sslmode=disable}"
export PORT="${PORT:-3001}"
export LOG_LEVEL="${LOG_LEVEL:-info}"

echo "Building..."
go build -o bin/api ./cmd/api

echo "Starting server on port $PORT..."
echo "For Docker full stack: docker compose up -d --build"
exec ./bin/api
