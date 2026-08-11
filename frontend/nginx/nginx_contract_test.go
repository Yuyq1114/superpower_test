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

func TestResolverUsesRuntimePlaceholderNotHardcodedDNS(t *testing.T) {
	text := readConf(t)
	if !strings.Contains(text, "resolver __DNS_RESOLVER__ valid=10s ipv6=off;") {
		t.Fatal("nginx.conf must declare a resolver using the __DNS_RESOLVER__ placeholder, substituted at container start")
	}
	for _, hardcoded := range []string{"127.0.0.11", "10.96.0.10", "kube-dns"} {
		if strings.Contains(text, hardcoded) {
			t.Errorf("nginx.conf must not hardcode a platform-specific DNS resolver address %q", hardcoded)
		}
	}
}

func TestApiGatewayUpstreamIsAVariableForLazyResolution(t *testing.T) {
	text := readConf(t)
	if !strings.Contains(text, "set $api_gateway http://api-gateway:8080;") {
		t.Fatal("nginx.conf must declare $api_gateway as a variable so nginx resolves it lazily per-request instead of at startup")
	}
}

func TestApiLocationProxiesWithoutSpaFallback(t *testing.T) {
	blocks := extractLocationBlocks(t, readConf(t))
	body, ok := blocks["/api/v1/"]
	if !ok {
		t.Fatal("missing location /api/v1/ block")
	}
	if !strings.Contains(body, "proxy_pass $api_gateway$request_uri;") {
		t.Error("location /api/v1/ must proxy_pass $api_gateway$request_uri to preserve the original path and query string via the dynamic upstream variable")
	}
	if strings.Contains(body, "proxy_pass http://api-gateway") {
		t.Error("location /api/v1/ must not proxy_pass to a statically resolved api-gateway host")
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
	if !strings.Contains(healthz, "proxy_pass $api_gateway/healthz;") {
		t.Error("/api-healthz must proxy to $api_gateway/healthz via the dynamic upstream variable")
	}

	readyz, ok := blocks["= /api-readyz"]
	if !ok {
		t.Fatal("missing location = /api-readyz block")
	}
	if !strings.Contains(readyz, "proxy_pass $api_gateway/readyz;") {
		t.Error("/api-readyz must proxy to $api_gateway/readyz via the dynamic upstream variable")
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

func TestStartScriptRendersResolverAndExecsNginx(t *testing.T) {
	text := readStartScript(t)
	if !strings.HasPrefix(text, "#!/bin/sh\n") {
		t.Error("start-nginx.sh must start with a POSIX sh shebang")
	}
	if !strings.Contains(text, "set -eu") {
		t.Error("start-nginx.sh must fail fast with set -eu")
	}
	if !strings.Contains(text, "/etc/resolv.conf") {
		t.Error("start-nginx.sh must read the nameserver from /etc/resolv.conf")
	}
	if !strings.Contains(text, `if [ -z "$resolver_raw" ]`) {
		t.Error("start-nginx.sh must fail closed when no nameserver is found")
	}
	if !strings.Contains(text, "__DNS_RESOLVER__") {
		t.Error("start-nginx.sh must substitute the __DNS_RESOLVER__ placeholder")
	}
	if !strings.Contains(text, "/tmp/nginx.conf") {
		t.Error("start-nginx.sh must render the config to a writable /tmp path")
	}
	if !strings.Contains(text, "exec nginx -c") {
		t.Error("start-nginx.sh must exec nginx so it becomes PID 1 and receives signals directly")
	}
}

func readStartScript(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("start-nginx.sh")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "\r\n") {
		t.Fatal("start-nginx.sh must use LF line endings")
	}
	return string(data)
}

func TestStartScriptValidatesStrictIPv4Octets(t *testing.T) {
	text := readStartScript(t)
	if !strings.Contains(text, "normalize_ipv4()") {
		t.Fatal("start-nginx.sh must define a normalize_ipv4 function")
	}
	if !strings.Contains(text, "IFS=.") || !strings.Contains(text, "set -- $addr") {
		t.Error("normalize_ipv4 must split the address on '.' into positional fields")
	}
	if !strings.Contains(text, `if [ "$#" -ne 4 ]`) {
		t.Error("normalize_ipv4 must require exactly four octets")
	}
	if !strings.Contains(text, "''|*[!0-9]*") {
		t.Error("normalize_ipv4 must reject any octet containing non-digit characters (rejects '/', letters, etc.)")
	}
	if !strings.Contains(text, `[ "${#octet}" -gt 3 ]`) {
		t.Error("normalize_ipv4 must reject octets longer than 3 digits")
	}
	if !strings.Contains(text, `[ "$octet" -gt 255 ]`) {
		t.Error("normalize_ipv4 must reject octet values above 255")
	}
}

func TestStartScriptValidatesIPv6WithScopeStrippingAndBrackets(t *testing.T) {
	text := readStartScript(t)
	if !strings.Contains(text, "normalize_ipv6()") {
		t.Fatal("start-nginx.sh must define a normalize_ipv6 function")
	}
	if !strings.Contains(text, "body=${addr%%%*}") {
		t.Error("normalize_ipv6 must strip an optional %zone suffix before validating the address body")
	}
	if !strings.Contains(text, `if [ -z "$body" ]`) {
		t.Error("normalize_ipv6 must reject an empty body (e.g. a zone-only value)")
	}
	if !strings.Contains(text, "*[!0-9A-Fa-f:.]*") {
		t.Error("normalize_ipv6 must whitelist only hex digits, colons, and dots in the address body")
	}
	if !strings.Contains(text, "printf '[%s]' \"$body\"") {
		t.Error("normalize_ipv6 must wrap the validated body in brackets for nginx's resolver directive")
	}
	if !strings.Contains(text, `case "$resolver_raw" in`) || !strings.Contains(text, "*:*)") {
		t.Error("start-nginx.sh must dispatch to normalize_ipv6 only when the raw value contains a colon")
	}
}

func TestStartScriptRejectsInvalidAddressesWithClearDiagnostics(t *testing.T) {
	text := readStartScript(t)
	if !strings.Contains(text, `echo "start-nginx: invalid IPv4 nameserver address: $resolver_raw" >&2`) {
		t.Error("start-nginx.sh must print a clear diagnostic and fail closed for an invalid IPv4 nameserver")
	}
	if !strings.Contains(text, `echo "start-nginx: invalid IPv6 nameserver address: $resolver_raw" >&2`) {
		t.Error("start-nginx.sh must print a clear diagnostic and fail closed for an invalid IPv6 nameserver")
	}
	// Both branches must reach an `exit 1` (not fall through into sed) when
	// normalization fails, so a malformed address can never reach sed.
	invalidIPv4 := regexp.MustCompile(`(?s)if ! resolver_ip=\$\(normalize_ipv4 "\$resolver_raw"\); then\s*\n\s*echo "start-nginx: invalid IPv4[^\n]*\n\s*exit 1\s*\n\s*fi`)
	if !invalidIPv4.MatchString(text) {
		t.Error("start-nginx.sh must exit 1 immediately when normalize_ipv4 fails, before reaching sed")
	}
	invalidIPv6 := regexp.MustCompile(`(?s)if ! resolver_ip=\$\(normalize_ipv6 "\$resolver_raw"\); then\s*\n\s*echo "start-nginx: invalid IPv6[^\n]*\n\s*exit 1\s*\n\s*fi`)
	if !invalidIPv6.MatchString(text) {
		t.Error("start-nginx.sh must exit 1 immediately when normalize_ipv6 fails, before reaching sed")
	}
}

func TestStartScriptUsesSafeSedDelimiterWithNormalizedValue(t *testing.T) {
	text := readStartScript(t)
	if strings.Contains(text, `sed "s/__DNS_RESOLVER__/`) {
		t.Error("start-nginx.sh must not use '/' as the sed delimiter (a bracketed IPv6 body could theoretically contain one, and it invites fragile escaping)")
	}
	if !strings.Contains(text, `sed "s#__DNS_RESOLVER__#${resolver_ip}#g"`) {
		t.Error(`start-nginx.sh must render the config with sed "s#__DNS_RESOLVER__#${resolver_ip}#g" using the validated/normalized resolver_ip, not the raw value`)
	}
	if strings.Contains(text, "${resolver_raw}") {
		t.Error("start-nginx.sh must never feed the unvalidated resolver_raw value into sed")
	}
}

func TestDockerfileInstallsTemplateAndStartScript(t *testing.T) {
	data, err := os.ReadFile("../Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "COPY nginx/nginx.conf /etc/nginx/nginx.conf.template") {
		t.Error("Dockerfile must install nginx.conf as a read-only template, not the live config")
	}
	if strings.Contains(text, "COPY nginx/nginx.conf /etc/nginx/nginx.conf\n") {
		t.Error("Dockerfile must not copy nginx.conf directly to the live config path anymore")
	}
	if !strings.Contains(text, "COPY --chmod=755 nginx/start-nginx.sh /usr/local/bin/start-nginx.sh") {
		t.Error("Dockerfile must install start-nginx.sh as an executable entrypoint helper")
	}
	if !strings.Contains(text, `CMD ["/usr/local/bin/start-nginx.sh"]`) {
		t.Error("Dockerfile must run start-nginx.sh instead of nginx directly")
	}
	if !strings.Contains(text, "USER 101:101") {
		t.Error("Dockerfile must still run as the unprivileged nginx UID/GID 101:101")
	}
}
