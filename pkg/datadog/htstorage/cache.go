package htstorage

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/opentracing/opentracing-go"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/gogo/protobuf/proto"
	"github.com/grafana/dskit/user"
	"github.com/prometheus/prometheus/prompb"

	"github.com/grafana/mimir-proxies/pkg/ctxlog"
	"github.com/grafana/mimir-proxies/pkg/memcached"
)

const (
	cacheFlagNotFoundError = 1 << iota
)

// Cache is a host tags storage implementation that caches the values for individual hosts.
type Cache struct {
	log ctxlog.Provider

	storage  Storage
	mc       memcached.Client
	keygen   CacheKeygen
	recorder CacheRecorder
	tracer   opentracing.Tracer
	cfg      CacheConfig

	timeAfterFunc func(d time.Duration, f func()) *time.Timer
}

type CacheConfig struct {
	Expiration                  time.Duration `yaml:"expiration"`
	CacheInvalidationRetryDelay time.Duration `yaml:"cache_invalidation_retry_delay"`
}

func (c *CacheConfig) RegisterFlags(flags *flag.FlagSet) {
	c.RegisterFlagsWithPrefix("", flags)
}

// RegisterFlagsWithPrefix registers flags, adding the provided prefix if
// needed. If the prefix is not blank and doesn't end with '.', a '.' is
// appended to it.
//
//nolint:gomnd
func (c *CacheConfig) RegisterFlagsWithPrefix(prefix string, flags *flag.FlagSet) {
	if prefix != "" && !strings.HasSuffix(prefix, ".") {
		prefix += "."
	}
	flags.DurationVar(&c.Expiration, prefix+"ht-cache-expiration", 10*time.Minute, "Expiration for cached values. Zero means no expiration. Seconds precision will be used. Should be less than one month.")
	flags.DurationVar(&c.CacheInvalidationRetryDelay, prefix+"ht-cache-invalidation-retry-delay", time.Minute, "RetryDelay to retry cache invalidation if update fails after storing. Zero means disabled. Arbitrary precision.")
}

func NewCachedStorage(
	log ctxlog.Provider,
	storage Storage,
	cache memcached.Client,
	keygen CacheKeygen,
	recorder CacheRecorder,
	tracer opentracing.Tracer,
	cfg CacheConfig,
	timeAfterFunc func(d time.Duration, f func()) *time.Timer,
) Storage {
	return &Cache{
		log: log,

		storage:  storage,
		mc:       cache,
		keygen:   keygen,
		recorder: recorder,
		tracer:   tracer,

		cfg: cfg,

		timeAfterFunc: timeAfterFunc,
	}
}

func NewCachedGetter(
	log ctxlog.Provider,
	getter Getter,
	cache memcached.Client,
	keygen CacheKeygen,
	recorder CacheRecorder,
	tracer opentracing.Tracer,
	cfg CacheConfig,
	timeAfterFunc func(d time.Duration, f func()) *time.Timer,
) Getter {
	return NewCachedStorage(log, panicOnSet{getter}, cache, keygen, recorder, tracer, cfg, timeAfterFunc)
}

// GetAll does not cache values
func (c *Cache) GetAll(ctx context.Context, from time.Time) (map[string]Host, error) {
	return c.storage.GetAll(ctx, from)
}

