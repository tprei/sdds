package httpapi

import (
	"net/http"
	"time"

	"golang.org/x/time/rate"

	"github.com/tprei/sdds/services/api/internal/openapi"
)

type authRateLimiters struct {
	signupLimiter *rate.Limiter
	loginLimiter  *rate.Limiter
	clock         func() time.Time
}

func newAuthRateLimiters(limits AuthLimits, clock func() time.Time) authRateLimiters {
	return authRateLimiters{
		signupLimiter: newRequestsPerMinuteLimiter(limits.SignupRequestsPerMinute),
		loginLimiter:  newRequestsPerMinuteLimiter(limits.LoginRequestsPerMinute),
		clock:         clock,
	}
}

func newRequestsPerMinuteLimiter(requestsPerMinute int) *rate.Limiter {
	return rate.NewLimiter(rate.Every(time.Minute/time.Duration(requestsPerMinute)), requestsPerMinute)
}

func (limiters authRateLimiters) signup(next http.Handler) http.Handler {
	return limiters.limit(next, limiters.signupLimiter)
}

func (limiters authRateLimiters) login(next http.Handler) http.Handler {
	return limiters.limit(next, limiters.loginLimiter)
}

func (limiters authRateLimiters) limit(next http.Handler, limiter *rate.Limiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.AllowN(limiters.clock(), 1) {
			writeRateLimited(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeRateLimited(w http.ResponseWriter) {
	writeError(w, http.StatusTooManyRequests, openapi.ErrorResponse{Code: openapi.ErrorCodeRateLimited})
}
