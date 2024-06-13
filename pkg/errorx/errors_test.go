package errorx

import (
	"context"
	"fmt"
	"testing"

	"github.com/grafana/mimir-proxies/pkg/errorxpb"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"

	grpcStatus "google.golang.org/grpc/status"
)

func TestGRPCStatusRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		err     Error
		wantErr Error
	}{
		{
			name:    "basic internal error",
			err:     Internal{Msg: "internal error"},
			wantErr: Internal{Msg: "grpc Internal: internal error"},
		},
		{
			name:    "wrapped errors are dropped, but message is preserved",
			err:     Internal{Msg: "internal error", Err: fmt.Errorf("some wrapped error")},
			wantErr: Internal{Msg: "grpc Internal: internal error: some wrapped error"},
		},
		{
			name:    "bad request",
			err:     BadRequest{Msg: "bad request", Err: fmt.Errorf("some bad request")},
			wantErr: BadRequest{Msg: "grpc InvalidArgument: bad request: some bad request"},
		},
		{
			name: "requires proxy request",
			err: RequiresProxyRequest{
				Msg:    "requires proxy request",
				Err:    fmt.Errorf("some unsupported function"),
				Reason: "I have my reasons"},
			wantErr: RequiresProxyRequest{
				Msg:    "grpc NotFound: requires proxy request: some unsupported function",
				Reason: "I have my reasons"},
		},
		{
			name:    "disabled",
			err:     Disabled{},
			wantErr: Disabled{},
		},
		{
			name:    "unimplemented",
			err:     Unimplemented{Msg: "we don't do that here"},
			wantErr: Unimplemented{Msg: "grpc Unimplemented: we don't do that here"},
		},
		{
			name:    "UnprocessableEntity",
			err:     UnprocessableEntity{Msg: "what even is this"},
			wantErr: UnprocessableEntity{Msg: "grpc InvalidArgument: what even is this"},
		},
		{
			name:    "Conflict",
			err:     Conflict{Msg: "a conflict"},
			wantErr: Conflict{Msg: "grpc Aborted: a conflict"},
		},
		{
			name:    "TooManyRequests",
			err:     TooManyRequests{Msg: "too much!"},
			wantErr: TooManyRequests{Msg: "grpc ResourceExhausted: too much!"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := tc.err.GRPCStatus()
			got := FromGRPCStatus(s)
			gotErrx, ok := got.(Error)
			require.True(t, ok)

			require.Equal(t, tc.wantErr.Message(), gotErrx.Message())
			if proxyRequest, ok := tc.wantErr.(RequiresProxyRequest); ok {
				require.Equal(t, got.(RequiresProxyRequest).Reason, proxyRequest.Reason)
			}
			require.ErrorAs(t, got, &tc.wantErr)
		})
	}
}

func TestFromGRPCStatusErrors(t *testing.T) {
	tests := []struct {
		name    string
		s       *grpcStatus.Status
		wantErr error
	}{
		{
			name:    "OK is ok",
			s:       grpcStatus.New(codes.OK, ""),
			wantErr: nil,
		},
		{
			name:    "Canceled is context.Canceled",
			s:       grpcStatus.New(codes.Canceled, ""),
			wantErr: context.Canceled,
		},
		{
			name:    "invalid argument missing details is Internal error",
			s:       grpcStatus.New(codes.InvalidArgument, "error without details"),
			wantErr: Internal{Msg: "missing errorx type specifier. grpc InvalidArgument: error without details"},
		},
		{
			name: "invalid argument bad subtype is Internal error",
			s: func() *grpcStatus.Status {
				s := grpcStatus.New(codes.InvalidArgument, "error without details")
				s, _ = s.WithDetails(&errorxpb.ErrorDetails{
					Type: errorxpb.ErrorxType_REQUIRES_PROXY_REQUEST,
				})
				return s
			}(),
			wantErr: Internal{Msg: "grpc InvalidArgument: error without details"},
		},
		{
			name:    "notfound missing details is Internal error",
			s:       grpcStatus.New(codes.NotFound, "not a proxy request"),
			wantErr: Internal{Msg: "missing errorx type specifier. grpc NotFound: not a proxy request"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FromGRPCStatus(tc.s)
			if tc.wantErr == nil {
				require.Nil(t, got)
				return
			}
			gotErrx, okGot := got.(Error)
			wantErrx, okWant := tc.wantErr.(Error)
			require.Equal(t, okWant, okGot)
			if okGot {
				require.Equal(t, gotErrx.Message(), wantErrx.Message())
				if proxyRequest, ok := wantErrx.(RequiresProxyRequest); ok {
					require.Equal(t, got.(RequiresProxyRequest).Reason, proxyRequest.Reason)
				}
			} else {
				require.Equal(t, tc.wantErr, got)
			}
		})
	}
}
