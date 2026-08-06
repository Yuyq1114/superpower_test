# 健身打卡计划微服务设计

日期：2026-08-06
状态：待用户审核

## 1. 目标与范围

构建一个面向个人用户的健身打卡 MVP。用户可以使用邮箱和密码注册登录，创建和编辑健身计划，按天安排训练动作，完成训练打卡，查看训练历史与连续打卡天数，记录体重和体脂，并查看周/月训练统计。第一版动作由用户手动输入，不建设固定动作库。

架构为微服务，部署目标是 Docker Desktop Kubernetes。设计保留未来教练和团队功能的扩展边界，但本期不实现多角色协作、社交、排行榜或支付功能。

## 2. 技术方案

- 语言：Go
- 对外接口：API Gateway 提供 REST API
- 内部通信：服务之间使用 gRPC，每个服务维护自己的 proto 契约
- API Gateway：Gin，实现 JWT 校验、路由编排、统一响应、请求日志和 trace/request ID
- 持久化：PostgreSQL 单实例，每个服务使用独立 schema
- 缓存与事件：Redis；缓存用于可选的热点数据和限流，Redis Streams 用于异步事件
- 配置：Kubernetes ConfigMap 管理非敏感配置，Secret 管理 JWT 密钥、数据库密码和 Redis 密码
- 可观测性：结构化 JSON 日志、Prometheus 指标、Grafana 看板
- 部署：每个无状态服务使用 Deployment 管理，使用 Kubernetes Service 提供稳定访问地址；副本数按环境配置

## 3. 服务边界

### api-gateway

对外暴露 REST API，完成认证信息提取、JWT 校验、请求参数初步校验、gRPC 转换、统一错误响应、日志和链路标识。业务服务不直接对公网暴露。

### auth-service

负责用户注册、邮箱/密码登录、Access Token 和 Refresh Token 刷新、退出和用户基础信息。密码只保存安全哈希，Refresh Token 必须支持撤销或失效控制。

### plan-service

负责健身计划、计划中的训练日、训练项目和动作参数。动作以用户输入的名称保存；训练参数支持组数、次数、重量、时长等可选字段。计划数据由该服务独占写入。

### checkin-service

负责完成训练打卡、打卡历史、重复打卡幂等校验和连续打卡基础数据。成功创建打卡后发布 `WorkoutCompleted` 事件。

### profile-service

负责用户体重、体脂及其他身体指标的时间序列记录和查询。

### statistics-service

消费 `WorkoutCompleted` 事件，生成用户周/月训练次数和相关统计。消费必须通过 `event_id` 幂等，重复消息不能重复累计。统计允许最终一致。

## 4. 数据边界

PostgreSQL 使用一个实例，但按服务隔离 schema：`auth_schema`、`plan_schema`、`checkin_schema`、`profile_schema`、`statistics_schema`。服务只能直接读写自己的 schema，不通过 SQL 跨服务访问其他 schema。需要其他领域数据时通过 gRPC 查询。

核心实体包括：用户、刷新令牌、健身计划、训练日、训练项目、打卡记录、身体指标和统计汇总。每个实体使用稳定 ID、创建时间和更新时间；用户关联数据必须带 `user_id` 并在服务层校验归属。

## 5. 关键流程

### 注册与登录

客户端调用 Gateway 的 REST 接口。Gateway 将请求转为 auth-service 的 gRPC 调用。auth-service 校验邮箱格式、密码策略和邮箱唯一性，返回 Token。无效凭证统一返回认证失败，不泄漏账号是否存在的额外信息。

### 创建计划与打卡

用户先创建计划并添加训练日和训练项目。打卡请求由 Gateway 校验 JWT 后转发到 checkin-service。checkin-service 校验请求归属、日期和重复记录；成功后写入打卡记录，再发布事件。打卡接口对相同用户、训练安排和打卡日期提供幂等约束。

### 统计更新

statistics-service 通过 Redis Consumer Group 消费 `workout-events`。处理成功后确认消息，并记录已处理的 `event_id`。处理失败时保留待重试状态；连续失败的消息进入明确的死信处理路径，不能静默丢弃。

## 6. 错误处理与可靠性

REST 和 gRPC 均使用稳定的错误码映射，例如参数错误、未认证、无权限、资源不存在、冲突和内部错误。对外响应不泄漏堆栈、密码、Token 或数据库细节。

服务提供 `/healthz` 存活检查和 `/readyz` 就绪检查。就绪检查应验证必要的数据库和 Redis 连接。数据库写入与事件发布需要明确处理一致性风险：第一版使用本地事务写入打卡和待发布事件记录，再由发布器投递 Redis Stream，避免数据库成功但事件丢失。

所有消费者和写接口都设计为可重试且幂等。外部调用设置超时，Gateway 和 gRPC 客户端记录耗时与错误指标。

## 7. Kubernetes 结构

每个服务对应一个 Deployment 和一个 ClusterIP Service。开发环境默认每个服务 1 个副本；需要验证高可用时配置多副本。配置通过 ConfigMap/Secret 注入，Pod 不保存业务数据。PostgreSQL 和 Redis 本地开发可以使用独立的 Kubernetes 工作负载和持久化卷，生产环境再替换为托管服务或高可用方案。

Gateway 是唯一对外入口，可通过 Ingress 暴露。服务间只通过 Kubernetes Service DNS 访问。后续可加入 HPA、NetworkPolicy、资源 requests/limits 和滚动发布策略。

## 8. 可观测性

日志使用 JSON 输出，至少包含 `service`、`level`、`trace_id`、`request_id`、`user_id`、`message` 和时间戳。Prometheus 指标覆盖请求数量、响应耗时、错误数量、gRPC 调用、数据库连接、Redis Stream 消费延迟、重试和死信数量。Grafana 提供服务健康、请求延迟、错误率和事件消费状态看板。

## 9. 测试策略

- 单元测试：业务规则、参数校验、Token 处理、连续打卡计算和幂等逻辑
- API 集成测试：验证 PostgreSQL、Redis 和服务接口
- gRPC 契约测试：验证 Gateway 与业务服务的 proto 兼容性
- 核心流程测试：注册登录、创建计划、训练打卡、重复打卡、统计事件消费
- 事件消费测试：重复消息、失败重试、恢复后继续消费和死信路径

## 10. 非目标与后续扩展

本期不实现教练/学员关系、团队空间、社交动态、排行榜、短信登录、固定动作库和复杂推荐算法。未来可以新增角色与租户边界，将动作库独立为 catalog-service，并按实际负载拆分数据库或消息系统。

## 11. 验收标准

- 用户能够完成注册、登录和 Token 刷新
- 用户能够创建、编辑和查询自己的健身计划
- 用户能够按计划完成一次训练打卡，重复请求不会创建重复记录
- 用户能够查询历史打卡和连续打卡天数
- 用户能够记录并查询体重和体脂
- 打卡事件能够被统计服务消费，并生成周/月训练统计
- 各服务能在 Docker Desktop Kubernetes 中启动并通过健康检查
- 关键错误、延迟和事件消费状态可通过日志和 Prometheus/Grafana 观察
