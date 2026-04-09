package middleware

import (
	"log"
	"net/http"
	"time"

	"github.com/lucaspose/goci/internal/api/response"
)

const (
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
	reset  = "\033[0m"
)

func colorStatus(code int) string {
	switch {
	case code >= 500:
		return red
	case code >= 400:
		return red
	case code >= 300:
		return yellow
	default:
		return green
	}
}

func Logging() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrapped := &response.WriterResponse{
				ResponseWriter: w,
				StatusCode:     http.StatusOK,
			}
			start := time.Now()
			next.ServeHTTP(wrapped, r)
			duration := time.Since(start)
			color := colorStatus(wrapped.StatusCode)
			log.Printf("[%s] %s → %s%d%s %s (%v)",
    			r.Method,
    			r.URL.Path,
    			color, wrapped.StatusCode, reset,
    			http.StatusText(wrapped.StatusCode),
    			duration.Round(time.Millisecond),
			)
		})
	}
}
