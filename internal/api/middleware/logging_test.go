package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoggingPreservesFlusher(t *testing.T) {
	logged := Logging()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			t.Fatal("expected response writer to support http.Flusher")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/jobs/stream", nil)
	rr := httptest.NewRecorder()

	logged.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rr.Code)
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  LogLevel
	}{
		{name: "debug", input: "DEBUG", want: LogLevelDebug},
		{name: "warn", input: "warn", want: LogLevelWarn},
		{name: "error", input: "ERROR", want: LogLevelError},
		{name: "default info", input: "", want: LogLevelInfo},
		{name: "unknown defaults info", input: "verbose", want: LogLevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseLogLevel(tt.input); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestSuccessSampler(t *testing.T) {
	samper := newSuccessSampler(3)

	if !samper.shouldLog(http.MethodGet, "/organizations", http.StatusOK) {
		t.Fatal("expected first request to be logged")
	}
	if samper.shouldLog(http.MethodGet, "/organizations", http.StatusOK) {
		t.Fatal("expected second request to be sampled out")
	}
	if !samper.shouldLog(http.MethodGet, "/organizations", http.StatusOK) {
		t.Fatal("expected third request to be logged")
	}
}
