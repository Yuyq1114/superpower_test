# Fitness Check-in MVP

这是一个 Go 多服务健身打卡示例，Gateway 对外提供 REST API，内部服务通过 gRPC 通信，打卡事件通过 Redis Stream 异步送入统计服务。

## 架构与接口

- `api-gateway`：HTTP `:8080`，公开 `/healthz`、`/readyz`、`/metrics`，业务路由前缀为 `/api/v1`。
- `auth-service`：认证与 JWT，gRPC `:9090`，HTTP 健康端点 `:8080`。
- `plan-service`：计划、训练日和训练项目。
- `checkin-service`：打卡、历史和 outbox；使用 Redis 发布 `WorkoutCompleted` 事件。
- `profile-service`：身体指标。
- `statistics-service`：消费事件并提供周/月统计；统计是最终一致的。
- `frontend`：React + Vite 单页应用，构建为独立 Nginx 静态镜像，同源反向代理 `/api/v1/*` 到 `api-gateway`；详见 `frontend/README.md`。
- PostgreSQL 为每个业务服务提供独立 schema，Redis 用于 outbox/stream。

Gateway 当前实际路由包括：注册/登录 `POST /api/v1/auth/register`、`POST /api/v1/auth/login`；计划 `POST /api/v1/plans`、训练日 `POST /api/v1/plans/:plan_id/days`、项目 `POST /api/v1/workout-days/:day_id/items`；打卡 `POST /api/v1/checkins`；指标 `POST/GET /api/v1/body-metrics`；统计 `GET /api/v1/statistics/summary?period=week|month`。受保护路由需要 `Authorization: Bearer <access_token>`；写操作的幂等请求使用 `Idempotency-Key`。

## 前置依赖

需要 Go 1.25、Docker Engine/Compose、GNU Make（Windows 可使用 Git Bash/WSL）以及生成 proto 所需的 `protoc` 和 Go 插件。前端需要 Node.js LTS 与 npm（见 `frontend/README.md`）。Kubernetes 部署还需要 `kubectl`、可用集群、已构建并导入到该集群容器运行时的 `fitness/*:dev` 镜像（本地开发用 Docker Desktop 内置的单节点 Kubernetes 时，`docker build` 产物已在同一个镜像存储中可见，无需额外 push/load）。

## 配置与本地启动

Compose 默认只使用本地开发凭据，默认值不得用于生产。可以用未跟踪的 `.env` 或环境变量覆盖 PostgreSQL 数据库名、业务角色密码、Redis、Gateway 端口和 `JWT_SECRET`；Compose 会将这些变量一致传入数据库初始化与对应服务。真实 secret 不应提交。

启动 PostgreSQL、Redis 及完整六服务栈：

```bash
make up
BASE_URL=http://127.0.0.1:8088 make test-e2e
make down
```

`make up` 构建并启动 `deploy/docker-compose.yml` 中的 PostgreSQL、Redis、auth、plan、checkin、profile、statistics 和 api-gateway。只启动依赖：

```bash
docker compose -f deploy/docker-compose.yml up -d postgres redis
```

服务也可以单独用 Docker Compose 的 service 名称启动；服务之间的地址由 Compose 环境变量配置。Windows PowerShell 等价命令示例：`$env:BASE_URL='http://127.0.0.1:8088'; go test -tags=e2e ./tests/e2e -count=1`。如果端口被占用，设置 `POSTGRES_PORT`、`REDIS_PORT` 或 `FRONTEND_PORT`。自 Task 8 起，`api-gateway` 不再发布 host 端口（只在 Compose 网络内 `expose`），必须通过前端同源入口（`frontend`，默认 `FRONTEND_PORT=8088`）访问 `/api/v1/*`、`/healthz`。PostgreSQL 空数据卷初始化时只自动执行 `init.sh`；SQL 挂载在 `/opt/fitness-init/001-init.sql`，由脚本显式执行一次。仓库通过 `.gitattributes` 强制 shell 脚本使用 LF，避免 Windows checkout 产生 CRLF。

## Proto

