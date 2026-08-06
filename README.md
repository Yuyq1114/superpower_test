# Fitness Check-in MVP

## 鏈湴鍩虹璁炬柦

`deploy/docker-compose.yml` 浠呭皢 PostgreSQL 鍜?Redis 缁戝畾鍒?`127.0.0.1`銆傞粯璁ゅ嚟鎹槸鏈湴寮€鍙戝崰浣嶅€硷紝鍙€氳繃鏈窡韪殑 `.env` 鎴栫幆澧冨彉閲忚鐩栵紱涓嶈鎻愪氦鐢熶骇 Secret銆?

鍚姩锛?

```bash
docker compose -f deploy/docker-compose.yml up -d postgres redis
```

PostgreSQL 鍒濆鍖栬剼鏈垱寤轰簲涓湇鍔¤鑹插強鍚勮嚜 schema锛屽苟璁剧疆 USAGE/CREATE 鍜岃〃銆佸簭鍒楅粯璁ゆ潈闄愩€備笟鍔℃湇鍔¤縼绉绘椂搴斾娇鐢ㄨ嚜宸辩殑 `*_service` 瑙掕壊鍜?`storage.PostgresSchemaTarget`锛屼笉瑕佷娇鐢ㄥ叏閲?schema 鍒濆鍖栨潈闄愩€俁edis 浣跨敤 `REDIS_PASSWORD` 娉ㄥ叆 `requirepass`銆?

闆嗘垚娴嬭瘯閫氳繃 `TEST_DATABASE_DSN` 鍜?`TEST_REDIS_ADDR` 鍚敤锛涙湭璁剧疆鏃朵細娓呮璺宠繃锛屼笉褰卞搷蹇€熷崟鍏冩祴璇曘€?


## auth-service 渚濊禆

auth-service 浠呬緷璧?PostgreSQL銆傚惎鍔ㄦ椂浼氭墽琛?uth_schema 杩佺Щ锛?readyz 鍙鏌?PostgreSQL銆俁edis 鏄叾浠栨湇鍔＄殑鍩虹璁炬柦锛屼笉鏄?auth-service 鐨勮繍琛屼緷璧栥€?

## plan-service dependencies

plan-service depends only on PostgreSQL and does not use Redis. Its `/readyz` endpoint checks PostgreSQL only; `/healthz` reports process liveness.

## checkin-service reliability
打卡日期统一按 UTC 日历日处理；`ListHistory.streak` 表示截至 UTC 今日且必须包含今日的当前连续天数。打卡使用 user_id 与 idempotency_key 及请求指纹幂等；outbox 采用 lease claim，Redis Stream 至少一次发布，消费者必须使用 event_id 幂等。
