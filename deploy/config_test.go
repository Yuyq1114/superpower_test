package deploy_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
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
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate contract test source")
	}
	repoRoot := filepath.Dir(filepath.Dir(sourceFile))
	files := []string{
		filepath.Join(repoRoot, "services", "statistics-service", "internal", "consumer", "integration_test.go"),
		filepath.Join(repoRoot, "services", "statistics-service", "internal", "repository", "integration_test.go"),
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if !strings.HasPrefix(string(data), "//go:build integration\n\n") {
			t.Errorf("%s must start with //go:build integration followed by a blank line", file)
		}
	}
}
