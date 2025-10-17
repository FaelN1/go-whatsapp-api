package middleware

import (
	"net/http"
	"time"

	waLog "go.mau.fi/whatsmeow/util/log"
)

func Logging(log waLog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			log.Infof("%s %s %s", r.Method, r.URL.Path, time.Since(start))
		})
	}
}
