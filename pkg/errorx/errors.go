package errorx

// NOTE: If you add a new error type to this file you must create a new
// type enum value in subquery.proto. You must also create a new round-trip
// test in errors_test.go.

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/grpc/codes"

	"github.com/grafana/mimir-graphite/v2/pkg/errorxpb"

	//nolint:staticcheck
	protov1 "github.com/golang/protobuf/proto"
	grpcStatus "google.golang.org/grpc/status"
)

type Error interface {
	error
	HTTPStatusCode() int
	Message() string
	GRPCStatus() *grpcStatus.Status
	GRPCStatusDetails() []protov1.Message
}

// FromGRPCStatus converts a Status to either context.Canceled or a native Error
// type. The GRPC Status type is ignored in this conversion -- instead we expect
// ErrorDetails to be included naming the correct internal type. Statuses
// without details will be returned as Internal errors.
func FromGRPCStatus(s *grpcStatus.Status) error { //nolint:gocyclo
	msg := fmt.Sprintf("grpc %v: %s", s.Code(), s.Message())
	if s.Code() == codes.OK {
		return nil
	} else if s.Code() == codes.Canceled {
		return context.Canceled
	}

	for _, di := range s.Details() {
		if d, ok := di.(*errorxpb.ErrorDetails); ok {
			switch d.Type {
			case errorxpb.ErrorxType_UNKNOWN:
				return Internal{Msg: "unknown errorx type specifier. " + msg}
			case errorxpb.ErrorxType_INTERNAL:
				return Internal{Msg: msg}
			case errorxpb.ErrorxType_BAD_REQUEST:
				return BadRequest{Msg: msg}
			case errorxpb.ErrorxType_REQUIRES_PROXY_REQUEST:
				return RequiresProxyRequest{Msg: msg, Reason: d.Reason}
			case errorxpb.ErrorxType_RATE_LIMITED:
				return TooManyRequests{Msg: msg}
			case errorxpb.ErrorxType_DISABLED:
				return Disabled{}
			case errorxpb.ErrorxType_UNIMPLEMENTED:
				return Unimplemented{Msg: msg}
			case errorxpb.ErrorxType_UNPROCESSABLE_ENTITY:
				return UnprocessableEntity{Msg: msg}
			case errorxpb.ErrorxType_CONFLICT:
				return Conflict{Msg: msg}
			case errorxpb.ErrorxType_TOO_MANY_REQUESTS:
				return TooManyRequests{Msg: msg}
			case errorxpb.ErrorxType_UNSUPPORTED_MEDIA_TYPE:
				return UnsupportedMediaType{Msg: msg}
			case errorxpb.ErrorxType_REQUEST_TIMEOUT:
				return RequestTimeout{Msg: msg}
			default:
				return Internal{Msg: "invalid errorx type specifier. " + msg}
			}
		}
	}
	return Internal{Msg: "missing errorx type specifier. " + msg}
}

func WithErrorxTypeDetail(s *grpcStatus.Status, details ...protov1.Message) *grpcStatus.Status {
	var err error

	if s, err = s.WithDetails(details...); err != nil {
		// This Should Not Happen, but let's not panic just in case.
		return grpcStatus.New(codes.Internal, err.Error())
	}
	return s
}

// ErrorAsGRPCStatus generates a new Status from the given generic error. If
// there is a GRPCStatus-compatible error wrapped inside, that type and details
// are used to generate the new status. If there is a problem encoding the
// details, an Internal error is thrown. The message is retrieved from the
// outermost error. Non-errorx errors are given the Unknown error type.
func ErrorAsGRPCStatus(err error) *grpcStatus.Status {
	var errx Error
	if errors.As(err, &errx) {
		s := grpcStatus.New(errx.GRPCStatus().Code(), err.Error())
		var detailsErr error

		details := errx.GRPCStatusDetails()
		s, detailsErr = s.WithDetails(details...)
		if detailsErr != nil {
			return grpcStatus.New(codes.Internal, fmt.Sprintf("problem encoding Details of underlying errorx.Error: %v", detailsErr))
		}
		return s
	}
	return grpcStatus.New(codes.Unknown, err.Error())
}

var _ Error = Internal{}

type Internal struct {
	Msg string
	Err error
}

func (e Internal) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s", e.Msg, e.Err)
	}
	return e.Msg
}

func (e Internal) Message() string {
	return e.Msg
}

