# 健身打卡计划微服务 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Go、Gin、gRPC、PostgreSQL、Redis Streams 和 Docker Desktop Kubernetes 上交付可运行的个人健身打卡微服务 MVP。

**Architecture:** 使用单仓库多服务结构。`api-gateway` 对外提供 REST，业务服务通过 gRPC 通信；服务分别拥有 PostgreSQL schema，`checkin-service` 通过可靠事件记录发布 `WorkoutCompleted`，`statistics-service` 幂等消费并生成统计。

**Tech Stack:** Go、Gin、gRPC/Protobuf、PostgreSQL、GORM、Redis Streams、Docker、Kubernetes、Prometheus、Grafana。

## Global Constraints

- 使用邮箱/密码认证，JWT Access Token + Refresh Token。
- 动作由用户手动输入，不建设固定动作库。
- PostgreSQL 使用单实例和服务独立 schema，禁止跨服务直接读写表。
- 服务间使用 gRPC；对外仅由 Gateway 暴露 REST。
- 配置使用 ConfigMap，敏感信息使用 Secret；真实 Secret 不提交 Git。
- 所有写接口和事件消费者必须幂等；外部调用必须设置超时。
- 每个服务提供 `/healthz` 和 `/readyz`，日志使用结构化 JSON，提供 Prometheus 指标。
- 每个任务必须先写失败测试，再实现最小代码，测试通过后单独提交。

---

### Task 1: 仓库骨架与公共模块

**Files:**
- Create: `go.work`, `Makefile`, `.gitignore`, `README.md`
- Create: `pkg/apperror/error.go`, `pkg/config/config.go`, `pkg/observability/log.go`, `pkg/observability/metrics.go`
- Create: `proto/auth/v1/auth.proto`, `proto/plan/v1/plan.proto`, `proto/checkin/v1/checkin.proto`, `proto/profile/v1/profile.proto`, `proto/statistics/v1/statistics.proto`
- Create: `scripts/generate-proto.ps1`
- Test: `pkg/apperror/error_test.go`, `pkg/config/config_test.go`

**Interfaces:**
- `apperror.Code`：`InvalidArgument`, `Unauthenticated`, `PermissionDenied`, `NotFound`, `Conflict`, `Internal`。
- `config.Load(service string) (Config, error)` 读取环境变量并拒绝缺失的必需配置。
- Protobuf 服务定义请求/响应、分页字段和稳定错误码映射；生成代码放在各服务 `internal/gen/`，禁止手工修改。

- [ ] 写配置缺失、默认值和错误码映射的失败测试。
- [ ] 运行 `go test ./pkg/...`，预期新增测试先失败。
- [ ] 实现公共配置、错误和观测性包，补齐 proto 和生成脚本。
- [ ] 运行 `go test ./pkg/...` 与 `make proto`，预期通过。
- [ ] 提交：`git add . && git commit --trailer "Co-authored-by: Cursor <cursoragent@cursor.com>" -m "build: add Go workspace and shared contracts"`。

### Task 2: 数据库、Redis 与本地基础设施

**Files:**
- Create: `deploy/docker-compose.yml`, `deploy/postgres/init.sql`, `deploy/redis/redis.conf`
- Create: `pkg/storage/postgres.go`, `pkg/storage/redis.go`
- Test: `pkg/storage/postgres_test.go`, `pkg/storage/redis_test.go`

**Interfaces:**
- `storage.OpenPostgres(ctx context.Context, cfg config.Config) (*gorm.DB, error)`。
- `storage.OpenRedis(ctx context.Context, cfg config.Config) (*redis.Client, error)`。
- 初始化脚本创建 `auth_schema`、`plan_schema`、`checkin_schema`、`profile_schema`、`statistics_schema`。

- [ ] 先写连接失败、schema 初始化和 Redis Stream 基本连通性测试。
- [ ] 用 `docker compose -f deploy/docker-compose.yml up -d postgres redis` 验证依赖启动，失败测试应能暴露未实现接口。
- [ ] 实现连接池、超时、迁移入口和 Redis 客户端。
- [ ] 运行 `go test ./pkg/storage/...`，预期通过；关闭依赖并确认失败路径可读。
- [ ] 提交：`git add . && git commit --trailer "Co-authored-by: Cursor <cursoragent@cursor.com>" -m "build: add local PostgreSQL and Redis infrastructure"`。

### Task 3: auth-service

**Files:**
- Create: `services/auth-service/cmd/main.go`
- Create: `services/auth-service/internal/model/user.go`, `refresh_token.go`
- Create: `services/auth-service/internal/repository/user.go`, `refresh_token.go`
- Create: `services/auth-service/internal/service/auth.go`, `token.go`
- Create: `services/auth-service/internal/grpc/server.go`
- Test: `services/auth-service/internal/service/auth_test.go`, `token_test.go`, `internal/grpc/server_test.go`

**Interfaces:**
- `AuthService.Register(ctx context.Context, email, password string) (User, TokenPair, error)`。
- `AuthService.Login(ctx context.Context, email, password string) (User, TokenPair, error)`。
- `AuthService.Refresh(ctx context.Context, refreshToken string) (TokenPair, error)`。
- gRPC 实现 `Register`、`Login`、`Refresh`、`Logout`、`GetUser`。

