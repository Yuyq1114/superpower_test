# statistics-service 消费约束

Redis DLQ 去重状态默认保留 7 天，可通过 Consumer 的 `DedupeTTL` 配置。该 TTL 必须覆盖最大 pending/recovery 窗口和人工恢复窗口；状态过期后语义是 at-least-once，极端恢复延迟可能重复写入 DLQ，但不会因去重状态过期而静默丢失源消息。

DLQ Lua 使用 source stream、DLQ stream 和 dedupe key 多 key 操作。当前实现明确不支持 Redis Cluster，启动配置 `RedisCluster=true` 会拒绝启动；默认部署为单实例 Redis。不要在未完成同 slot key 迁移前宣称 Cluster 支持。
