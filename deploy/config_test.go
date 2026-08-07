package deploy_test

import (
	"bytes"
	"os"
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
