# Fitness Check-in MVP

这是一个 Go 多服务健身打卡示例，Gateway 对外提供 REST API，内部服务通过 gRPC 通信，打卡事件通过 Redis Stream 异步送入统计服务。

## 架构与接口

- `api-gateway`：HTTP `:8080`，公开 `/healthz`、`/readyz`、`/metrics`，业务路由前缀为 `/api/v1`。
- `auth-service`：认证与 JWT，gRPC `:9090`，HTTP 健康端点 `:8080`。
- `plan-service`：计划、训练日和训练项目。
- `checkin-service`：打卡、历史和 outbox；使用 Redis 发布 `WorkoutCompleted` 事件。
- `profile-service`：身体指标。
- `statistics-service`：消费事件并提供周/月统计；统计是最终一致的。
- PostgreSQL 为每个业务服务提供独立 schema，Redis 用于 outbox/stream。

Gateway 当前实际路由包括：注册/登录 `POST /api/v1/auth/register`、`POST /api/v1/auth/login`；计划 `POST /api/v1/plans`、训练日 `POST /api/v1/plans/:plan_id/days`、项目 `POST /api/v1/workout-days/:day_id/items`；打卡 `POST /api/v1/checkins`；指标 `POST/GET /api/v1/body-metrics`；统计 `GET /api/v1/statistics/summary?period=week|month`。受保护路由需要 `Authorization: Bearer <access_token>`；写操作的幂等请求使用 `Idempotency-Key`。

## 前置依赖

需要 Go 1.25、Docker Engine/Compose、GNU Make（Windows 可使用 Git Bash/WSL）以及生成 proto 所需的 `protoc` 和 Go 插件。Kubernetes 部署还需要 `kubectl`、可用集群、已构建并推送或导入的 `fitness/*:dev` 镜像。

## 配置与本地启动

Compose 默认只使用本地开发凭据，默认值不得用于生产。可以用未跟踪的 `.env` 或环境变量覆盖 PostgreSQL 数据库名、业务角色密码、Redis、Gateway 端口和 `JWT_SECRET`；Compose 会将这些变量一致传入数据库初始化与对应服务。真实 secret 不应提交。

启动 PostgreSQL、Redis 及完整六服务栈：

```bash
make up
BASE_URL=http://127.0.0.1:8080 make test-e2e
make down
```

`make up` 构建并启动 `deploy/docker-compose.yml` 中的 PostgreSQL、Redis、auth、plan、checkin、profile、statistics 和 api-gateway。只启动依赖：

```bash
docker compose -f deploy/docker-compose.yml up -d postgres redis
```

服务也可以单独用 Docker Compose 的 service 名称启动；服务之间的地址由 Compose 环境变量配置。Windows PowerShell 等价命令示例：`$env:BASE_URL='http://127.0.0.1:8080'; go test ./tests/e2e -count=1`。如果端口被占用，设置 `POSTGRES_PORT`、`REDIS_PORT` 或 `GATEWAY_PORT`。

## Proto

源文件位于 `proto/*/v1/*.proto`，生成文件位于 `proto/gen`。在 Windows 使用：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/generate-proto.ps1
```

常规 shell 下运行 `make proto`，脚本会检查工具并生成 Go protobuf/gRPC 文件。

## 测试

```bash
make test                 # go test ./...
make test-integration     # go test -tags=integration ./...
go test ./tests/...       # E2E 必须设置 BASE_URL，否则明确失败
BASE_URL=http://127.0.0.1:8080 make test-e2e
```

E2E 会执行注册、登录、建计划、训练日、训练项目、打卡、相同幂等键重复打卡、身体指标和统计查询。统计查询最多轮询 20 秒，且验证重复打卡最终只计一次；Redis Stream 的生产事件 payload 由契约测试覆盖，但完整栈 E2E 不直接重放 Redis 消息。`make test-e2e` 强制要求 `BASE_URL`；服务不可达或业务断言失败都会失败，不会静默跳过。

## Kubernetes

开发 kustomization 位于 `deploy/k8s/dev`。首次使用时复制示例并填写本地或密钥管理系统提供的值：

```bash
cp deploy/k8s/dev/secret.env.example deploy/k8s/dev/secret.env
make k8s-up
make k8s-down
```

`k8s-up` 在 `secret.env` 缺失时拒绝执行，不生成 secret，也不会提交它。执行前必须确认 `kubectl config current-context`、namespace 和集群；`kubectl apply/delete -k` 作用于当前 context，误用生产 context 可能造成真实破坏。Gateway 是 NodePort `30080`，Prometheus 服务端口为 `9090`，Grafana 为 `3000`。

本仓库当前验证环境只有 `default` context，目标 API `https://117.50.85.130:6443` 不可达，且没有 Docker Desktop 本地 Kubernetes context，因此不能声称 Kubernetes apply 成功。K8s 镜像也需要先按部署环境构建/发布。

## 可观测性与限制

Gateway 及各服务提供 `/metrics`；K8s 监控资源包含 Prometheus 和 Grafana。当前限制包括：统计依赖 Redis Stream 消费，结果不是同步写入；Compose 是开发编排，凭据仅为本地占位值；K8s secret 只通过未跟踪文件注入；没有为生产提供 ingress、TLS、备份或外部 secret manager。
