-- Local development bootstrap only. Override POSTGRES_* and REDIS_PASSWORD
-- through an untracked .env file or the environment for other environments.
CREATE ROLE auth_service LOGIN PASSWORD 'auth-local-only';
CREATE ROLE plan_service LOGIN PASSWORD 'plan-local-only';
CREATE ROLE checkin_service LOGIN PASSWORD 'checkin-local-only';
CREATE ROLE profile_service LOGIN PASSWORD 'profile-local-only';
CREATE ROLE statistics_service LOGIN PASSWORD 'statistics-local-only';

CREATE SCHEMA IF NOT EXISTS auth_schema AUTHORIZATION auth_service;
CREATE SCHEMA IF NOT EXISTS plan_schema AUTHORIZATION plan_service;
CREATE SCHEMA IF NOT EXISTS checkin_schema AUTHORIZATION checkin_service;
CREATE SCHEMA IF NOT EXISTS profile_schema AUTHORIZATION profile_service;
CREATE SCHEMA IF NOT EXISTS statistics_schema AUTHORIZATION statistics_service;

GRANT USAGE, CREATE ON SCHEMA auth_schema TO auth_service;
GRANT USAGE, CREATE ON SCHEMA plan_schema TO plan_service;
GRANT USAGE, CREATE ON SCHEMA checkin_schema TO checkin_service;
GRANT USAGE, CREATE ON SCHEMA profile_schema TO profile_service;
GRANT USAGE, CREATE ON SCHEMA statistics_schema TO statistics_service;

ALTER DEFAULT PRIVILEGES FOR ROLE auth_service IN SCHEMA auth_schema GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO auth_service;
ALTER DEFAULT PRIVILEGES FOR ROLE auth_service IN SCHEMA auth_schema GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO auth_service;
ALTER DEFAULT PRIVILEGES FOR ROLE plan_service IN SCHEMA plan_schema GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO plan_service;
ALTER DEFAULT PRIVILEGES FOR ROLE plan_service IN SCHEMA plan_schema GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO plan_service;
ALTER DEFAULT PRIVILEGES FOR ROLE checkin_service IN SCHEMA checkin_schema GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO checkin_service;
ALTER DEFAULT PRIVILEGES FOR ROLE checkin_service IN SCHEMA checkin_schema GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO checkin_service;
ALTER DEFAULT PRIVILEGES FOR ROLE profile_service IN SCHEMA profile_schema GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO profile_service;
ALTER DEFAULT PRIVILEGES FOR ROLE profile_service IN SCHEMA profile_schema GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO profile_service;
ALTER DEFAULT PRIVILEGES FOR ROLE statistics_service IN SCHEMA statistics_schema GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO statistics_service;
ALTER DEFAULT PRIVILEGES FOR ROLE statistics_service IN SCHEMA statistics_schema GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO statistics_service;

GRANT CONNECT ON DATABASE fitness TO auth_service, plan_service, checkin_service, profile_service, statistics_service;
GRANT CREATE ON DATABASE fitness TO auth_service, plan_service, checkin_service, profile_service, statistics_service;
