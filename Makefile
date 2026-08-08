.PHONY: proto test test-integration test-e2e up down k8s-up k8s-down

proto:
	powershell -ExecutionPolicy Bypass -File scripts/generate-proto.ps1

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

test-e2e:
	@if [ -z "$(BASE_URL)" ]; then echo "BASE_URL is required, e.g. BASE_URL=http://127.0.0.1:8080 make test-e2e"; exit 2; fi
	BASE_URL=$(BASE_URL) go test ./tests/e2e -count=1

up:
	docker compose -f deploy/docker-compose.yml up -d

down:
	docker compose -f deploy/docker-compose.yml down

k8s-up:
	@if [ ! -f deploy/k8s/dev/secret.env ]; then echo "missing deploy/k8s/dev/secret.env; copy secret.env.example and fill local values without committing it"; exit 2; fi
	kubectl apply -k deploy/k8s/dev

k8s-down:
	kubectl delete -k deploy/k8s/dev
