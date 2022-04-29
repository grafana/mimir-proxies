package memcached

import (
	"time"

	"github.com/bradfitz/gomemcache/memcache"
)

type MeasuredClient struct {
	client   Client
	recorder Recorder
	timeNow  func() time.Time
}

func NewMeasuredClient(client Client, recorder Recorder, timeNow func() time.Time) Client {
	return &MeasuredClient{
		client:   client,
		recorder: recorder,
		timeNow:  timeNow,
	}
}

func (mc *MeasuredClient) Get(key string) (item *memcache.Item, err error) {
	defer func(t0 time.Time) {
		mc.recorder.measure("Get", mc.timeNow().Sub(t0), err)
	}(mc.timeNow())
	return mc.client.Get(key)
}

func (mc *MeasuredClient) Add(item *memcache.Item) (err error) {
	defer func(t0 time.Time) {
		mc.recorder.measure("Add", mc.timeNow().Sub(t0), err)
	}(mc.timeNow())
	return mc.client.Add(item)
}

func (mc *MeasuredClient) Set(item *memcache.Item) (err error) {
	defer func(t0 time.Time) {
		mc.recorder.measure("Set", mc.timeNow().Sub(t0), err)
	}(mc.timeNow())
	return mc.client.Set(item)
}

func (mc *MeasuredClient) CompareAndSwap(item *memcache.Item) (err error) {
	defer func(t0 time.Time) {
		mc.recorder.measure("CompareAndSwap", mc.timeNow().Sub(t0), err)
	}(mc.timeNow())
	return mc.client.CompareAndSwap(item)
}

func (mc *MeasuredClient) Delete(key string) (err error) {
	defer func(t0 time.Time) {
		mc.recorder.measure("Delete", mc.timeNow().Sub(t0), err)
	}(mc.timeNow())
	return mc.client.Delete(key)
}
