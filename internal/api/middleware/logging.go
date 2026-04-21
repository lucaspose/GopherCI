package middleware

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lucaspose/goci/internal/api/response"
)

type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError

	defaultSuccessSampleEvery = 20
)

type loggingConfig struct {
	minLevel           LogLevel
	successSampleEvery int
}

type successSampler struct {
	mu     sync.Mutex
	counts map[string]uint64
	every  uint64
}

func newSuccessSampler(every int) *successSampler {
	if every <= 1 {
		every = 1
	}
	return &successSampler{
		counts: make(map[string]uint64),
		every:  uint64(every),
	}
}

func (s *successSampler) shouldLog(method string, path string, status int) bool {
	if s.every <= 1 {
		return true
	}
	key := method + " " + path + " " + strconv.Itoa(status/100)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counts[key]++
	count := s.counts[key]
	return count == 1 || count%s.every == 0
}

func parseLogLevel(raw string) LogLevel {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DEBUG":
		return LogLevelDebug
	case "WARN", "WARNING":
		return LogLevelWarn
	case "ERROR":
		return LogLevelError
	default:
		return LogLevelInfo
	}
}

func levelString(level LogLevel) string {
	switch level {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

func statusLevel(code int) LogLevel {
	switch {
	case code >= 500:
		return LogLevelError
	case code >= 400:
		return LogLevelWarn
	default:
		return LogLevelInfo
	}
}

func loadLoggingConfig() loggingConfig {
	level := parseLogLevel(os.Getenv("LOG_LEVEL"))
	sampleEvery := defaultSuccessSampleEvery
	if raw := strings.TrimSpace(os.Getenv("LOG_SAMPLE_SUCCESS_EVERY")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil && parsed > 0 {
			sampleEvery = parsed
		}
	}
	if level == LogLevelDebug {
		sampleEvery = 1
	}
	return loggingConfig{
		minLevel:           level,
		successSampleEvery: sampleEvery,
	}
}

func Logging() func(http.Handler) http.Handler {
	config := loadLoggingConfig()
	sampler := newSuccessSampler(config.successSampleEvery)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrapped := &response.WriterResponse{
				ResponseWriter: w,
				StatusCode:     http.StatusOK,
			}
			start := time.Now()
			next.ServeHTTP(wrapped, r)
			duration := time.Since(start)
			level := statusLevel(wrapped.StatusCode)
			if level < config.minLevel {
				return
			}
			if level == LogLevelInfo && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
				if !sampler.shouldLog(r.Method, r.URL.Path, wrapped.StatusCode) {
					return
				}
			}

			msg := "[%s] %s %s -> %d %s (%v)"
			args := []any{
				levelString(level),
				r.Method,
				r.URL.Path,
				wrapped.StatusCode,
				http.StatusText(wrapped.StatusCode),
				duration.Round(time.Millisecond),
			}
			if config.minLevel == LogLevelDebug {
				msg = "[%s] %s %s -> %d %s (%v) remote=%s query=%q"
				args = append(args, r.RemoteAddr, r.URL.RawQuery)
			}
			log.Printf(msg, args...)
		})
	}
}
