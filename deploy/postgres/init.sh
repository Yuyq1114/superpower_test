#!/bin/sh
set -eu
psql -v ON_ERROR_STOP=1 -v postgres_db="$POSTGRES_DB" -v auth_db_password="$AUTH_DB_PASSWORD" -v plan_db_password="$PLAN_DB_PASSWORD" -v checkin_db_password="$CHECKIN_DB_PASSWORD" -v profile_db_password="$PROFILE_DB_PASSWORD" -v statistics_db_password="$STATISTICS_DB_PASSWORD" --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -f /opt/fitness-init/001-init.sql
