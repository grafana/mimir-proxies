package htstorage

import (
	"context"
	"flag"
	"io"
	"testing"
	"time"

	"github.com/opentracing/opentracing-go"

	"github.com/stretchr/testify/mock"

	"github.com/bradfitz/gomemcache/memcache"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/dskit/user"

	"github.com/prometheus/prometheus/prompb"

	"github.com/go-kit/log"
	"github.com/grafana/mimir-proxies/pkg/ctxlog"
	"github.com/grafana/mimir-proxies/pkg/memcached/memcachedmock"
)

const (
	someOrgID      = "12345"
	someHostName   = "some.host.internal"
	mockedCacheKey = someOrgID + ":" + someHostName
)

func TestCache_GetAll(t *testing.T) {
	someFrom := time.Unix(12345, 0)
	someCtx := ctxWithOrgID()

	expectedResult := map[string]Host{"foo": {Labels: []prompb.Label{{Name: "bar", Value: "baz"}}, LastReportedTime: someFrom}}

	getterMock := &MockGetter{}
	getterMock.On("GetAll", someCtx, someFrom).Return(expectedResult, nil)

	cachedGetter := NewCachedGetter(
		ctxlog.NewProvider(log.NewNopLogger()),
		getterMock,
		&memcachedmock.Client{},
		&MockCacheKeygen{},
		&MockCacheRecorder{},
		opentracing.NoopTracer{},
		defaultConfig(),
		time.AfterFunc,
	)

	got, err := cachedGetter.GetAll(someCtx, someFrom)
	require.NoError(t, err)
	assert.Equal(t, expectedResult, got)
}

