package memorycache

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/util/httpheader"

	cache "github.com/patrickmn/go-cache"
)

const (
	cleanupIntervalFactor = 2
	cleanupIntervalMin    = 1 * time.Minute
)

type (
	// MemoryCache is plugin MemoryCache.
	MemoryCache struct {
		spec *Spec

		cache *cache.Cache
	}

	// Spec describes the MemoryCache.
	Spec struct {
		ExpirationSeconds uint32   `yaml:"expirationSeconds" v:"gte=1"`
		MaxEntryBytes     uint32   `yaml:"maxEntryBytes" v:"gte=1"`
		Size              uint32   `yaml:"size" v:"gte=1"`
		Codes             []int    `yaml:"codes" v:"gte=1,unique,dive,httpcode"`
		Methods           []string `yaml:"methods" v:"gte=1,unique"`
	}

	cacheEntry struct {
		statusCode int
		header     *httpheader.HTTPHeader
		body       []byte
	}
)

// New creates a MemoryCache.
func New(spec *Spec) *MemoryCache {
	expiration := time.Duration(spec.ExpirationSeconds) * time.Second
	cleanupInterval := expiration * cleanupIntervalFactor
	if cleanupInterval < cleanupIntervalMin {
		cleanupInterval = cleanupIntervalMin
	}
	cache := cache.New(expiration, cleanupInterval)

	return &MemoryCache{
		spec:  spec,
		cache: cache,
	}
}

// Close closes MemoryCache.
func (mc *MemoryCache) Close() {
	// NOTICE: No need to call the below method, just leave them to GC.
	// mc.cache.Flush()
}

func (mc *MemoryCache) key(ctx context.HTTPContext) string {
	r := ctx.Request()
	return fmt.Sprintf("%s%s%s%s", r.Scheme(), r.Host(), r.Path(), r.Method())
}

// Load tries to load cache for HTTPContext.
func (mc *MemoryCache) Load(ctx context.HTTPContext) (loaded bool) {
	// Reference: https://tools.ietf.org/html/rfc7234#section-5.2
	r, w := ctx.Request(), ctx.Response()

	matchMethod := false
	for _, method := range mc.spec.Methods {
		if r.Method() == method {
			matchMethod = true
			break
		}
	}
	if !matchMethod {
		return false
	}

	for _, value := range r.Header().GetAll(httpheader.KeyCacheControl) {
		if strings.Contains(value, "no-cache") {
			return false
		}
	}

	v, ok := mc.cache.Get(mc.key(ctx))
	if ok {
		entry := v.(*cacheEntry)
		w.SetStatusCode(entry.statusCode)
		w.Header().AddFrom(entry.header)
		w.SetBody(bytes.NewReader(entry.body))
		ctx.AddTag("cacheLoad")
	}

	return ok
}

// Store tries to store cache for HTTPContext.
func (mc *MemoryCache) Store(ctx context.HTTPContext) {
	r, w := ctx.Request(), ctx.Response()

	matchMethod := false
	for _, method := range mc.spec.Methods {
		if r.Method() == method {
			matchMethod = true
			break
		}
	}
	if !matchMethod {
		return
	}

	matchCode := false
	for _, code := range mc.spec.Codes {
		if w.StatusCode() == code {
			matchCode = true
			break
		}
	}
	if !matchCode {
		return
	}

	for _, value := range r.Header().GetAll(httpheader.KeyCacheControl) {
		if strings.Contains(value, "no-store") ||
			strings.Contains(value, "no-cache") {
			return
		}
	}
	for _, value := range w.Header().GetAll(httpheader.KeyCacheControl) {
		if strings.Contains(value, "no-store") ||
			strings.Contains(value, "no-cache") ||
			strings.Contains(value, "must-revalidate") {
			return
		}
	}

	key := mc.key(ctx)
	entry := &cacheEntry{
		statusCode: w.StatusCode(),
		header:     w.Header().Copy(),
	}
	ctx.Response().OnFlushBody(func(body []byte, complete bool) []byte {
		if len(entry.body)+len(body) > int(mc.spec.MaxEntryBytes) {
			return body
		}

		entry.body = append(entry.body, body...)
		if complete {
			mc.cache.SetDefault(key, entry)
			ctx.AddTag("cacheStore")
		}

		return body
	})
}
