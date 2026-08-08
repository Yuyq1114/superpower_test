package deploy_test

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestDeploymentFilesExistAndContainNoBOM(t *testing.T) {
	files := []string{"docker/Dockerfile", "postgres/init.sh", "postgres/init.sql", "k8s/base/kustomization.yaml", "k8s/base/namespace.yaml", "k8s/base/configmap.yaml", "k8s/base/secret.example.yaml", "k8s/base/postgres.yaml", "k8s/base/redis.yaml", "k8s/base/gateway.yaml", "k8s/base/services.yaml", "k8s/dev/kustomization.yaml", "k8s/dev/secret.env.example", "monitoring/prometheus.yaml", "monitoring/grafana.yaml"}
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Errorf("required deployment file %s: %v", name, err)
			continue
		}
		if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
			t.Errorf("%s must not contain a UTF-8 BOM", name)
		}
	}
}

func TestComposePostgresHasSingleEntrypointInitScript(t *testing.T) {
	data, err := os.ReadFile("docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Count(text, "/docker-entrypoint-initdb.d/") != 1 {
		t.Fatalf("compose must mount exactly one automatically scanned init file")
	}
	if !strings.Contains(text, "./postgres/init.sql:/opt/fitness-init/001-init.sql:ro") {
		t.Fatal("compose must mount SQL outside /docker-entrypoint-initdb.d")
	}
}

func TestPostgresInitScriptUsesLFAndNonScannedSQL(t *testing.T) {
	data, err := os.ReadFile("postgres/init.sh")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte{'\r', '\n'}) {
		t.Fatal("postgres init.sh must use LF line endings")
	}
	if !bytes.Contains(data, []byte("-f /opt/fitness-init/001-init.sql")) {
		t.Fatal("postgres init.sh must execute SQL from the non-scanned mount")
	}
}

func TestSecretExampleIsNotRendered(t *testing.T) {
	data, err := os.ReadFile("k8s/base/kustomization.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret.example") {
		t.Fatal("base kustomization must not deploy secret.example.yaml")
	}
}

func TestApplicationDeploymentsDeclareOperationalSafety(t *testing.T) {
	for _, name := range []string{"k8s/base/gateway.yaml", "k8s/base/services.yaml"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, required := range []string{"startupProbe:", "readinessProbe:", "livenessProbe:", "resources:", "runAsNonRoot: true", "readOnlyRootFilesystem: true", "allowPrivilegeEscalation: false", "drop:", "RuntimeDefault", "imagePullPolicy: IfNotPresent"} {
			if !strings.Contains(text, required) {
				t.Errorf("%s missing %q", name, required)
			}
		}
	}
}

func TestDevKustomizationRequiresLocalSecretFile(t *testing.T) {
	path := "k8s/dev/secret.env"
	if _, err := os.Stat(path); err == nil {
		t.Fatal("secret.env must remain untracked and absent from the repository")
	}
	cmd := exec.Command("kubectl", "kustomize", "k8s/dev")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("kustomize unexpectedly succeeded without secret.env")
	}
	if !strings.Contains(string(output), "secret.env") {
		t.Fatalf("failure must explain missing secret.env: %s", output)
	}
}

func TestPostgresInitUsesQuotedPsqlVariablesForRolePasswords(t *testing.T) {
	data, err := os.ReadFile("k8s/base/configmap.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	roles := []string{"auth", "plan", "checkin", "profile", "statistics"}
	for _, role := range roles {
		want := "PASSWORD :'" + role + "_password'"
		if !strings.Contains(text, want) {
			t.Errorf("postgres init missing safe psql variable %q", want)
		}
	}
	if strings.Contains(text, "PASSWORD '${") {
		t.Fatal("postgres init must not use shell placeholders inside quoted heredoc")
	}
}

func TestComposePostgresCredentialsAreSharedWithServices(t *testing.T) {
	data, err := os.ReadFile("docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, variable := range []string{"POSTGRES_DB", "AUTH_DB_PASSWORD", "PLAN_DB_PASSWORD", "CHECKIN_DB_PASSWORD", "PROFILE_DB_PASSWORD", "STATISTICS_DB_PASSWORD"} {
		if strings.Count(text, "${"+variable+":-") < 2 {
			t.Errorf("docker-compose.yml must pass %s to postgres and owning service", variable)
		}
	}
}

func TestPostgresBusinessRolesCannotCreateArbitrarySchemas(t *testing.T) {
	data, err := os.ReadFile("postgres/init.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "GRANT CREATE ON DATABASE") {
		t.Fatal("business roles must not receive database-level CREATE")
	}
	for _, role := range []string{"auth_service", "plan_service", "checkin_service", "profile_service", "statistics_service"} {
		schema := strings.TrimSuffix(role, "_service") + "_schema"
		if !strings.Contains(text, "GRANT USAGE, CREATE ON SCHEMA "+schema+" TO "+role) {
			t.Errorf("%s must retain CREATE in its owned schema", role)
		}
	}
}
