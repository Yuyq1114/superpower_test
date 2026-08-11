package nginx_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func readConf(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("nginx.conf")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// extractLocationBlocks parses "location <modifier> <path> { ... }" blocks,
// respecting nested braces, and returns a map keyed by the trimmed location
// selector (e.g. "/api/v1/" or "= /healthz").
func extractLocationBlocks(t *testing.T, text string) map[string]string {
	t.Helper()
	blocks := map[string]string{}
	re := regexp.MustCompile(`(?m)^\s*location\s+([^{]+?)\s*\{`)
	matches := re.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		t.Fatal("no location blocks found in nginx.conf")
	}
	for _, m := range matches {
		key := strings.TrimSpace(text[m[2]:m[3]])
		depth := 1
		i := m[1]
		for depth > 0 {
			if i >= len(text) {
				t.Fatalf("unterminated location block for %q", key)
			}
			switch text[i] {
			case '{':
				depth++
			case '}':
				depth--
			}
			i++
		}
		blocks[key] = text[m[1] : i-1]
	}
	return blocks
}

func TestApiLocationProxiesWithoutSpaFallback(t *testing.T) {
	blocks := extractLocationBlocks(t, readConf(t))
	body, ok := blocks["/api/v1/"]
	if !ok {
		t.Fatal("missing location /api/v1/ block")
	}
	if !strings.Contains(body, "proxy_pass http://api-gateway:8080;") {
		t.Error("location /api/v1/ must proxy_pass to http://api-gateway:8080")
	}
	if strings.Contains(body, "try_files") {
		t.Error("location /api/v1/ must not contain a SPA try_files fallback")
	}
}

func TestOnlyRootLocationUsesSpaFallback(t *testing.T) {
	blocks := extractLocationBlocks(t, readConf(t))
	root, ok := blocks["/"]
	if !ok {
		t.Fatal("missing location / block")
	}
	if !strings.Contains(root, "try_files $uri $uri/ /index.html;") {
		t.Error("location / must use try_files $uri $uri/ /index.html")
	}
	for path, body := range blocks {
		if path == "/" {
			continue
		}
		if strings.Contains(body, "/index.html") {
			t.Errorf("location %s must not fall back to /index.html", path)
		}
	}
}

func TestAssetsLocationReturns404ForMissingFiles(t *testing.T) {
	blocks := extractLocationBlocks(t, readConf(t))
	body, ok := blocks["/assets/"]
	if !ok {
		t.Fatal("missing location /assets/ block")
	}
	if !strings.Contains(body, "try_files $uri =404;") {
		t.Error("location /assets/ must return 404 for missing files instead of the SPA shell")
	}
	if !strings.Contains(body, "immutable") {
		t.Error("location /assets/ must be cached as immutable")
	}
}

func TestHealthAndReadyProxyToGatewayCorrectly(t *testing.T) {
	blocks := extractLocationBlocks(t, readConf(t))

	healthz, ok := blocks["= /api-healthz"]
	if !ok {
		t.Fatal("missing location = /api-healthz block")
	}
	if !strings.Contains(healthz, "proxy_pass http://api-gateway:8080/healthz;") {
		t.Error("/api-healthz must proxy to http://api-gateway:8080/healthz")
	}

	readyz, ok := blocks["= /api-readyz"]
	if !ok {
		t.Fatal("missing location = /api-readyz block")
	}
	if !strings.Contains(readyz, "proxy_pass http://api-gateway:8080/readyz;") {
		t.Error("/api-readyz must proxy to http://api-gateway:8080/readyz")
	}

	local, ok := blocks["= /healthz"]
	if !ok {
		t.Fatal("missing location = /healthz block")
	}
	if strings.Contains(local, "proxy_pass") {
		t.Error("/healthz must be answered locally by nginx, not proxied to the gateway")
	}
	if !strings.Contains(local, "200") {
		t.Error("/healthz must return HTTP 200")
	}
}

func TestApiLocationForwardsRequestContext(t *testing.T) {
	text := readConf(t)
	blocks := extractLocationBlocks(t, text)
	body := blocks["/api/v1/"]
	for _, header := range []string{
		"proxy_set_header Host $host;",
		"proxy_set_header X-Forwarded-Proto $scheme;",
		"proxy_set_header X-Request-ID $request_id;",
	} {
		if !strings.Contains(body, header) {
			t.Errorf("location /api/v1/ must set header: %s", header)
		}
	}
	if strings.Contains(text, "X-Forwarded-For") && !strings.Contains(body, "proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;") {
		t.Error("if X-Forwarded-For is forwarded it must use $proxy_add_x_forwarded_for")
	}
}

func TestNginxRunsUnprivilegedWithWritableRuntimePaths(t *testing.T) {
	text := readConf(t)
	if !strings.Contains(text, "pid /tmp/nginx.pid;") {
		t.Error("pid file must live under writable /tmp for a read-only root filesystem")
	}
	if !strings.Contains(text, "listen 8080;") {
		t.Error("server must listen on the unprivileged port 8080")
	}
}
