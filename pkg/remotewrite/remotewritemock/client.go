// Code generated by mockery 2.9.0. DO NOT EDIT.

package remotewritemock

import (
	context "context"

	mimirpb "github.com/grafana/mimir/pkg/mimirpb"
	mock "github.com/stretchr/testify/mock"
)

// Client is an autogenerated mock type for the Client type
type Client struct {
	mock.Mock
}

// Write provides a mock function with given fields: ctx, req
func (_m *Client) Write(ctx context.Context, req *mimirpb.WriteRequest) error {
	ret := _m.Called(ctx, req)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *mimirpb.WriteRequest) error); ok {
		r0 = rf(ctx, req)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