- [ ] 测试邮箱/密码校验、哈希不可逆、重复邮箱、错误凭证、Access/Refresh 过期和刷新令牌撤销。
- [ ] 运行 `go test ./services/auth-service/...`，预期失败。
- [ ] 用 bcrypt/等价安全哈希实现服务，JWT claims 包含 `sub`、`jti`、`exp`，Refresh Token 存哈希并支持撤销。
- [ ] 运行单元与 gRPC 集成测试，预期通过。
- [ ] 提交：`git add . && git commit --trailer "Co-authored-by: Cursor <cursoragent@cursor.com>" -m "feat: add authentication service"`。

### Task 4: plan-service

**Files:**
- Create: `services/plan-service/cmd/main.go`
- Create: `services/plan-service/internal/model/*.go`, `internal/repository/*.go`, `internal/service/plan.go`, `internal/grpc/server.go`
- Test: `services/plan-service/internal/service/plan_test.go`, `internal/grpc/server_test.go`

**Interfaces:**
- `CreatePlan(ctx, userID, CreatePlanInput) (Plan, error)`。
- `UpdatePlan(ctx, userID, planID, UpdatePlanInput) (Plan, error)`。
- `AddWorkoutDay(ctx, userID, planID, WorkoutDayInput) (WorkoutDay, error)`。
- `ListPlans(ctx, userID, page, pageSize int) (Page[Plan], error)`。
- gRPC 提供计划 CRUD、训练日和训练项目管理。

- [ ] 测试资源归属、计划状态、训练参数至少一种组/次/时长、非法日期和分页。
- [ ] 运行该服务测试，确认先失败。
- [ ] 实现 GORM 模型、唯一 ID、用户隔离和 gRPC 映射。
- [ ] 运行 `go test ./services/plan-service/...`，预期通过。
- [ ] 提交：`git add . && git commit --trailer "Co-authored-by: Cursor <cursoragent@cursor.com>" -m "feat: add workout plan service"`。

### Task 5: checkin-service 与可靠事件发布

**Files:**
- Create: `services/checkin-service/cmd/main.go`
- Create: `services/checkin-service/internal/model/checkin.go`, `event.go`
- Create: `services/checkin-service/internal/repository/*.go`, `internal/service/checkin.go`, `internal/events/publisher.go`, `internal/grpc/server.go`
- Test: `services/checkin-service/internal/service/checkin_test.go`, `internal/events/publisher_test.go`

**Interfaces:**
- `Complete(ctx context.Context, userID, workoutItemID string, date time.Time, note string) (Checkin, error)`。
- `ListHistory(ctx context.Context, userID string, from, to time.Time, page, pageSize int) (Page[Checkin], error)`。
- `CalculateStreak(checkins []time.Time) int`。
- 事件 `WorkoutCompleted` 固定包含 `event_id`、`event_type`、`user_id`、`checkin_id`、`completed_at`、`occurred_at`。

- [ ] 测试同一用户/训练项目/日期重复打卡冲突或返回原记录、连续日期计算、跨月查询和事务失败。
- [ ] 测试打卡事务同时写 `outbox_events`，发布成功后标记已发布；Redis 失败时数据可重试。
- [ ] 运行测试确认失败，再实现唯一约束、事务和 outbox publisher。
- [ ] 运行 `go test ./services/checkin-service/...`，预期通过。
- [ ] 提交：`git add . && git commit --trailer "Co-authored-by: Cursor <cursoragent@cursor.com>" -m "feat: add check-in service and workout events"`。

### Task 6: profile-service

**Files:**
- Create: `services/profile-service/cmd/main.go`, `internal/model/metric.go`, `internal/repository/metric.go`, `internal/service/profile.go`, `internal/grpc/server.go`
- Test: `services/profile-service/internal/service/profile_test.go`, `internal/grpc/server_test.go`

**Interfaces:**
- `RecordMetric(ctx context.Context, userID string, input MetricInput) (Metric, error)`。
- `ListMetrics(ctx context.Context, userID string, metricType string, from, to time.Time) ([]Metric, error)`。

- [ ] 测试体重/体脂的单位、非负范围、时间排序和用户隔离。
- [ ] 实现记录与查询接口及 gRPC 映射。
- [ ] 运行 `go test ./services/profile-service/...`，预期通过。
- [ ] 提交：`git add . && git commit --trailer "Co-authored-by: Cursor <cursoragent@cursor.com>" -m "feat: add body metrics service"`。

### Task 7: statistics-service 消费与统计

**Files:**
- Create: `services/statistics-service/cmd/main.go`, `internal/model/statistic.go`, `internal/repository/statistic.go`, `internal/consumer/workout_completed.go`, `internal/service/statistics.go`, `internal/grpc/server.go`
- Test: `services/statistics-service/internal/consumer/workout_completed_test.go`, `internal/service/statistics_test.go`

