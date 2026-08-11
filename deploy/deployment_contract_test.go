package deploy_test

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestDeploymentFilesExistAndContainNoBOM(t *testing.T) {
	files := []string{"docker/Dockerfile", "postgres/init.sh", "postgres/init.sql", "k8s/base/kustomization.yaml", "k8s/base/namespace.yaml", "k8s/base/configmap.yaml", "k8s/base/secret.example.yaml", "k8s/base/postgres.yaml", "k8s/base/redis.yaml", "k8s/base/gateway.yaml", "k8s/base/frontend.yaml", "k8s/base/services.yaml", "k8s/dev/kustomization.yaml", "k8s/dev/secret.env.example", "monitoring/prometheus.yaml", "monitoring/grafana.yaml"}
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

func TestKubernetesPostgresUsesWritablePGDataSubdirectory(t *testing.T) {
	data, err := os.ReadFile("k8s/base/postgres.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "name: PGDATA") || !strings.Contains(text, "value: /var/lib/postgresql/data/pgdata") {
		t.Fatal("Kubernetes Postgres must place PGDATA in a writable PVC subdirectory")
	}
	if !strings.Contains(text, "runAsNonRoot: true") {
		t.Fatal("Kubernetes Postgres must remain non-root")
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

func TestDevKustomizationUsesIgnoredLocalSecretFile(t *testing.T) {
	path := "k8s/dev/secret.env"
	ignored := exec.Command("git", "check-ignore", "--quiet", "deploy/"+path)
	ignored.Dir = ".."
	if err := ignored.Run(); err != nil {
		t.Fatal("secret.env must remain ignored by Git")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skip("local secret.env is absent; skipping dev overlay render")
	}
	cmd := exec.Command("kubectl", "kustomize", "k8s/dev")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render dev overlay: %v: %s", err, output)
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

func read(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestFrontendIsTheOnlyBrowserNodePort(t *testing.T) {
	frontend := read(t, "k8s/base/frontend.yaml")
	gateway := read(t, "k8s/base/gateway.yaml")
	if !strings.Contains(frontend, "nodePort: 30080") {
		t.Fatal("frontend must expose NodePort 30080")
	}
	if strings.Contains(gateway, "type: NodePort") || strings.Contains(gateway, "nodePort:") {
		t.Fatal("gateway must remain internal")
	}
	if !strings.Contains(gateway, "type: ClusterIP") {
		t.Fatal("gateway service must explicitly declare type: ClusterIP")
	}
}

func TestFrontendDeploymentDeclaresOperationalSafety(t *testing.T) {
	text := read(t, "k8s/base/frontend.yaml")
	for _, required := range []string{
		"startupProbe:", "readinessProbe:", "livenessProbe:", "resources:",
		"runAsNonRoot: true", "readOnlyRootFilesystem: true",
		"allowPrivilegeEscalation: false", "drop: [ALL]", "RuntimeDefault",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("missing %q", required)
		}
	}
}

func TestFrontendDeploymentContract(t *testing.T) {
	text := read(t, "k8s/base/frontend.yaml")
	for _, required := range []string{
		"image: fitness/frontend:dev",
		"imagePullPolicy: IfNotPresent",
		"containerPort: 8080",
		"runAsUser: 101",
		"runAsGroup: 101",
		"httpGet: {path: /healthz, port: http}",
		"httpGet: {path: /api-readyz, port: http}",
		"requests: {cpu: 25m, memory: 32Mi}",
		"limits: {cpu: 200m, memory: 128Mi}",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("k8s/base/frontend.yaml missing %q", required)
		}
	}
	for _, mount := range []string{"mountPath: /tmp", "mountPath: /var/cache/nginx", "mountPath: /var/run"} {
		if !strings.Contains(text, mount) {
			t.Errorf("k8s/base/frontend.yaml missing writable mount %q", mount)
		}
	}
	if strings.Count(text, "emptyDir:") < 3 {
		t.Error("k8s/base/frontend.yaml must declare an emptyDir for each writable nginx directory")
	}
	if !strings.Contains(text, "type: NodePort") {
		t.Error("frontend Service must be type: NodePort")
	}
}

func TestFrontendKustomizationIncludesFrontend(t *testing.T) {
	text := read(t, "k8s/base/kustomization.yaml")
	if !strings.Contains(text, "frontend.yaml") {
		t.Fatal("base kustomization must include frontend.yaml")
	}
}

func TestComposeAddsSameOriginFrontendAndHidesGatewayFromHost(t *testing.T) {
	text := read(t, "docker-compose.yml")
	if !strings.Contains(text, "127.0.0.1:${FRONTEND_PORT:-8088}:8080") {
		t.Fatal("compose must publish the frontend on 127.0.0.1:${FRONTEND_PORT:-8088}:8080")
	}
	gatewayIdx := strings.Index(text, "api-gateway:")
	if gatewayIdx < 0 {
		t.Fatal("compose must define api-gateway service")
	}
	frontendIdx := strings.Index(text, "\n  frontend:")
	if frontendIdx < 0 {
		t.Fatal("compose must define a frontend service")
	}
	gatewayBlockEnd := len(text)
	if frontendIdx > gatewayIdx {
		gatewayBlockEnd = frontendIdx
	}
	gatewayBlock := text[gatewayIdx:gatewayBlockEnd]
	if strings.Contains(gatewayBlock, "ports:") {
		t.Error("api-gateway must no longer publish a host port once the frontend is the same-origin entrypoint")
	}
	if !strings.Contains(gatewayBlock, `expose: ["8080"]`) {
		t.Error("api-gateway must still expose 8080 on the compose network")
	}
}

func TestDockerignoreExcludesBuildArtifactsFromFrontendContext(t *testing.T) {
	// The frontend image is built with `docker build ... frontend` (and Compose
	// build.context: ../frontend), so Docker only honors the .dockerignore that
	// lives at the root of that build context, not the repository root one.
	frontendIgnore := read(t, "../frontend/.dockerignore")
	for _, required := range []string{"node_modules", "dist", "test-results", "playwright-report", "coverage"} {
		if !strings.Contains(frontendIgnore, required) {
			t.Errorf("frontend/.dockerignore must exclude %q from the frontend build context", required)
		}
	}

	rootIgnore := read(t, "../.dockerignore")
	for _, required := range []string{"node_modules", "dist", "test-results", "playwright-report", "coverage"} {
		if !strings.Contains(rootIgnore, required) {
			t.Errorf("root .dockerignore must also exclude %q for consistency", required)
		}
	}
}

func TestE2EAndOperatorHintsPointToTheFrontendEntrypoint(t *testing.T) {
	// Since Task 8, api-gateway no longer publishes a host port; every
	// operator-facing BASE_URL/test-e2e hint must point at the same-origin
	// frontend entrypoint (127.0.0.1:8088) instead of the retired gateway
	// host port (127.0.0.1:8080), or E2E runs will fail to even connect.
	for _, name := range []string{"../tests/e2e/fitness_flow_test.go", "../Makefile", "../README.md"} {
		text := read(t, name)
		if strings.Contains(text, "127.0.0.1:8080") {
			t.Errorf("%s still references the retired gateway host port 127.0.0.1:8080; it must point at the frontend entrypoint 127.0.0.1:8088", name)
		}
	}
	if !strings.Contains(read(t, "../tests/e2e/fitness_flow_test.go"), "127.0.0.1:8088") {
		t.Error("tests/e2e/fitness_flow_test.go must hint at BASE_URL=http://127.0.0.1:8088")
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
