package deploy_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPostgresPGDataMigrationContract(t *testing.T) {
	data, err := os.ReadFile("k8s/base/postgres.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{
		"initContainers:", "name: migrate-pgdata", "image: postgres:16-alpine",
		"imagePullPolicy: IfNotPresent", "runAsUser: 0", "readOnlyRootFilesystem: true",
		"allowPrivilegeEscalation: false", "drop: [ALL]", "add: [CHOWN, DAC_OVERRIDE, FOWNER]",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("Kubernetes Postgres PGDATA migration missing %q", required)
		}
	}

	config, err := os.ReadFile("k8s/base/configmap.yaml")
	if err != nil {
		t.Fatal(err)
	}
	configText := string(config)
	for _, required := range []string{
		`[ -f "$root/PG_VERSION" ] && legacy=true`,
		`[ -f "$target/PG_VERSION" ] && destination=true`,
		`if [ "$legacy" = true ] && [ "$destination" = true ]; then`,
		`! -name pgdata ! -name lost+found ! -name .snapshot`,
		`chown -R "$uid:$gid" "$target"`,
		`chmod 0700 "$target"`,
	} {
		if !strings.Contains(configText, required) {
			t.Errorf("deployed PGDATA migration script missing %q", required)
		}
	}
}

func TestPostgresPGDataMigrationSmoke(t *testing.T) {
	script, err := filepath.Abs("postgres/migrate-pgdata.sh")
	if err != nil {
		t.Fatal(err)
	}
	run := func(t *testing.T, root string) error {
		t.Helper()
		cmd := exec.Command("sh", script)
		cmd.Env = append(os.Environ(), "POSTGRES_DATA_ROOT="+root, "POSTGRES_UID="+currentUnixID(t, "-u"), "POSTGRES_GID="+currentUnixID(t, "-g"))
		return cmd.Run()
	}

	t.Run("moves legacy cluster", func(t *testing.T) {
		root := t.TempDir()
		mustWrite(t, filepath.Join(root, "PG_VERSION"), "16")
		mustWrite(t, filepath.Join(root, "base", "1"), "data")
		mustWrite(t, filepath.Join(root, "lost+found", "keep"), "filesystem metadata")
		if err := run(t, root); err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{"pgdata/PG_VERSION", "pgdata/base/1", "lost+found/keep"} {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
				t.Fatalf("expected %s after migration: %v", path, err)
			}
		}
	})

	t.Run("existing target is no-op", func(t *testing.T) {
		root := t.TempDir()
		mustWrite(t, filepath.Join(root, "pgdata", "PG_VERSION"), "16")
		mustWrite(t, filepath.Join(root, "sentinel"), "keep")
		if err := run(t, root); err != nil {
			t.Fatal(err)
		}
		if data, err := os.ReadFile(filepath.Join(root, "sentinel")); err != nil || string(data) != "keep" {
			t.Fatal("no-op migration changed unrelated data")
		}
	})

	t.Run("conflicting clusters fail closed", func(t *testing.T) {
		root := t.TempDir()
		mustWrite(t, filepath.Join(root, "PG_VERSION"), "16")
		mustWrite(t, filepath.Join(root, "pgdata", "PG_VERSION"), "16")
		if err := run(t, root); err == nil {
			t.Fatal("migration must fail when legacy and target clusters both exist")
		}
	})

	t.Run("empty volume creates target", func(t *testing.T) {
		root := t.TempDir()
		if err := run(t, root); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(filepath.Join(root, "pgdata"))
		if err != nil || !info.IsDir() {
			t.Fatal("empty volume did not get a pgdata directory")
		}
		if info.Mode().Perm() != 0700 {
			t.Fatalf("pgdata permissions = %o, want 700", info.Mode().Perm())
		}
	})
}

func TestAuthIntegrationUsesAdminProvisionedRandomSchema(t *testing.T) {
	data, err := os.ReadFile("../services/auth-service/internal/repository/integration_test.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{"//go:build integration", "TEST_DATABASE_ADMIN_DSN", "AUTHORIZATION", "MigrateSchema"} {
		if !strings.Contains(text, required) {
			t.Errorf("auth integration setup missing %q", required)
		}
	}
	if strings.Contains(text, "schema := DefaultSchema") || strings.Contains(text, "MigrateSchema(context.Background(), db, DefaultSchema)") {
		t.Fatal("auth integration must not use the fixed auth schema")
	}
}

func currentUnixID(t *testing.T, flag string) string {
	t.Helper()
	output, err := exec.Command("id", flag).Output()
	if err != nil {
		t.Skip("POSIX id command is required for migration smoke test")
	}
	return strings.TrimSpace(string(output))
}

func mustWrite(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
}
