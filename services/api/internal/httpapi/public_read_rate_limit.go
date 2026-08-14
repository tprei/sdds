package httpapi

import (
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

const publicReadRateLimitMaxSources = 4096

// PublicReadLimits configures the per-IP and global rate limiters that guard
// the anonymous-readable content routes.
type PublicReadLimits struct {
	GlobalRequestsPerMinute int
	SourceRequestsPerMinute int
}

// DefaultPublicReadLimits holds limits well above any legitimate single-client
// usage so the limiter only engages under abuse.
func DefaultPublicReadLimits() PublicReadLimits {
	return PublicReadLimits{
		GlobalRequestsPerMinute: 3000,
		SourceRequestsPerMinute: 300,
	}
}

// publicReadRateLimiters guards the public read routes with a global limiter and
// a per-source-IP keyed limiter, reusing the primitives shared with the auth
// rate limiters. It is chi middleware rather than a handler-level call because
// it applies uniformly to the whole public-read route group.
type publicReadRateLimiters struct {
	clock  func() time.Time
	global *rate.Limiter
	source *keyedRateLimiters
}

func newPublicReadRateLimiters(limits PublicReadLimits, clock func() time.Time) publicReadRateLimiters {
	if limits.GlobalRequestsPerMinute < 1 {
		limits.GlobalRequestsPerMinute = 1
	}
	if limits.SourceRequestsPerMinute < 1 {
		limits.SourceRequestsPerMinute = 1
	}
	return publicReadRateLimiters{
		clock:  clock,
		global: newRequestsPerMinuteLimiter(limits.GlobalRequestsPerMinute),
		source: newKeyedRequestsPerMinuteLimiters(limits.SourceRequestsPerMinute, publicReadRateLimitMaxSources),
	}
}

func (limiters publicReadRateLimiters) middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			now := limiters.clock()
			globalReservation := limiters.global.ReserveN(now, 1)
			if delay := rejectDelay(globalReservation, now); delay >= 0 {
				globalReservation.CancelAt(now)
				writeRetryableRateLimited(w, delay)
				return
			}
			sourceReservation := limiters.source.limiterFor(requestSourceKey(r), now).ReserveN(now, 1)
			if delay := rejectDelay(sourceReservation, now); delay >= 0 {
				globalReservation.CancelAt(now)
				sourceReservation.CancelAt(now)
				writeRetryableRateLimited(w, delay)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
