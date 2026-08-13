package deploy_test

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedisConfigHasNoUTF8BOM(t *testing.T) {
	data, err := os.ReadFile("redis/redis.conf")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatal("redis.conf must not start with a UTF-8 BOM")
	}
}

func TestStatisticsIntegrationSuitesHaveBuildTag(t *testing.T) {
	repoRoot := findRepoRoot(t)
	files := []string{
		filepath.Join("services", "statistics-service", "internal", "consumer", "integration_test.go"),
		filepath.Join("services", "statistics-service", "internal", "repository", "integration_test.go"),
	}
	for _, file := range files {
		requireIntegrationBuildTag(t, repoRoot, file)
	}
}

// TestPlanIntegrationSuitesHaveBuildTag guards the whole plan repository
// package instead of a fixed list: any test file there that opens PostgreSQL
// would otherwise run during a plain `go test ./...` and fail without a
// database.
func TestPlanIntegrationSuitesHaveBuildTag(t *testing.T) {
	repoRoot := findRepoRoot(t)
	dir := filepath.Join("services", "plan-service", "internal", "repository")
	entries, err := os.ReadDir(filepath.Join(repoRoot, dir))
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, "_test.go") {
			continue
		}
		file := filepath.Join(dir, name)
		data, err := os.ReadFile(filepath.Join(repoRoot, file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if !opensPostgres(string(data)) {
			continue
		}
		requireIntegrationBuildTag(t, repoRoot, file)
	}
}

func opensPostgres(source string) bool {
	for _, marker := range []string{"TEST_DATABASE_ADMIN_DSN", "TEST_DATABASE_DSN", "postgres.Open", "storage.OpenPostgres", "integrationRepo(", "openPostgres("} {
		if strings.Contains(source, marker) {
			return true
		}
	}
	return false
}

func requireIntegrationBuildTag(t *testing.T, repoRoot, file string) {
	t.Helper()
	f, err := os.Open(filepath.Join(repoRoot, file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	scanner := bufio.NewScanner(f)
	firstLineOK := scanner.Scan() && scanner.Text() == "//go:build integration"
	secondLineOK := scanner.Scan() && scanner.Text() == ""
	if err := scanner.Err(); err != nil {
		f.Close()
		t.Fatalf("read %s: %v", file, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close %s: %v", file, err)
	}
	if !firstLineOK || !secondLineOK {
		t.Errorf("%s must start with //go:build integration followed by a blank line", file)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal("get working directory:", err)
	}
	for {
		for _, marker := range []string{"go.mod", "go.work"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("locate repository root from working directory")
		}
		dir = parent
	}
}
