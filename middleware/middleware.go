package middleware

import (
	"net/http"

	"golang.org/x/time/rate"
)

func RateLimiter(limit rate.Limit, burst int) func(next http.Handler) http.Handler {
	limiter := rate.NewLimiter(limit, burst)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}