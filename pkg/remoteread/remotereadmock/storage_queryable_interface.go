// Code generated by mockery v2.20.0. DO NOT EDIT.

package remotereadmock

import (
	mock "github.com/stretchr/testify/mock"

	storage "github.com/grafana/mimir-proxies/pkg/remoteread/storage"
)

// StorageQueryableInterface is an autogenerated mock type for the StorageQueryableInterface type
type StorageQueryableInterface struct {
	mock.Mock
}

// Querier provides a mock function with given fields: mint, maxt
func (_m *StorageQueryableInterface) Querier(mint int64, maxt int64) (storage.Querier, error) {
	ret := _m.Called(mint, maxt)

	var r0 storage.Querier
	var r1 error
	if rf, ok := ret.Get(0).(func(int64, int64) (storage.Querier, error)); ok {
		return rf(mint, maxt)
	}
	if rf, ok := ret.Get(0).(func(int64, int64) storage.Querier); ok {
		r0 = rf(mint, maxt)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(storage.Querier)
		}
	}

	if rf, ok := ret.Get(1).(func(int64, int64) error); ok {
		r1 = rf(mint, maxt)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewStorageQueryableInterface interface {
	mock.TestingT
	Cleanup(func())
}

// NewStorageQueryableInterface creates a new instance of StorageQueryableInterface. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewStorageQueryableInterface(t mockConstructorTestingTNewStorageQueryableInterface) *StorageQueryableInterface {
	mock := &StorageQueryableInterface{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