func (e Internal) Unwrap() error {
	return e.Err
}

func (e Internal) HTTPStatusCode() int {
	return http.StatusInternalServerError
}

func (e Internal) GRPCStatus() *grpcStatus.Status {
	return WithErrorxTypeDetail(grpcStatus.New(codes.Internal, e.Error()), e.GRPCStatusDetails()...)
}

func (e Internal) GRPCStatusDetails() []protov1.Message {
	return []protov1.Message{&errorxpb.ErrorDetails{
		Type: errorxpb.ErrorxType_INTERNAL,
	}}
}

var _ Error = BadRequest{}

type BadRequest struct {
	Msg string
	Err error
}

func (e BadRequest) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s", e.Msg, e.Err)
	}
	return e.Msg
}

func (e BadRequest) Message() string {
	return e.Msg
}

func (e BadRequest) Unwrap() error {
	return e.Err
}

func (e BadRequest) HTTPStatusCode() int {
	return http.StatusBadRequest
}

func (e BadRequest) GRPCStatus() *grpcStatus.Status {
	return WithErrorxTypeDetail(grpcStatus.New(codes.InvalidArgument, e.Error()), e.GRPCStatusDetails()...)
}

func (e BadRequest) GRPCStatusDetails() []protov1.Message {
	return []protov1.Message{&errorxpb.ErrorDetails{
		Type: errorxpb.ErrorxType_BAD_REQUEST,
	}}
}

var _ Error = RequiresProxyRequest{}

// RequiresProxyRequest signifies the request could not be completed locally (eg. unsupported target function), so should be forwarded to the appropriate proxy.
type RequiresProxyRequest struct {
	Msg string
	Err error
	// Reason field should be a low cardinality value, used for labeling metrics.
	Reason string
}

func (e RequiresProxyRequest) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s", e.Msg, e.Err)
	}
	return e.Msg
}

func (e RequiresProxyRequest) Message() string {
	return e.Msg
}

func (e RequiresProxyRequest) Unwrap() error {
	return e.Err
}

func (e RequiresProxyRequest) HTTPStatusCode() int {
	return http.StatusBadRequest
}

func (e RequiresProxyRequest) GRPCStatus() *grpcStatus.Status {
	return WithErrorxTypeDetail(grpcStatus.New(codes.NotFound, e.Error()), e.GRPCStatusDetails()...)
}

func (e RequiresProxyRequest) GRPCStatusDetails() []protov1.Message {
	return []protov1.Message{&errorxpb.ErrorDetails{
		Type:   errorxpb.ErrorxType_REQUIRES_PROXY_REQUEST,
		Reason: e.Reason,
	}}
}

var _ Error = Disabled{}

type Disabled struct{}

func (e Disabled) Message() string {
	return "feature disabled"
}

func (e Disabled) Error() string {
	return "disabled"
}

func (e Disabled) HTTPStatusCode() int {
	return http.StatusNotImplemented
}

func (e Disabled) GRPCStatus() *grpcStatus.Status {
	return WithErrorxTypeDetail(grpcStatus.New(codes.Unavailable, e.Error()), e.GRPCStatusDetails()...)
}

func (e Disabled) GRPCStatusDetails() []protov1.Message {
	return []protov1.Message{&errorxpb.ErrorDetails{
		Type: errorxpb.ErrorxType_DISABLED,
	}}
}

var _ Error = Unimplemented{}

type Unimplemented struct {
	Msg string
}

func (e Unimplemented) Error() string {
	return e.Msg
}

func (e Unimplemented) Message() string {
	return e.Msg
}

func (e Unimplemented) HTTPStatusCode() int {
	return http.StatusNotImplemented
}

func (e Unimplemented) GRPCStatus() *grpcStatus.Status {
	return WithErrorxTypeDetail(grpcStatus.New(codes.Unimplemented, e.Error()), e.GRPCStatusDetails()...)
}

func (e Unimplemented) GRPCStatusDetails() []protov1.Message {
	return []protov1.Message{&errorxpb.ErrorDetails{
		Type: errorxpb.ErrorxType_UNIMPLEMENTED,
	}}
}

var _ Error = UnprocessableEntity{}

type UnprocessableEntity struct {
	Msg string
}

func (e UnprocessableEntity) Error() string {
	return e.Msg
}

func (e UnprocessableEntity) Message() string {
	return e.Msg
}

