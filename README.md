# Fitness Check-in MVP

## 本地基础设施

`deploy/docker-compose.yml` 仅将 PostgreSQL 和 Redis 绑定到 `127.0.0.1`。默认凭据是本地开发占位值，可通过未跟踪的 `.env` 或环境变量覆盖；不要提交生产 Secret。

启动：

```bash
docker compose -f deploy/docker-compose.yml up -d postgres redis
```

PostgreSQL 初始化脚本创建五个服务角色及各自 schema，并设置 USAGE/CREATE 和表、序列默认权限。业务服务迁移时应使用自己的 `*_service` 角色和 `storage.PostgresSchemaTarget`，不要使用全量 schema 初始化权限。Redis 使用 `REDIS_PASSWORD` 注入 `requirepass`。

集成测试通过 `TEST_DATABASE_DSN` 和 `TEST_REDIS_ADDR` 启用；未设置时会清楚跳过，不影响快速单元测试。