// Get tries to retrieve the labels from cache
// - if labels are found in cache, they're returned
// - if cache fails to respond successfully, then request will be done to the underlying storage
// - if cache responds with a miss, then value is retrieved from the storage and it's tried to be _added_ to the cache
// We are using the ADD operation instead of SET for cache, to avoid overwriting the recently set value by Set() method in a race condition
// If the value retrieved can't be unmarshaled, then the key will be deleted and it will be treated as a cache miss.
func (c *Cache) Get(ctx context.Context, hostName string) ([]prompb.Label, error) {
	sp, ctx := opentracing.StartSpanFromContextWithTracer(ctx, c.tracer, "htstorage.Cache.Get")
	defer sp.Finish()
	sp.LogKV("hostName", hostName)

	orgID, err := c.orgID(ctx)
	if err != nil {
		c.recorder.missingOrgID()
		return nil, err
	}

	key := c.keygen.HostKey(orgID, hostName)

	item, err := c.mc.Get(key)
	c.recorder.mcGetTotal()
	if err != nil {
		if errors.Is(err, memcache.ErrCacheMiss) {
			c.recorder.mcGetMiss()
			return c.getAndAddCached(ctx, hostName)
		}
		c.recorder.mcGetErr()

		c.log.For(ctx).Warn("msg", "can't read host tags cache", "err", err)
		return c.storageGet(ctx, hostName)
	}

	if item.Flags&cacheFlagNotFoundError > 0 {
		return nil, NotFoundError{msg: "not found host tags were cached"}
	}

	value := &prompb.Labels{}
	if err := proto.Unmarshal(item.Value, value); err != nil {
		c.recorder.unmarshalError()
		c.tryToInvalidateAfterFailedUnmarshal(ctx, hostName)
		c.log.For(ctx).Warn("msg", "can't unmarshal host tags cache entry, will try to overwrite", "err", err)
		return c.getAndAddCached(ctx, hostName)
	}

	return value.Labels, nil
}

func (c *Cache) storageGet(ctx context.Context, hostName string) ([]prompb.Label, error) {
	lbls, err := c.storage.Get(ctx, hostName)
	c.recorder.storageGetTotal()
	if errors.As(err, &NotFoundError{}) {
		c.recorder.storageGetNotFound()
	} else if err != nil {
		c.recorder.storageGetErr()
	}
	return lbls, err
}

func (c *Cache) getAndAddCached(ctx context.Context, hostName string) ([]prompb.Label, error) {
	var flags uint32
	var maybeNotFoundErr error

	labels, err := c.storageGet(ctx, hostName)
	if errors.As(err, &NotFoundError{}) {
		flags |= cacheFlagNotFoundError
		maybeNotFoundErr = err
	} else if err != nil {
		return nil, err
	}

	item := &memcache.Item{
		Key:        c.keygen.HostKey(c.mustOrgID(ctx), hostName),
		Flags:      flags,
		Expiration: int32(c.cfg.Expiration.Seconds()),
		Value:      mustMarshalCachedLabels(labels),
	}

	err = c.mc.Add(item)
	c.recorder.mcAddTotal()
	if errors.Is(err, memcache.ErrNotStored) {
		c.recorder.mcAddNotStored()
		c.log.For(ctx).Debug("msg", "did not update cache (there was a value already)")
		return labels, maybeNotFoundErr
	} else if err != nil {
		c.recorder.mcAddErr()
		c.log.For(ctx).Warn("msg", "can't update host tags cache", "err", err)
		return labels, maybeNotFoundErr
	}

	return labels, maybeNotFoundErr
}

// Set will try to store the value in the storage, and if succeeds, it will overwrite the cached value with the new one.
// If cache write fails, a retry will be scheduled (unless disabled by setting CacheConfig.CacheInvalidationRetryDelay to 0)
// After which the cache will be invalidated. The values retrieved from storage within this period (between Set call and invalidation)
// can be inconsistent.
func (c *Cache) Set(ctx context.Context, hostName string, labels []prompb.Label) error {
	sp, ctx := opentracing.StartSpanFromContextWithTracer(ctx, c.tracer, "htstorage.Cache.Set")
	defer sp.Finish()
	sp.LogKV("hostName", hostName, "labels", fmt.Sprintf("%v", labels))

	err := c.storage.Set(ctx, hostName, labels)
	c.recorder.storageSetTotal()
	if err != nil {
		c.recorder.storageSetErr()
		return err
	}

	c.storeCached(ctx, hostName, labels)
	return nil
}

