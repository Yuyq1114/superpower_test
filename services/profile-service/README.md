# profile-service

`profile-service` depends only on PostgreSQL. `/healthz` reports process liveness; `/readyz` checks only the PostgreSQL connection. Redis is intentionally not part of this service or its readiness contract.
