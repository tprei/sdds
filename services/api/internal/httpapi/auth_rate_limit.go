package httpapi

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/tprei/sdds/services/api/internal/openapi"
)

const authRateLimitMaxKeys = 4096

type authRateLimitPurpose string

const (
	authPurposeSignup        authRateLimitPurpose = "signup"
	authPurposeLogin         authRateLimitPurpose = "login"
	authPurposeVerification  authRateLimitPurpose = "verification"
	authPurposePasswordReset authRateLimitPurpose = "password_reset"
)

type authRateLimiters struct {
	global     map[authRateLimitPurpose]*rate.Limiter
	source     map[authRateLimitPurpose]*keyedRateLimiters
	identifier map[authRateLimitPurpose]*keyedRateLimiters
	clock      func() time.Time
}

func newAuthRateLimiters(limits AuthLimits, clock func() time.Time) authRateLimiters {
	limiters := authRateLimiters{
		global:     map[authRateLimitPurpose]*rate.Limiter{},
		source:     map[authRateLimitPurpose]*keyedRateLimiters{},
		identifier: map[authRateLimitPurpose]*keyedRateLimiters{},
		clock:      clock,
	}
	limiters.registerPurpose(authPurposeSignup, limits.SignupRequestsPerMinute, limits.SignupGlobalRequestsPerMinute)
	limiters.registerPurpose(authPurposeLogin, limits.LoginRequestsPerMinute, limits.LoginGlobalRequestsPerMinute)
	limiters.registerPurpose(authPurposePasswordReset, limits.PasswordResetRequestsPerMinute, limits.PasswordResetGlobalRequestsPerMinute)
	limiters.registerPurpose(authPurposeVerification, limits.VerificationRequestsPerMinute, limits.VerificationGlobalRequestsPerMinute)
	return limiters
}

func (limiters authRateLimiters) registerPurpose(purpose authRateLimitPurpose, perSource, globalPerMinute int) {
	if perSource < 1 {
		perSource = 1
	}
	if globalPerMinute < 1 {
		globalPerMinute = 1
	}
	limiters.global[purpose] = newRequestsPerMinuteLimiter(globalPerMinute)
	limiters.source[purpose] = newKeyedRequestsPerMinuteLimiters(perSource, authRateLimitMaxKeys)
	limiters.identifier[purpose] = newKeyedRequestsPerMinuteLimiters(perSource, authRateLimitMaxKeys)
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

func (limiters authRateLimiters) allow(r *http.Request, purpose authRateLimitPurpose, identifier string) (retryAfterSeconds int, allowed bool) {
	now := limiters.clock()
	globalReservation := limiters.global[purpose].ReserveN(now, 1)
	if delay := rejectDelay(globalReservation, now); delay >= 0 {
		globalReservation.CancelAt(now)
		return clampRetryAfter(delay), false
	}
	sourceReservation := limiters.source[purpose].limiterFor(requestSourceKey(r), now).ReserveN(now, 1)
	if delay := rejectDelay(sourceReservation, now); delay >= 0 {
		globalReservation.CancelAt(now)
		sourceReservation.CancelAt(now)
		return clampRetryAfter(delay), false
	}
	identifierReservation := limiters.identifier[purpose].limiterFor(identifier, now).ReserveN(now, 1)
	if delay := rejectDelay(identifierReservation, now); delay >= 0 {
		globalReservation.CancelAt(now)
		sourceReservation.CancelAt(now)
		identifierReservation.CancelAt(now)
		return clampRetryAfter(delay), false
	}
	return 0, true
}

// rejectDelay reports the non-negative delay a reservation would impose, or -1
// when the reservation is immediately admissible. Reservations that exceed the
// limiter burst budget report a negative OK and are treated as rejected with a
// full-minute backoff.
func rejectDelay(reservation *rate.Reservation, now time.Time) int {
	if !reservation.OK() {
		return 60
	}
	delay := reservation.DelayFrom(now)
	if delay <= 0 {
		return -1
	}
	return int(math.Ceil(delay.Seconds()))
}

func clampRetryAfter(seconds int) int {
	if seconds < 1 {
		seconds = 1
	}
	if seconds > 60 {
		seconds = 60
	}
	return seconds
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
	suppliers := make([]func() *rate.Limiter, 0, len(limiters))
	for _, limiter := range limiters {
		limiter := limiter
		suppliers = append(suppliers, func() *rate.Limiter { return limiter })
	}
	return takeRateLimitTokenLazy(now, suppliers...)
}

func takeRateLimitTokenLazy(now time.Time, suppliers ...func() *rate.Limiter) bool {
	reservations := make([]*rate.Reservation, 0, len(suppliers))
	for _, supplier := range suppliers {
		reservation := supplier().ReserveN(now, 1)
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

// writeRetryableRateLimited writes a 429 with the rate_limited error code and,
// when retryAfterSeconds is positive, a Retry-After header. Shared by the auth
// flow limiters and the public-read limiter.
func writeRetryableRateLimited(w http.ResponseWriter, retryAfterSeconds int) {
	if retryAfterSeconds > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
	}
	writeError(w, http.StatusTooManyRequests, openapi.ErrorResponse{Code: openapi.ErrorCodeRateLimited})
}
