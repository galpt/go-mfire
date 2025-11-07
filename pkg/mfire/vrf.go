package mfire

import (
	"container/list"
	"encoding/base64"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Vrf generator ported from kotatsu's implementation.
// It reproduces the sequence of RC4 + transforms and final base64url output.

// atob: base64 decode
func atob(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// btoa: base64 encode (then convert to base64url without padding)
func btoa(b []byte) string {
	s := base64.StdEncoding.EncodeToString(b)
	s = strings.ReplaceAll(s, "+", "-")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.TrimRight(s, "=")
	return s
}

func rc4(key, input []byte) []byte {
	s := make([]int, 256)
	for i := 0; i < 256; i++ {
		s[i] = i
	}
	j := 0
	// KSA
	for i := 0; i < 256; i++ {
		j = (j + s[i] + int(key[i%len(key)])&0xFF) & 0xFF
		s[i], s[j] = s[j], s[i]
	}
	// PRGA
	out := make([]byte, len(input))
	i := 0
	j = 0
	for y := 0; y < len(input); y++ {
		i = (i + 1) & 0xFF
		j = (j + s[i]) & 0xFF
		s[i], s[j] = s[j], s[i]
		k := s[(s[i]+s[j])&0xFF]
		out[y] = byte(int(input[y]) ^ k)
	}
	return out
}

func transform(input, initSeedBytes, prefixKeyBytes []byte, prefixLen int, schedule []func(int) int) []byte {
	out := make([]byte, 0, len(input)+prefixLen)
	for i := 0; i < len(input); i++ {
		if i < prefixLen {
			out = append(out, prefixKeyBytes[i])
		}
		transformed := schedule[i%10]((int(input[i])^int(initSeedBytes[i%32]))&0xFF) & 0xFF
		out = append(out, byte(transformed))
	}
	return out
}

func makeScheduleC() []func(int) int {
	return []func(int) int{
		func(c int) int { return (c - 48 + 256) & 0xFF },
		func(c int) int { return (c - 19 + 256) & 0xFF },
		func(c int) int { return (c ^ 241) & 0xFF },
		func(c int) int { return (c - 19 + 256) & 0xFF },
		func(c int) int { return (c + 223) & 0xFF },
		func(c int) int { return (c - 19 + 256) & 0xFF },
		func(c int) int { return (c - 170 + 256) & 0xFF },
		func(c int) int { return (c - 19 + 256) & 0xFF },
		func(c int) int { return (c - 48 + 256) & 0xFF },
		func(c int) int { return (c ^ 8) & 0xFF },
	}
}

func makeScheduleY() []func(int) int {
	return []func(int) int{
		func(c int) int { return ((c << 4) | (c >> 4)) & 0xFF },
		func(c int) int { return (c + 223) & 0xFF },
		func(c int) int { return ((c << 4) | (c >> 4)) & 0xFF },
		func(c int) int { return (c ^ 163) & 0xFF },
		func(c int) int { return (c - 48 + 256) & 0xFF },
		func(c int) int { return (c + 82) & 0xFF },
		func(c int) int { return (c + 223) & 0xFF },
		func(c int) int { return (c - 48 + 256) & 0xFF },
		func(c int) int { return (c ^ 83) & 0xFF },
		func(c int) int { return ((c << 4) | (c >> 4)) & 0xFF },
	}
}

func makeScheduleB() []func(int) int {
	return []func(int) int{
		func(c int) int { return (c - 19 + 256) & 0xFF },
		func(c int) int { return (c + 82) & 0xFF },
		func(c int) int { return (c - 48 + 256) & 0xFF },
		func(c int) int { return (c - 170 + 256) & 0xFF },
		func(c int) int { return ((c << 4) | (c >> 4)) & 0xFF },
		func(c int) int { return (c - 48 + 256) & 0xFF },
		func(c int) int { return (c - 170 + 256) & 0xFF },
		func(c int) int { return (c ^ 8) & 0xFF },
		func(c int) int { return (c + 82) & 0xFF },
		func(c int) int { return (c ^ 163) & 0xFF },
	}
}

func makeScheduleJ() []func(int) int {
	return []func(int) int{
		func(c int) int { return (c + 223) & 0xFF },
		func(c int) int { return ((c << 4) | (c >> 4)) & 0xFF },
		func(c int) int { return (c + 223) & 0xFF },
		func(c int) int { return (c ^ 83) & 0xFF },
		func(c int) int { return (c - 19 + 256) & 0xFF },
		func(c int) int { return (c + 223) & 0xFF },
		func(c int) int { return (c - 170 + 256) & 0xFF },
		func(c int) int { return (c + 223) & 0xFF },
		func(c int) int { return (c - 170 + 256) & 0xFF },
		func(c int) int { return (c ^ 83) & 0xFF },
	}
}

func makeScheduleE() []func(int) int {
	return []func(int) int{
		func(c int) int { return (c + 82) & 0xFF },
		func(c int) int { return (c ^ 83) & 0xFF },
		func(c int) int { return (c ^ 163) & 0xFF },
		func(c int) int { return (c + 82) & 0xFF },
		func(c int) int { return (c - 170 + 256) & 0xFF },
		func(c int) int { return (c ^ 8) & 0xFF },
		func(c int) int { return (c ^ 241) & 0xFF },
		func(c int) int { return (c + 82) & 0xFF },
		func(c int) int { return (c + 176) & 0xFF },
		func(c int) int { return ((c << 4) | (c >> 4)) & 0xFF },
	}
}

// static data (base64 strings taken from kotatsu)
var rc4Keys = map[string]string{
	"l": "u8cBwTi1CM4XE3BkwG5Ble3AxWgnhKiXD9Cr279yNW0=",
	"g": "t00NOJ/Fl3wZtez1xU6/YvcWDoXzjrDHJLL2r/IWgcY=",
	"B": "S7I+968ZY4Fo3sLVNH/ExCNq7gjuOHjSRgSqh6SsPJc=",
	"m": "7D4Q8i8dApRj6UWxXbIBEa1UqvjI+8W0UvPH9talJK8=",
	"F": "0JsmfWZA1kwZeWLk5gfV5g41lwLL72wHbam5ZPfnOVE=",
}

var seeds32 = map[string]string{
	"A": "pGjzSCtS4izckNAOhrY5unJnO2E1VbrU+tXRYG24vTo=",
	"V": "dFcKX9Qpu7mt/AD6mb1QF4w+KqHTKmdiqp7penubAKI=",
	"N": "owp1QIY/kBiRWrRn9TLN2CdZsLeejzHhfJwdiQMjg3w=",
	"P": "H1XbRvXOvZAhyyPaO68vgIUgdAHn68Y6mrwkpIpEue8=",
	"k": "2Nmobf/mpQ7+Dxq1/olPSDj3xV8PZkPbKaucJvVckL0=",
}

var prefixKeys = map[string]string{
	"O": "Rowe+rg/0g==",
	"v": "8cULcnOMJVY8AA==",
	"L": "n2+Og2Gth8Hh",
	"p": "aRpvzH+yoA==",
	"W": "ZB4oBi0=",
}

// vrfCache is a small LRU cache for VRF tokens. It's safe for concurrent use.
type vrfCache struct {
	mu       sync.Mutex
	ll       *list.List
	cache    map[string]*list.Element
	capacity int
}

type cacheEntry struct {
	key   string
	value string
}

func newVrfCache(cap int) *vrfCache {
	return &vrfCache{
		ll:       list.New(),
		cache:    make(map[string]*list.Element, cap),
		capacity: cap,
	}
}

// computeFn should produce the value; if it returns an error the value won't be cached.
func (c *vrfCache) getOrCompute(key string, computeFn func() (string, error)) (string, error) {
	// Fast path without locking for compute? keep simple: lock during check/insert.
	c.mu.Lock()
	if el, ok := c.cache[key]; ok {
		c.ll.MoveToFront(el)
		val := el.Value.(*cacheEntry).value
		c.mu.Unlock()
		return val, nil
	}
	c.mu.Unlock()

	// Compute without holding lock to avoid blocking other goroutines.
	val, err := computeFn()
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// Double-check another goroutine didn't insert while we computed.
	if el, ok := c.cache[key]; ok {
		c.ll.MoveToFront(el)
		return el.Value.(*cacheEntry).value, nil
	}

	// Insert
	ent := &cacheEntry{key: key, value: val}
	el := c.ll.PushFront(ent)
	c.cache[key] = el
	if c.ll.Len() > c.capacity {
		// remove oldest
		tail := c.ll.Back()
		if tail != nil {
			c.ll.Remove(tail)
			delete(c.cache, tail.Value.(*cacheEntry).key)
		}
	}
	return val, nil
}

var (
	// DefaultVrfCacheSize is the default capacity for the VRF LRU cache.
	DefaultVrfCacheSize = 1024

	// defaultVrfCache is the package-level cache instance. Access it via
	// defaultVrfCacheMu to allow safe swapping.
	defaultVrfCache   *vrfCache
	defaultVrfCacheMu sync.RWMutex
)

func init() {
	// allow overriding the default cache size with an environment variable
	if s := os.Getenv("MGFIRE_VRF_CACHE_SIZE"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			DefaultVrfCacheSize = v
		}
	}
	defaultVrfCache = newVrfCache(DefaultVrfCacheSize)
}

// generateNoCache performs the VRF generation without any caching.
func generateNoCache(input string) (string, error) {
	bytes := []byte(input)

	// rc4 1
	k1, _ := atob(rc4Keys["l"])
	bytes = rc4(k1, bytes)

	// step C1
	seedA, _ := atob(seeds32["A"])
	prefO, _ := atob(prefixKeys["O"])
	bytes = transform(bytes, seedA, prefO, 7, makeScheduleC())

	// rc4 2
	k2, _ := atob(rc4Keys["g"])
	bytes = rc4(k2, bytes)

	// step Y
	seedV, _ := atob(seeds32["V"])
	prefV, _ := atob(prefixKeys["v"])
	bytes = transform(bytes, seedV, prefV, 10, makeScheduleY())

	// rc4 3
	k3, _ := atob(rc4Keys["B"])
	bytes = rc4(k3, bytes)

	// step B
	seedN, _ := atob(seeds32["N"])
	prefL, _ := atob(prefixKeys["L"])
	bytes = transform(bytes, seedN, prefL, 9, makeScheduleB())

	// rc4 4
	k4, _ := atob(rc4Keys["m"])
	bytes = rc4(k4, bytes)

	// step J
	seedP, _ := atob(seeds32["P"])
	prefP, _ := atob(prefixKeys["p"])
	bytes = transform(bytes, seedP, prefP, 7, makeScheduleJ())

	// rc4 5
	k5, _ := atob(rc4Keys["F"])
	bytes = rc4(k5, bytes)

	// step E
	seedK, _ := atob(seeds32["k"])
	prefW, _ := atob(prefixKeys["W"])
	bytes = transform(bytes, seedK, prefW, 5, makeScheduleE())

	// base64url encode
	return btoa(bytes), nil
}

// GenerateVrf returns the vrf token for the given input string. It uses an
// in-memory LRU cache to avoid recomputing tokens for repeated queries.
func GenerateVrf(input string) (string, error) {
	// obtain current cache pointer under read lock so SetVrfCacheSize can swap it
	defaultVrfCacheMu.RLock()
	c := defaultVrfCache
	defaultVrfCacheMu.RUnlock()
	if c == nil {
		// defensive initialization
		defaultVrfCacheMu.Lock()
		if defaultVrfCache == nil {
			defaultVrfCache = newVrfCache(DefaultVrfCacheSize)
		}
		c = defaultVrfCache
		defaultVrfCacheMu.Unlock()
	}
	return c.getOrCompute(input, func() (string, error) {
		return generateNoCache(input)
	})
}

// SetVrfCacheSize atomically replaces the package-level VRF cache with a
// new LRU cache of the given size. Passing size <= 0 does nothing.
func SetVrfCacheSize(size int) {
	if size <= 0 {
		return
	}
	nc := newVrfCache(size)
	defaultVrfCacheMu.Lock()
	defaultVrfCache = nc
	DefaultVrfCacheSize = size
	defaultVrfCacheMu.Unlock()
}

// GetVrfCacheSize returns the current VRF cache capacity. If the cache is
// not initialized it returns DefaultVrfCacheSize.
func GetVrfCacheSize() int {
	defaultVrfCacheMu.RLock()
	defer defaultVrfCacheMu.RUnlock()
	if defaultVrfCache == nil {
		return DefaultVrfCacheSize
	}
	return defaultVrfCache.capacity
}
