package httpapi

import (
	"math"
	"net/http"
	"time"

	"golang.org/x/time/rate"

	"github.com/tprei/sdds/services/api/internal/openapi"
)

const eventRateLimitMaxUsers = 4096

type eventRateLimiters struct {
	global *rate.Limiter
	users  *keyedRateLimiters
	clock  func() time.Time
}

func newEventRateLimiters(limits EventLimits, clock func() time.Time) eventRateLimiters {
	if limits.UserEventsPerMinute < 1 {
		limits.UserEventsPerMinute = 1
	}
	if limits.GlobalEventsPerMinute < 1 {
		limits.GlobalEventsPerMinute = 1
	}
	return eventRateLimiters{
		global: newRequestsPerMinuteLimiter(limits.GlobalEventsPerMinute),
		users:  newKeyedRequestsPerMinuteLimiters(limits.UserEventsPerMinute, eventRateLimitMaxUsers),
		clock:  clock,
	}
}

func (limiters eventRateLimiters) reserve(now time.Time, userID string, count int) (int, bool) {
	if count < 1 {
		return 0, false
	}
	userReservation := limiters.users.limiterFor(userID, now).ReserveN(now, count)
	globalReservation := limiters.global.ReserveN(now, count)
	userDelay := userReservation.DelayFrom(now)
	globalDelay := globalReservation.DelayFrom(now)
	if !userReservation.OK() || !globalReservation.OK() || userDelay > 0 || globalDelay > 0 {
		userReservation.CancelAt(now)
		globalReservation.CancelAt(now)
		delay := userDelay
		if globalDelay > delay {
			delay = globalDelay
		}
		seconds := int(math.Ceil(delay.Seconds()))
		if seconds < 1 {
			seconds = 1
		}
		if seconds > 60 {
			seconds = 60
		}
		return seconds, false
	}
	return 0, true
}

func writeEventRateLimited(w http.ResponseWriter, retryAfter int) {
	if retryAfter < 1 {
		retryAfter = 1
	}
	w.Header().Set("Retry-After", decimalString(retryAfter))
	writeError(w, http.StatusTooManyRequests, openapi.ErrorResponse{Code: openapi.ErrorCodeRateLimited})
}