源文件位于 `proto/*/v1/*.proto`，生成文件位于 `proto/gen`。在 Windows 使用：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/generate-proto.ps1
```

常规 shell 下运行 `make proto`，脚本会检查工具并生成 Go protobuf/gRPC 文件。

## 测试

```bash
make test                 # go test ./...
TEST_DATABASE_DSN=postgres://statistics_service:statistics-local-only@127.0.0.1:5432/fitness?sslmode=disable \
TEST_DATABASE_ADMIN_DSN=postgres://fitness:postgres-local-only@127.0.0.1:5432/fitness?sslmode=disable \
make test-integration     # go test -tags=integration ./...
go test ./...             # 常规测试，不编译或运行带 e2e tag 的测试
BASE_URL=http://127.0.0.1:8088 make test-e2e
```

带 `e2e` tag 的完整栈测试会执行注册、登录、建计划、训练日、训练项目、打卡、相同幂等键重复打卡、身体指标和统计查询。统计查询最多轮询 20 秒，且验证重复打卡最终只计一次；它不直接重放 Redis 消息。`make test-e2e` 先检查 `BASE_URL`，随后执行 `go test -tags=e2e ./tests/e2e -count=1`；直接运行 tagged 测试时，`BASE_URL` 缺失、服务不可达或业务断言失败也都会失败。

PostgreSQL integration 测试必须同时设置 `TEST_DATABASE_DSN`（被测服务的业务角色）和 `TEST_DATABASE_ADMIN_DSN`（仅用于创建、授权和删除随机测试 schema）。profile 与 statistics 迁移测试分别使用 `profile_test_<hex>` 和 `statistics_test_<hex>`；缺少任一变量时测试会 Skip，admin DSN 不会回退到业务 DSN。admin 连接以当前业务角色为 owner 创建 schema，业务连接只在该 schema 内执行迁移和断言，因此业务角色不需要创建任意 schema 的数据库级权限。每个测试只删除自己创建的随机 schema，不会清空或删除固定 `profile_schema` 或 `statistics_schema`，可以并行运行。

重复事件另有两层集成证据：真实 Redis 测试验证同一 `event_id` 会按 at-least-once 语义投递两次；带 `integration` tag 的生产路径测试通过 statistics service 和 GORM repository 的真实 PostgreSQL 事务连续消费同一事件，并断言周汇总 `workout_count=1`、`active_days=1` 且 `processed_events` 仅一行。Redis 投递测试本身不负责数据库幂等，数据库测试也不模拟 Redis consumer。

### 前端

```bash
cd frontend && npm install
make frontend-test                                          # vitest + typecheck + lint
make frontend-build                                         # tsc -b && vite build
npx playwright install chromium                              # 首次使用需要安装浏览器
PLAYWRIGHT_BASE_URL=http://127.0.0.1:8088 make frontend-e2e  # 针对 Compose 的浏览器端到端测试
PLAYWRIGHT_BASE_URL=http://127.0.0.1:30080 make frontend-e2e # 针对 Kubernetes 的同一套测试
```

Playwright 套件（`frontend/e2e/`）覆盖认证会话（`HttpOnly` 刷新 cookie、无 token 落地 Web Storage、登出、未登录跳转）、完整打卡业务流程（注册→计划→训练日→训练项目→打卡→历史→体重→仪表盘统计一致性）、以及登录/注册/仪表盘/表单页面的 axe-core 自动化可访问性检查，在桌面（1280×800）和移动（390×844）两个 viewport 下分别运行。详见 `frontend/README.md`。

## Kubernetes

开发 kustomization 位于 `deploy/k8s/dev`。首次使用时复制示例并填写本地或密钥管理系统提供的值：

```bash
cp deploy/k8s/dev/secret.env.example deploy/k8s/dev/secret.env
make k8s-up
make k8s-down
```

`k8s-up` 在 `secret.env` 缺失时拒绝执行，不生成 secret，也不会提交它。执行前必须确认 `kubectl config current-context`、namespace 和集群；`kubectl apply/delete -k` 作用于当前 context，误用生产 context 可能造成真实破坏。自 Task 8 起，`frontend` 是唯一的浏览器 NodePort `30080`（同源提供静态资源与 `/api/v1/*` 反向代理），`api-gateway` 改为集群内部 `ClusterIP`，不再直接暴露；Prometheus 服务端口为 `9090`，Grafana 为 `3000`。

在应用前，先为每个服务和前端构建 `dev` 镜像（Docker Desktop 内置的单节点 Kubernetes 与本机 `docker build` 共享同一个镜像存储，不需要额外 push/load）：

```bash
docker build -t fitness/api-gateway:dev -f deploy/docker/Dockerfile --build-arg SERVICE=api-gateway .
docker build -t fitness/auth-service:dev -f deploy/docker/Dockerfile --build-arg SERVICE=auth-service .
docker build -t fitness/plan-service:dev -f deploy/docker/Dockerfile --build-arg SERVICE=plan-service .
docker build -t fitness/checkin-service:dev -f deploy/docker/Dockerfile --build-arg SERVICE=checkin-service .
docker build -t fitness/profile-service:dev -f deploy/docker/Dockerfile --build-arg SERVICE=profile-service .
docker build -t fitness/statistics-service:dev -f deploy/docker/Dockerfile --build-arg SERVICE=statistics-service .
docker build -t fitness/frontend:dev -f frontend/Dockerfile frontend
make k8s-up
```

`fitness/*:dev` 标签是可变的：如果集群里已经跑着旧镜像，`kubectl apply -k deploy/k8s/dev` 之后需要 `kubectl -n fitness-dev rollout restart deployment/<name>` 才会真正切到新构建的镜像内容（`imagePullPolicy: IfNotPresent` 不会重新拉取同名 tag）。等待所有 Deployment/StatefulSet ready 后，可以针对 `http://127.0.0.1:30080` 跑与 Compose 相同的 Playwright 套件（见上文“测试 → 前端”）。

前端容器镜像同时支持 Compose 与 Kubernetes：Nginx 反代的上游地址在容器启动时按运行环境渲染（Kubernetes 下通过 Downward API 注入的 `POD_NAMESPACE` 拼出 `api-gateway.<namespace>.svc.cluster.local`，Compose 下直接用短服务名 `api-gateway`），详见 `frontend/nginx/start-nginx.sh`。

## 可观测性与限制

Gateway 及各服务提供 `/metrics`；K8s 监控资源包含 Prometheus 和 Grafana。当前限制包括：统计依赖 Redis Stream 消费，结果不是同步写入；Compose 是开发编排，凭据仅为本地占位值；K8s secret 只通过未跟踪文件注入；没有为生产提供 ingress、TLS、备份或外部 secret manager。