func (e UnprocessableEntity) HTTPStatusCode() int {
	return http.StatusUnprocessableEntity
}

func (e UnprocessableEntity) GRPCStatus() *grpcStatus.Status {
	return WithErrorxTypeDetail(grpcStatus.New(codes.InvalidArgument, e.Error()), e.GRPCStatusDetails()...)
}

func (e UnprocessableEntity) GRPCStatusDetails() []protov1.Message {
	return []protov1.Message{&errorxpb.ErrorDetails{
		Type: errorxpb.ErrorxType_UNPROCESSABLE_ENTITY,
	}}
}

var _ Error = Conflict{}

type Conflict struct {
	Msg string
	Err error
}

func (e Conflict) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s", e.Msg, e.Err)
	}
	return e.Msg
}

func (e Conflict) Message() string {
	return e.Msg
}

func (e Conflict) Unwrap() error {
	return e.Err
}

func (e Conflict) HTTPStatusCode() int {
	return http.StatusConflict
}

func (e Conflict) GRPCStatus() *grpcStatus.Status {
	return WithErrorxTypeDetail(grpcStatus.New(codes.Aborted, e.Error()), e.GRPCStatusDetails()...)
}

func (e Conflict) GRPCStatusDetails() []protov1.Message {
	return []protov1.Message{&errorxpb.ErrorDetails{
		Type: errorxpb.ErrorxType_CONFLICT,
	}}
}

var _ Error = UnsupportedMediaType{}

type UnsupportedMediaType struct {
	Msg string
	Err error
}

func (e UnsupportedMediaType) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s", e.Msg, e.Err)
	}
	return e.Msg
}

func (e UnsupportedMediaType) Message() string {
	return e.Msg
}

func (e UnsupportedMediaType) Unwrap() error {
	return e.Err
}

func (e UnsupportedMediaType) HTTPStatusCode() int {
	return http.StatusUnsupportedMediaType
}

func (e UnsupportedMediaType) GRPCStatus() *grpcStatus.Status {
	return WithErrorxTypeDetail(grpcStatus.New(codes.Unimplemented, e.Error()), e.GRPCStatusDetails()...)
}

func (e UnsupportedMediaType) GRPCStatusDetails() []protov1.Message {
	return []protov1.Message{&errorxpb.ErrorDetails{
		Type: errorxpb.ErrorxType_UNSUPPORTED_MEDIA_TYPE,
	}}
}

var _ Error = TooManyRequests{}

type TooManyRequests struct {
	Msg string
	Err error
}

func (e TooManyRequests) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s", e.Msg, e.Err)
	}
	return e.Msg
}

func (e TooManyRequests) Message() string {
	return e.Msg
}

func (e TooManyRequests) Unwrap() error {
	return e.Err
}

func (e TooManyRequests) HTTPStatusCode() int {
	return http.StatusTooManyRequests
}

func (e TooManyRequests) GRPCStatus() *grpcStatus.Status {
	return WithErrorxTypeDetail(grpcStatus.New(codes.ResourceExhausted, e.Error()), e.GRPCStatusDetails()...)
}

func (e TooManyRequests) GRPCStatusDetails() []protov1.Message {
	return []protov1.Message{&errorxpb.ErrorDetails{
		Type: errorxpb.ErrorxType_TOO_MANY_REQUESTS,
	}}
}

var _ Error = RequestTimeout{}

type RequestTimeout struct {
	Msg string
	Err error
}

func (e RequestTimeout) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s", e.Msg, e.Err)
	}
	return e.Msg
}

func (e RequestTimeout) Message() string {
	return e.Msg
}

func (e RequestTimeout) Unwrap() error {
	return e.Err
}

func (e RequestTimeout) HTTPStatusCode() int {
	return http.StatusRequestTimeout
}

func (e RequestTimeout) GRPCStatus() *grpcStatus.Status {
	return WithErrorxTypeDetail(grpcStatus.New(codes.DeadlineExceeded, e.Error()), e.GRPCStatusDetails()...)
}

func (e RequestTimeout) GRPCStatusDetails() []protov1.Message {
	return []protov1.Message{&errorxpb.ErrorDetails{
		Type: errorxpb.ErrorxType_REQUEST_TIMEOUT,
	}}
}

func TryUnwrap(err error) error {
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return wrapped.Unwrap()
	}
	return err
}
