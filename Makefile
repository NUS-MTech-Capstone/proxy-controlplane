SERVICE := proxy-controlplane
IMAGE ?= dendenmushi/$(SERVICE):local
PORT ?= 8080
SESSION_MANAGER_URL ?= http://localhost:8081
STS_URL ?= http://localhost:8082
GOCACHE ?= /private/tmp/ddm-go-build-cache
GOMODCACHE ?= /private/tmp/ddm-go-mod-cache

.PHONY: run test build docker-build clean

run:
	PORT=$(PORT) SESSION_MANAGER_URL=$(SESSION_MANAGER_URL) STS_URL=$(STS_URL) go run ./cmd/server

test:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go test ./...

build:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go build -o bin/$(SERVICE) ./cmd/server

docker-build:
	docker build -t $(IMAGE) .

clean:
	rm -rf bin
