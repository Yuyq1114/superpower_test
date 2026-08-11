.PHONY: proto test test-integration test-e2e up down k8s-up k8s-down frontend-test frontend-build frontend-e2e

proto:
	powershell -ExecutionPolicy Bypass -File scripts/generate-proto.ps1

test:
	go test ./...

test-integration:
	@if [ -z "$(TEST_DATABASE_DSN)" ] || [ -z "$(TEST_DATABASE_ADMIN_DSN)" ]; then echo "TEST_DATABASE_DSN and TEST_DATABASE_ADMIN_DSN are required for isolated PostgreSQL integration tests"; exit 2; fi
	go test -tags=integration ./... -count=1

test-e2e:
	@if [ -z "$(BASE_URL)" ]; then echo "BASE_URL is required, e.g. BASE_URL=http://127.0.0.1:8088 make test-e2e (the same-origin frontend entrypoint since Task 8)"; exit 2; fi
	BASE_URL=$(BASE_URL) go test -tags=e2e ./tests/e2e -count=1

up:
	docker compose -f deploy/docker-compose.yml up -d

down:
	docker compose -f deploy/docker-compose.yml down

k8s-up:
	@if [ ! -f deploy/k8s/dev/secret.env ]; then echo "missing deploy/k8s/dev/secret.env; copy secret.env.example and fill local values without committing it"; exit 2; fi
	kubectl apply -k deploy/k8s/dev

k8s-down:
	kubectl delete -k deploy/k8s/dev

frontend-test:
	cd frontend && npm run test:run && npm run typecheck && npm run lint

frontend-build:
	cd frontend && npm run build

frontend-e2e:
	@if [ -z "$(PLAYWRIGHT_BASE_URL)" ]; then echo "PLAYWRIGHT_BASE_URL is required, e.g. PLAYWRIGHT_BASE_URL=http://127.0.0.1:8088 make frontend-e2e (or http://127.0.0.1:30080 for the Kubernetes deployment)"; exit 2; fi
	cd frontend && PLAYWRIGHT_BASE_URL=$(PLAYWRIGHT_BASE_URL) npm run e2e
