package middleware

import (
	"net/http"
	"strings"
)

type TokenValidator func(token string, r *http.Request) bool

func BearerAuth(validator TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			token := ""
			if strings.HasPrefix(auth, "Bearer ") {
				token = strings.TrimPrefix(auth, "Bearer ")
			} else {
				token = r.Header.Get("apikey")
			}

			if token == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if !validator(token, r) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
