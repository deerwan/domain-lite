.PHONY: dev-backend dev-frontend build docker tidy

dev-backend:
	go run ./cmd/server

dev-frontend:
	cd web && pnpm install && pnpm dev

build:
	cd web && pnpm install --frozen-lockfile && pnpm build
	cp -r web/dist internal/static/dist
	CGO_ENABLED=0 go build -ldflags="-s -w" -o domain-lite ./cmd/server

docker:
	docker compose up --build

tidy:
	go mod tidy
