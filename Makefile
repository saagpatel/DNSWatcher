SHELL := /bin/zsh

.PHONY: install frontend-install backend-test frontend-test generate test build docker-build runtime-smoke render-blueprint-validate

install: frontend-install
	cd backend && go mod tidy

frontend-install:
	cd frontend && npm install

backend-test:
	cd backend && go test ./...

frontend-test:
	cd frontend && npm test -- --run

generate:
	cd frontend && npm run generate:types

test: backend-test frontend-test

build:
	cd frontend && npm run build
	cd backend && go build -buildvcs=false ./...

docker-build:
	docker build -t dnswatcher:local .

runtime-smoke:
	./scripts/runtime-smoke.sh

render-blueprint-validate:
	render blueprints validate render.yaml