func TestCache_Get(t *testing.T) {
	t.Run("no org ID in the context", func(t *testing.T) {
		recorderMock := &MockCacheRecorder{}
		recorderMock.On("missingOrgID").Once()
		defer recorderMock.AssertExpectations(t)

		cachedGetter := NewCachedGetter(
			ctxlog.NewProvider(log.NewNopLogger()),
			&MockGetter{},
			&memcachedmock.Client{},
			&MockCacheKeygen{},
			recorderMock,
			opentracing.NoopTracer{},
			defaultConfig(),
			time.AfterFunc,
		)

		_, err := cachedGetter.Get(context.Background(), someHostName)
		assert.Error(t, err)
		assert.ErrorIs(t, err, user.ErrNoOrgID)
	})

	t.Run("cache miss and storage failure", func(t *testing.T) {
		mcMock := &memcachedmock.Client{}
		mcMock.On("Get", mockedCacheKey).Return(nil, memcache.ErrCacheMiss)

		ctx := ctxWithOrgID()
		getterMock := &MockGetter{}
		getterMock.On("Get", matchOrgIDCtx(), someHostName).Return(nil, context.DeadlineExceeded)

		recorderMock := &MockCacheRecorder{}
		recorderMock.On("mcGetTotal").Once()
		recorderMock.On("mcGetMiss").Once()
		recorderMock.On("storageGetTotal").Once()
		recorderMock.On("storageGetErr").Once()
		defer recorderMock.AssertExpectations(t)

		cachedGetter := NewCachedGetter(
			ctxlog.NewProvider(log.NewNopLogger()),
			getterMock,
			mcMock,
			mockedKeyGen(),
			recorderMock,
			opentracing.NoopTracer{},
			defaultConfig(),
			time.AfterFunc,
		)

		_, err := cachedGetter.Get(ctx, someHostName)
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("cache miss and storage not found", func(t *testing.T) {
		mcMock := &memcachedmock.Client{}
		mcMock.On("Get", mockedCacheKey).Return(nil, memcache.ErrCacheMiss)
		expectedItem := &memcache.Item{
			Key:        mockedCacheKey,
			Expiration: 600,
			Flags:      cacheFlagNotFoundError,
			Value:      mustMarshalCachedLabels([]prompb.Label{}),
		}
		mcMock.On("Add", expectedItem).Return(nil)

		ctx := ctxWithOrgID()
		getterMock := &MockGetter{}
		getterMock.On("Get", matchOrgIDCtx(), someHostName).Return(nil, NotFoundError{msg: "host not found 🔥"})

		recorderMock := &MockCacheRecorder{}
		recorderMock.On("mcGetTotal").Once()
		recorderMock.On("mcGetMiss").Once()
		recorderMock.On("storageGetTotal").Once()
		recorderMock.On("storageGetNotFound").Once()
		recorderMock.On("mcAddTotal").Once()
		defer recorderMock.AssertExpectations(t)

		cachedGetter := NewCachedGetter(
			ctxlog.NewProvider(log.NewNopLogger()),
			getterMock,
			mcMock,
			mockedKeyGen(),
			recorderMock,
			opentracing.NoopTracer{},
			defaultConfig(),
			time.AfterFunc,
		)

		_, err := cachedGetter.Get(ctx, someHostName)
		assert.ErrorAs(t, err, &NotFoundError{})
	})

	t.Run("cache miss and storage success", func(t *testing.T) {
		// no matter what happens with memcache store, we always succeed here
		for _, test := range []struct {
			name            string
			memcacheError   error
			recorderMethods []string
		}{
			{"memcache success", nil, []string{"mcGetTotal", "mcGetMiss", "storageGetTotal", "mcAddTotal"}},
			{"racy memcache add with another get", memcache.ErrNotStored, []string{"mcGetTotal", "mcGetMiss", "storageGetTotal", "mcAddTotal", "mcAddNotStored"}},
			{"memcache transient error", io.ErrUnexpectedEOF, []string{"mcGetTotal", "mcGetMiss", "storageGetTotal", "mcAddTotal", "mcAddErr"}},
		} {
			t.Run(test.name, func(t *testing.T) {
				expectedLabels := []prompb.Label{{Name: "env", Value: "test"}}

				mcMock := &memcachedmock.Client{}
				mcMock.On("Get", mockedCacheKey).Return(nil, memcache.ErrCacheMiss)
				expectedItem := &memcache.Item{
					Key:        mockedCacheKey,
					Expiration: 600,
					Value:      mustMarshalCachedLabels(expectedLabels),
				}
				mcMock.On("Add", expectedItem).Return(test.memcacheError)

				ctx := ctxWithOrgID()
				getterMock := &MockGetter{}
				getterMock.On("Get", matchOrgIDCtx(), someHostName).Return(expectedLabels, nil)

				recorderMock := &MockCacheRecorder{}
				for _, method := range test.recorderMethods {
					recorderMock.On(method).Once()
				}
				defer recorderMock.AssertExpectations(t)

				cachedGetter := NewCachedGetter(
					ctxlog.NewProvider(log.NewNopLogger()),
					getterMock,
					mcMock,
					mockedKeyGen(),
					recorderMock,
					opentracing.NoopTracer{},
					defaultConfig(),
					time.AfterFunc,
				)

				gotLabels, err := cachedGetter.Get(ctx, someHostName)
				assert.NoError(t, err)
				assert.Equal(t, expectedLabels, gotLabels)
			})
		}
	})

	t.Run("transient memcache error", func(t *testing.T) {
		expectedLabels := []prompb.Label{{Name: "env", Value: "test"}}

		mcMock := &memcachedmock.Client{}
		mcMock.On("Get", mockedCacheKey).Return(nil, io.ErrUnexpectedEOF)

		ctx := ctxWithOrgID()
		getterMock := &MockGetter{}
		getterMock.On("Get", matchOrgIDCtx(), someHostName).Return(expectedLabels, nil)

		recorderMock := &MockCacheRecorder{}
		recorderMock.On("mcGetTotal").Once()
		recorderMock.On("mcGetErr").Once()
		recorderMock.On("storageGetTotal").Once()
		defer recorderMock.AssertExpectations(t)

		cachedGetter := NewCachedGetter(
			ctxlog.NewProvider(log.NewNopLogger()),
			getterMock,
			mcMock,
			mockedKeyGen(),
			recorderMock,
			opentracing.NoopTracer{},
			defaultConfig(),
			time.AfterFunc,
		)

		gotLabels, err := cachedGetter.Get(ctx, someHostName)
		assert.NoError(t, err)
		assert.Equal(t, expectedLabels, gotLabels)

		mcMock.AssertExpectations(t)
	})

	t.Run("malformed cached value", func(t *testing.T) {
		expectedLabels := []prompb.Label{{Name: "env", Value: "test"}}

		mcMock := &memcachedmock.Client{}
		cachedItem := &memcache.Item{
			Key:        mockedCacheKey,
			Expiration: 600,
			Value:      []byte("garbage"),
		}
		mcMock.On("Get", mockedCacheKey).Return(cachedItem, nil)
		mcMock.On("Delete", mockedCacheKey).Return(nil)
		expectedItem := &memcache.Item{
			Key:        mockedCacheKey,
			Expiration: 600,
			Value:      mustMarshalCachedLabels(expectedLabels),
		}
		mcMock.On("Add", expectedItem).Return(nil)

		ctx := ctxWithOrgID()
		getterMock := &MockGetter{}
		getterMock.On("Get", matchOrgIDCtx(), someHostName).Return(expectedLabels, nil)

		recorderMock := &MockCacheRecorder{}
		recorderMock.On("mcGetTotal").Once()
		recorderMock.On("unmarshalError").Once()
		recorderMock.On("mcDeleteAfterFailedUnmarshalTotal")
		recorderMock.On("storageGetTotal").Once()
		recorderMock.On("mcAddTotal").Once()
		defer recorderMock.AssertExpectations(t)

		cachedGetter := NewCachedGetter(
			ctxlog.NewProvider(log.NewNopLogger()),
			getterMock,
			mcMock,
			mockedKeyGen(),
			recorderMock,
			opentracing.NoopTracer{},
			defaultConfig(),
			time.AfterFunc,
		)

		gotLabels, err := cachedGetter.Get(ctx, someHostName)
		assert.NoError(t, err)
		assert.Equal(t, expectedLabels, gotLabels)

		mcMock.AssertExpectations(t)
	})

	t.Run("happy case: cached value", func(t *testing.T) {
		expectedLabels := []prompb.Label{{Name: "env", Value: "test"}}

		mcMock := &memcachedmock.Client{}
		cachedItem := &memcache.Item{
			Key:        mockedCacheKey,
			Expiration: 600,
			Value:      mustMarshalCachedLabels(expectedLabels),
		}
		mcMock.On("Get", mockedCacheKey).Return(cachedItem, nil)

		ctx := ctxWithOrgID()
		getterMock := &MockGetter{}

		recorderMock := &MockCacheRecorder{}
		recorderMock.On("mcGetTotal").Once()
		defer recorderMock.AssertExpectations(t)

		cachedGetter := NewCachedGetter(
			ctxlog.NewProvider(log.NewNopLogger()),
			getterMock,
			mcMock,
			mockedKeyGen(),
			recorderMock,
			opentracing.NoopTracer{},
			defaultConfig(),
			time.AfterFunc,
		)

		gotLabels, err := cachedGetter.Get(ctx, someHostName)
		assert.NoError(t, err)
		assert.Equal(t, expectedLabels, gotLabels)

		getterMock.AssertNotCalled(t, "Get")
	})

	t.Run("happy case: cached not found value", func(t *testing.T) {
		mcMock := &memcachedmock.Client{}
		cachedItem := &memcache.Item{
			Key:        mockedCacheKey,
			Expiration: 600,
			Flags:      cacheFlagNotFoundError,
			Value:      []byte("this is not expected to be unmarshaled"),
		}
		mcMock.On("Get", mockedCacheKey).Return(cachedItem, nil)

		ctx := ctxWithOrgID()
		getterMock := &MockGetter{}

		recorderMock := &MockCacheRecorder{}
		recorderMock.On("mcGetTotal").Once()
		defer recorderMock.AssertExpectations(t)

		cachedGetter := NewCachedGetter(
			ctxlog.NewProvider(log.NewNopLogger()),
			getterMock,
			mcMock,
			mockedKeyGen(),
			recorderMock,
			opentracing.NoopTracer{},
			defaultConfig(),
			time.AfterFunc,
		)

		_, err := cachedGetter.Get(ctx, someHostName)
		assert.ErrorAs(t, err, &NotFoundError{})
	})
}

func TestCache_Set(t *testing.T) {
	someLabels := []prompb.Label{{Name: "env", Value: "test"}}

	t.Run("storage failure", func(t *testing.T) {
		ctx := ctxWithOrgID()

		storageMock := &MockStorage{}
		storageMock.On("Set", matchOrgIDCtx(), someHostName, someLabels).Return(context.DeadlineExceeded)

		recorderMock := &MockCacheRecorder{}
		recorderMock.On("storageSetTotal").Once()
		recorderMock.On("storageSetErr").Once()
		defer recorderMock.AssertExpectations(t)

		cachedStorage := NewCachedStorage(
			ctxlog.NewProvider(log.NewNopLogger()),
			storageMock,
			&memcachedmock.Client{},
			&MockCacheKeygen{},
			recorderMock,
			opentracing.NoopTracer{},
			defaultConfig(),
			time.AfterFunc,
		)

		err := cachedStorage.Set(ctx, someHostName, someLabels)
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("storage and memcache success", func(t *testing.T) {
		ctx := ctxWithOrgID()

		storageMock := &MockStorage{}
		storageMock.On("Set", matchOrgIDCtx(), someHostName, someLabels).Return(nil)

		mcMock := &memcachedmock.Client{}
		expectedItem := &memcache.Item{
			Key:        mockedCacheKey,
			Expiration: 600,
			Value:      mustMarshalCachedLabels(someLabels),
		}
		mcMock.On("Set", expectedItem).Return(nil)

		recorderMock := &MockCacheRecorder{}
		recorderMock.On("storageSetTotal").Once()
		recorderMock.On("mcSetTotal").Once()
		defer recorderMock.AssertExpectations(t)

		cachedStorage := NewCachedStorage(
			ctxlog.NewProvider(log.NewNopLogger()),
			storageMock,
			mcMock,
			mockedKeyGen(),
			recorderMock,
			opentracing.NoopTracer{},
			defaultConfig(),
			time.AfterFunc,
		)
		err := cachedStorage.Set(ctx, someHostName, someLabels)
		assert.NoError(t, err)

		storageMock.AssertExpectations(t)
		mcMock.AssertExpectations(t)
	})

	t.Run("storage success but memcache failed", func(t *testing.T) {
		ctx := ctxWithOrgID()

		invalidated := false

		storageMock := &MockStorage{}
		storageMock.On("Set", matchOrgIDCtx(), someHostName, someLabels).Return(nil)

		mcMock := &memcachedmock.Client{}
		expectedItem := &memcache.Item{
			Key:        mockedCacheKey,
			Expiration: 600,
			Value:      mustMarshalCachedLabels(someLabels),
		}
		mcMock.On("Set", expectedItem).Return(io.ErrUnexpectedEOF)
		mcMock.On("Delete", mockedCacheKey).
			Run(func(args mock.Arguments) { invalidated = true }).
			Return(nil)

		timeAfterFuncCallDuration := make(chan time.Duration)
		timeAfterFuncCalled := make(chan struct{})
		timeAfterFunc := func(d time.Duration, f func()) *time.Timer {
			go func() {
				timeAfterFuncCallDuration <- d
				f()
				close(timeAfterFuncCalled)
			}()
			return nil // we don't use this value in our code
		}

		recorderMock := &MockCacheRecorder{}
		recorderMock.On("storageSetTotal").Once()
		recorderMock.On("mcSetTotal").Once()
		recorderMock.On("mcSetErr").Once()
		recorderMock.On("mcDeleteAfterFailedSetTotal").Once()
		defer recorderMock.AssertExpectations(t)

		cachedStorage := NewCachedStorage(
			ctxlog.NewProvider(log.NewNopLogger()),
			storageMock,
			mcMock,
			mockedKeyGen(),
			recorderMock,
			opentracing.NoopTracer{},
			defaultConfig(),
			timeAfterFunc,
		)

		err := cachedStorage.Set(ctx, someHostName, someLabels)
		assert.NoError(t, err)

		assert.False(t, invalidated)
		afterDuration := <-timeAfterFuncCallDuration
		assert.Equal(t, defaultConfig().CacheInvalidationRetryDelay, afterDuration)
		<-timeAfterFuncCalled
		assert.True(t, invalidated)

		storageMock.AssertExpectations(t)
		mcMock.AssertExpectations(t)
	})
}

func ctxWithOrgID() context.Context {
	return user.InjectOrgID(context.Background(), someOrgID)
}

func defaultConfig() CacheConfig {
	cfg := CacheConfig{}
	fs := flag.NewFlagSet("flags", flag.PanicOnError)
	cfg.RegisterFlags(fs)
	_ = fs.Parse(nil)
	return cfg
}

func mockedKeyGen() *MockCacheKeygen {
	keygen := &MockCacheKeygen{}
	keygen.On("HostKey", someOrgID, someHostName).Return(mockedCacheKey)
	return keygen
}

func matchOrgIDCtx() interface{} {
	return mock.MatchedBy(func(input context.Context) bool {
		orgID, err := user.ExtractOrgID(input)
		return err == nil && orgID == someOrgID
	})
}
