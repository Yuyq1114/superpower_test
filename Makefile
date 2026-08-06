.PHONY: proto test test-integration test-e2e up down k8s-up k8s-down

proto:
	powershell -ExecutionPolicy Bypass -File scripts/generate-proto.ps1

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

test-e2e:
	go test ./tests/...

up:
	docker compose -f deploy/docker-compose.yml up -d

down:
	docker compose -f deploy/docker-compose.yml down

k8s-up:
	kubectl apply -k deploy/k8s/dev

k8s-down:
	kubectl delete -k deploy/k8s/dev
