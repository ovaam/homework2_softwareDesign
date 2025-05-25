#!/bin/bash

exec 2>/dev/null

cd "$(dirname "$0")"

is_port_free() {
    ! lsof -i :$1 >/dev/null 2>&1
}

for port in 8080 8081 8082; do
    if ! is_port_free $port; then
        kill -9 $(lsof -ti :$port) 2>/dev/null || true
        sleep 1
    fi
done

mkdir -p file_store/storage

(cd api_gateway && go run main.go >/dev/null 2>&1) &
(cd file_store && go run main.go >/dev/null 2>&1) &
(cd file_analysis && go run main.go >/dev/null 2>&1) &

sleep 1

(cd client && go run main.go)

pkill -f "go run main.go" >/dev/null 2>&1