package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	count   int
	resetAt time.Time
}

// IPLimiter applies a fixed-window request limit per key (IP).
type IPLimiter struct {
	mu      sync.Mutex
	max     int
	window  time.Duration
	buckets map[string]bucket
}

func NewIPLimiter(max int, window time.Duration) *IPLimiter {
	return &IPLimiter{
		max:     max,
		window:  window,
		buckets: make(map[string]bucket),
	}
}

func (limiter *IPLimiter) Allow(key string, now time.Time) (bool, time.Duration) {
	if limiter.max <= 0 || limiter.window <= 0 {
		return true, 0
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	entry, exists := limiter.buckets[key]
	if !exists || now.After(entry.resetAt) {
		limiter.buckets[key] = bucket{
			count:   1,
			resetAt: now.Add(limiter.window),
		}
		return true, 0
	}

	if entry.count >= limiter.max {
		retryAfter := time.Until(entry.resetAt)
		if retryAfter < 0 {
			retryAfter = 0
		}
		return false, retryAfter
	}

	entry.count++
	limiter.buckets[key] = entry
	return true, 0
}
