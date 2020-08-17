package api

import (
	"sync"
	"time"

	"github.com/bluele/gcache"
	"golang.org/x/time/rate"
)

const cacheSzie = 1000

type ipRateLimiter struct {
	cache gcache.Cache
	//ips map[string]*rate.Limiter
	mu *sync.RWMutex
	r  rate.Limit
	b  int
}

func newIPRateLimiter(r rate.Limit, b int) *ipRateLimiter {
	return &ipRateLimiter{
		cache: gcache.New(cacheSzie).LRU().Build(),
		//ips: make(map[string]*rate.Limiter),
		mu: &sync.RWMutex{},
		r:  r,
		b:  b,
	}
}

func (i *ipRateLimiter) addIP(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter := rate.NewLimiter(i.r, i.b)
	i.cache.SetWithExpire(ip, limiter, 24*time.Hour)
	//i.ips[ip] = limiter
	return limiter
}

func (i *ipRateLimiter) getLimiter(ip string) *rate.Limiter {
	i.mu.Lock()

	limiter, err := i.cache.Get(ip)
	if err != nil {
		i.mu.Unlock()
		return i.addIP(ip)
	}
	// limiter, ok := i.ips[ip]
	// if !ok {
	// 	i.mu.Unlock()
	// 	return i.AddIP(ip)
	// }
	i.mu.Unlock()
	return limiter.(*rate.Limiter)
}
