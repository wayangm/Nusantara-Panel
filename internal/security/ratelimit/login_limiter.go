package ratelimit

import (
	"sync"
	"time"
)

type LoginLimiter struct {
	mu      sync.Mutex
	maxFail int
	window  time.Duration
	entries map[string]*entry
}

type entry struct {
	failCount    int
	blockedUntil time.Time
	lastAttempt  time.Time
}

func NewLoginLimiter(maxFail int, window time.Duration) *LoginLimiter {
	if maxFail < 1 {
		maxFail = 5
	}
	if window < time.Second {
		window = 5 * time.Minute
	}
	return &LoginLimiter{
		maxFail: maxFail,
		window:  window,
		entries: make(map[string]*entry),
	}
}

func (l *LoginLimiter) Allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	e := l.entries[key]
	if e == nil {
		return true
	}
	if now.Sub(e.lastAttempt) > l.window {
		delete(l.entries, key)
		return true
	}
	return !now.Before(e.blockedUntil)
}

func (l *LoginLimiter) RegisterFailure(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	e := l.entries[key]
	if e == nil || now.Sub(e.lastAttempt) > l.window {
		e = &entry{}
		l.entries[key] = e
	}
	e.failCount++
	e.lastAttempt = now
	if e.failCount >= l.maxFail {
		e.blockedUntil = now.Add(l.window)
	}
}

func (l *LoginLimiter) RegisterSuccess(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

