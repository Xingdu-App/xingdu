SHELL := /bin/sh

.PHONY: setup dev-api dev-web worker agent migrate build check up down
setup:
	go mod download
	npm --prefix apps/web ci

dev-api:
	go run ./cmd/server

dev-web:
	npm --prefix apps/web run dev

worker:
	go run ./cmd/worker

agent:
	go run ./cmd/agent

migrate:
	go run ./cmd/migrate

build:
	go build -trimpath -o ./bin/ ./cmd/...
	npm --prefix apps/web run build

check:
	go vet ./...
	go test -race ./...
	npm --prefix apps/web run lint
	npm --prefix apps/web run build

up:
	docker compose up --build -d --wait

down:
	docker compose down

.PHONY: agents
agents:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false -o bin/agents/xingdu-agent-linux-amd64 ./cmd/agent
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -buildvcs=false -o bin/agents/xingdu-agent-linux-arm64 ./cmd/agent

.PHONY: agent-lab-up agent-lab-test agent-lab-cleanup
agent-lab-up:
	python3 scripts/agent-lab.py up

agent-lab-test:
	python3 scripts/agent-lab.py test

agent-lab-cleanup:
	python3 scripts/agent-lab.py cleanup

.PHONY: runtimes protocol-lab-test
runtimes:
	go run ./tools/fetch-runtime

protocol-lab-test:
	python3 scripts/protocol-lab.py
