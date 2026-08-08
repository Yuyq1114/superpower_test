package deploy_test

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
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