**Interfaces:**
- `ConsumeWorkoutCompleted(ctx context.Context, event WorkoutCompleted) error`。
- `GetSummary(ctx context.Context, userID string, period Period, start, end time.Time) (Summary, error)`。
- Consumer group 使用 `workout-events`，处理成功 ACK；`processed_events(event_id primary key)` 保证幂等。

- [ ] 测试首次消费、重复消费、失败重试、统计按周/月边界聚合和死信计数。
- [ ] 运行测试确认失败，再实现 consumer group、事务和汇总查询。
- [ ] 运行 `go test ./services/statistics-service/...`，预期通过。
- [ ] 提交：`git add . && git commit --trailer "Co-authored-by: Cursor <cursoragent@cursor.com>" -m "feat: add asynchronous workout statistics"`。

### Task 8: API Gateway

**Files:**
- Create: `services/api-gateway/cmd/main.go`, `internal/auth/middleware.go`, `internal/clients/*.go`, `internal/http/*.go`, `internal/mapper/errors.go`
- Test: `services/api-gateway/internal/auth/middleware_test.go`, `internal/http/handlers_test.go`

**Interfaces:**
- REST 路由：`POST /api/v1/auth/register`、`POST /api/v1/auth/login`、`POST /api/v1/auth/refresh`、`POST /api/v1/auth/logout`。
- REST 路由：`/api/v1/plans`、`/api/v1/checkins`、`/api/v1/body-metrics`、`/api/v1/statistics/summary`。
- `Authorization: Bearer <access-token>`；统一错误响应 `{code, message, request_id}`。

- [ ] 测试 JWT 缺失/过期/错误签名、公开路由、用户 ID 注入和 gRPC 错误映射。
- [ ] 运行 Gateway 测试确认失败，再实现 Gin 路由、middleware、客户端连接和超时。
- [ ] 运行 `go test ./services/api-gateway/...`，预期通过。
- [ ] 提交：`git add . && git commit --trailer "Co-authored-by: Cursor <cursoragent@cursor.com>" -m "feat: add REST API gateway"`。

### Task 9: Docker、Kubernetes 和可观测性

**Files:**
- Create: `deploy/docker/Dockerfile`, `deploy/k8s/base/*.yaml`, `deploy/k8s/dev/kustomization.yaml`
- Create: `deploy/k8s/base/configmap.yaml`, `secret.example.yaml`, `postgres.yaml`, `redis.yaml`, `gateway.yaml`, `services.yaml`
- Create: `deploy/monitoring/prometheus.yaml`, `grafana.yaml`
- Modify: each service `cmd/main.go` to expose health and metrics endpoints

- [ ] 用 `docker build` 构建 Gateway 和每个服务镜像，确认健康检查未配置前的预期失败。
- [ ] 添加 Deployment、ClusterIP Service、ConfigMap、Secret 示例、探针、资源限制和非 root 运行配置。
- [ ] 添加 Prometheus scrape 配置与 Grafana 基础看板；真实 Secret 仅通过本地未跟踪文件注入。
- [ ] 运行 `kubectl apply -k deploy/k8s/dev`、`kubectl get pods`、`kubectl get svc`，预期所有 Pod Ready。
- [ ] 提交：`git add . && git commit --trailer "Co-authored-by: Cursor <cursoragent@cursor.com>" -m "ops: add Docker Kubernetes and monitoring manifests"`。

### Task 10: 端到端验证与文档

**Files:**
- Create: `tests/e2e/fitness_flow_test.go`, `tests/contract/*.go`
- Modify: `README.md`, `Makefile`

**Interfaces:**
- 测试使用 `BASE_URL` 调用 Gateway，覆盖注册、登录、建计划、打卡、重复打卡、身体指标和统计查询。
- Make targets：`test`、`test-integration`、`test-e2e`、`up`、`down`、`proto`、`k8s-up`、`k8s-down`。

- [ ] 先运行 `go test ./tests/...`，确认环境未启动时失败且错误清晰。
- [ ] 启动 Docker Compose/Kubernetes 依赖和服务，执行完整流程并验证最终统计。
- [ ] 增加重复打卡和重复事件消费断言，确保结果只计一次。
- [ ] 运行 `go test ./...`、`go vet ./...`、`git diff --check`。
- [ ] 更新 README 的启动、配置、proto 生成、测试和 Kubernetes 说明。
- [ ] 提交：`git add . && git commit --trailer "Co-authored-by: Cursor <cursoragent@cursor.com>" -m "test: verify fitness check-in end-to-end flow"`。

## Self-Review

- 规格覆盖：目标功能分别由 Task 3-8 覆盖；基础设施、可靠事件、Kubernetes、观测和测试分别由 Task 1-2、5、9-10 覆盖。
- 占位符检查：计划不使用 TBD、TODO 或未定义的“适当处理”描述；每项均给出文件、接口、命令和验收结果。
- 类型一致性：公共 `Page[T]`、`WorkoutCompleted`、认证 Token 接口、打卡 `Complete` 和统计 `GetSummary` 在相邻任务中保持同名和同一语义。
- 范围检查：这是一个完整微服务 MVP，实施应按任务逐步交付；每个任务都有独立测试和提交点。
