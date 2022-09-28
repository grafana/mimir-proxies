package errorx

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
)

func TestLogAndSetHttpError(t *testing.T) {
	for _, tc := range map[string]struct {
		err             error
		expectedCode    int
		expectedMessage string
	}{
		"canceled": {
			err:             context.Canceled,
			expectedCode:    httpStatusCanceled,
			expectedMessage: "request canceled",
		},
		"unknown": {
			err:             errors.New("unknown error with a secret that shouldn't be logged"),
			expectedCode:    http.StatusInternalServerError,
			expectedMessage: "unknown error",
		},
		"bad request with wrapped error": {
			err:             BadRequest{Msg: "some message", Err: errors.New("this shouldn't be logged")},
			expectedCode:    http.StatusBadRequest,
			expectedMessage: "some message",
		},
		"disabled": {
			err:             Disabled{},
			expectedCode:    http.StatusNotImplemented,
			expectedMessage: "feature disabled",
		},
		"unimplemented": {
			err:          Unimplemented{},
			expectedCode: http.StatusNotImplemented,
		},
		"badrequest": {
			err:          BadRequest{},
			expectedCode: http.StatusBadRequest,
		},
		"badrequest.wrapping.internal": {
			err:          BadRequest{Err: Internal{}},
			expectedCode: http.StatusBadRequest,
		},
		"ratelimited": {
			err:          RateLimited{},
			expectedCode: http.StatusTooManyRequests,
		},
		"internal": {
			err:          Internal{},
			expectedCode: http.StatusInternalServerError,
		},
		"internal.wrapping.bad.mrequest": {
			err:          Internal{Err: BadRequest{}},
			expectedCode: http.StatusInternalServerError,
		},
		"unmapped": {
			err:             context.DeadlineExceeded,
			expectedCode:    http.StatusInternalServerError,
			expectedMessage: "unknown error",
		},
	} {
		logger := log.NewNopLogger()
		recorder := httptest.NewRecorder()

		LogAndSetHTTPError(context.TODO(), recorder, logger, tc.err)

		assert.Equal(t, tc.expectedCode, recorder.Code)
		retrievedBody, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(t, tc.expectedMessage+"\n", string(retrievedBody))
	}
}
