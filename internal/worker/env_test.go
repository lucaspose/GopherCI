package worker

import (
	"strings"
	"testing"
)

func TestJobEnvDoesNotLeakServerSecrets(t *testing.T) {
	t.Setenv("JWT_SECRET", "super-secret")
	t.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("PATH", "/usr/bin")

	env := jobEnv()

	hasPath := false
	for _, kv := range env {
		if strings.HasPrefix(kv, "JWT_SECRET=") || strings.HasPrefix(kv, "ENCRYPTION_KEY=") {
			t.Fatalf("secret leaked to job environment: %s", kv)
		}
		if kv == "PATH=/usr/bin" {
			hasPath = true
		}
	}
	if !hasPath {
		t.Fatal("expected PATH to be passed to jobs")
	}
}