func (c *Cache) storeCached(ctx context.Context, hostName string, labels []prompb.Label) {
	item := &memcache.Item{
		Key:        c.keygen.HostKey(c.mustOrgID(ctx), hostName),
		Flags:      0, // In the future, if decide to stop marshaling json for performance reasons, we can store the format here
		Expiration: int32(c.cfg.Expiration.Seconds()),
		Value:      mustMarshalCachedLabels(labels),
	}
	err := c.mc.Set(item)
	c.recorder.mcSetTotal()
	if err != nil {
		c.recorder.mcSetErr()
		c.log.For(ctx).Warn("msg", "can't update host tags cache", "err", err)
		c.tryToInvalidateCacheAsyncWithDelayAfterFailedSet(ctx, hostName)
	}
}

func (c *Cache) tryToInvalidateCacheAsyncWithDelayAfterFailedSet(ctx context.Context, hostName string) {
	if c.cfg.CacheInvalidationRetryDelay == 0 {
		c.log.For(ctx).Warn("msg", "cache invalidation retry disabled, won't retry")
		return
	}

	orgID := c.mustOrgID(ctx)
	key := c.keygen.HostKey(orgID, hostName)
	logger := c.log.For(ctx)

	c.timeAfterFunc(c.cfg.CacheInvalidationRetryDelay, func() {
		err := c.mc.Delete(key)
		c.recorder.mcDeleteAfterFailedSetTotal()
		if errors.Is(err, memcache.ErrCacheMiss) {
			c.recorder.mcDeleteAfterFailedSetMiss()
			logger.Warn("msg", "succeeded invalidating cache later: nothing to invalidate")
			return
		} else if err != nil {
			c.recorder.mcDeleteAfterFailedSetErr()
			logger.Warn("msg", "couldn't invalidate cache later", "err", err)
			return
		}
		logger.Info("msg", "succeeded invalidating cache")
	})
}

func (c *Cache) tryToInvalidateAfterFailedUnmarshal(ctx context.Context, hostName string) {
	orgID := c.mustOrgID(ctx)
	key := c.keygen.HostKey(orgID, hostName)

	err := c.mc.Delete(key)
	c.recorder.mcDeleteAfterFailedUnmarshalTotal()
	if errors.Is(err, memcache.ErrCacheMiss) {
		c.recorder.mcDeleteAfterFailedUnmarshalMiss()
		c.log.For(ctx).Warn("msg", "succeeded invalidating failed to unmarshal cached entry: nothing to invalidate")
		return
	} else if err != nil {
		c.recorder.mcDeleteAfterFailedUnmarshalErr()
		c.log.For(ctx).Warn("msg", "couldn't invalidate failed to unmarshal cached entry", "err", err)
		return
	}
	c.log.For(ctx).Info("msg", "succeeded invalidating failed to unmarshal cached entry")
}

func (c *Cache) orgID(ctx context.Context) (string, error) {
	orgID, err := user.ExtractOrgID(ctx)
	if err != nil {
		return "", err
	}

	if orgID == "" {
		return "", fmt.Errorf("org ID was empty (%w)", user.ErrNoOrgID)
	}

	return orgID, nil
}

func (c *Cache) mustOrgID(ctx context.Context) string {
	orgID, err := c.orgID(ctx)
	if err != nil {
		panic(err)
	}
	return orgID
}

// mustMarshalCachedLabels serializes labels as protobuf
// it will panic if something goes wrong, but nothing should go wrong since this is pretty straightforward
func mustMarshalCachedLabels(labels []prompb.Label) []byte {
	data, err := proto.Marshal(&prompb.Labels{Labels: labels})
	if err != nil {
		panic(fmt.Errorf("can't marshal labels: %w", err))
	}
	return data
}

// panicOnSet adapts a Getter to be a Storage, panicking if Set() is called
type panicOnSet struct{ Getter }

func (panicOnSet) Set(_ context.Context, _ string, _ []prompb.Label) error {
	panic("tried to use getter to set")
}
