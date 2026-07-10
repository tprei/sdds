package httpapi

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/tprei/sdds/services/api/internal/openapi"
)

const authRateLimitMaxKeys = 4096

type authRateLimiters struct {
	signupGlobalLimiter   *rate.Limiter
	loginGlobalLimiter    *rate.Limiter
	signupSourceLimiters  *keyedRateLimiters
	loginSourceLimiters   *keyedRateLimiters
	signupAccountLimiters *keyedRateLimiters
	loginAccountLimiters  *keyedRateLimiters
	clock                 func() time.Time
}

func newAuthRateLimiters(limits AuthLimits, clock func() time.Time) authRateLimiters {
	return authRateLimiters{
		signupGlobalLimiter:   newRequestsPerMinuteLimiter(limits.SignupGlobalRequestsPerMinute),
		loginGlobalLimiter:    newRequestsPerMinuteLimiter(limits.LoginGlobalRequestsPerMinute),
		signupSourceLimiters:  newKeyedRequestsPerMinuteLimiters(limits.SignupRequestsPerMinute, authRateLimitMaxKeys),
		loginSourceLimiters:   newKeyedRequestsPerMinuteLimiters(limits.LoginRequestsPerMinute, authRateLimitMaxKeys),
		signupAccountLimiters: newKeyedRequestsPerMinuteLimiters(limits.SignupRequestsPerMinute, authRateLimitMaxKeys),
		loginAccountLimiters:  newKeyedRequestsPerMinuteLimiters(limits.LoginRequestsPerMinute, authRateLimitMaxKeys),
		clock:                 clock,
	}
}

func newRequestsPerMinuteLimiter(requestsPerMinute int) *rate.Limiter {
	return rate.NewLimiter(rate.Every(time.Minute/time.Duration(requestsPerMinute)), requestsPerMinute)
}

func newKeyedRequestsPerMinuteLimiters(requestsPerMinute int, maxKeys int) *keyedRateLimiters {
	return &keyedRateLimiters{
		limit:   rate.Every(time.Minute / time.Duration(requestsPerMinute)),
		burst:   requestsPerMinute,
		maxKeys: max(1, maxKeys),
		entries: map[string]keyedRateLimiterEntry{},
	}
}

func (limiters authRateLimiters) allowSignup(r *http.Request, username string) bool {
	now := limiters.clock()
	return takeRateLimitToken(
		now,
		limiters.signupSourceLimiters.limiterFor(requestSourceKey(r), now),
		limiters.signupAccountLimiters.limiterFor(username, now),
		limiters.signupGlobalLimiter,
	)
}

func (limiters authRateLimiters) allowLogin(r *http.Request, username string) bool {
	now := limiters.clock()
	return takeRateLimitToken(
		now,
		limiters.loginSourceLimiters.limiterFor(requestSourceKey(r), now),
		limiters.loginAccountLimiters.limiterFor(username, now),
		limiters.loginGlobalLimiter,
	)
}

type keyedRateLimiters struct {
	mu      sync.Mutex
	limit   rate.Limit
	burst   int
	maxKeys int
	entries map[string]keyedRateLimiterEntry
}

type keyedRateLimiterEntry struct {
	lastSeen time.Time
	limiter  *rate.Limiter
}

func (limiters *keyedRateLimiters) limiterFor(key string, now time.Time) *rate.Limiter {
	limiters.mu.Lock()
	defer limiters.mu.Unlock()

	if entry, ok := limiters.entries[key]; ok {
		entry.lastSeen = now
		limiters.entries[key] = entry
		return entry.limiter
	}

	if len(limiters.entries) >= limiters.maxKeys {
		limiters.evictOldest()
	}

	limiter := rate.NewLimiter(limiters.limit, limiters.burst)
	limiters.entries[key] = keyedRateLimiterEntry{
		lastSeen: now,
		limiter:  limiter,
	}
	return limiter
}

func (limiters *keyedRateLimiters) evictOldest() {
	var oldestKey string
	var oldestSeen time.Time
	for key, entry := range limiters.entries {
		if oldestKey == "" || entry.lastSeen.Before(oldestSeen) {
			oldestKey = key
			oldestSeen = entry.lastSeen
		}
	}
	delete(limiters.entries, oldestKey)
}

func requestSourceKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "unknown"
}

func takeRateLimitToken(now time.Time, limiters ...*rate.Limiter) bool {
	reservations := make([]*rate.Reservation, 0, len(limiters))
	for _, limiter := range limiters {
		reservation := limiter.ReserveN(now, 1)
		if !reservation.OK() || reservation.DelayFrom(now) > 0 {
			reservation.CancelAt(now)
			for _, previous := range reservations {
				previous.CancelAt(now)
			}
			return false
		}
		reservations = append(reservations, reservation)
	}
	return true
}

func writeRateLimited(w http.ResponseWriter) {
	writeError(w, http.StatusTooManyRequests, openapi.ErrorResponse{Code: openapi.ErrorCodeRateLimited})
}
