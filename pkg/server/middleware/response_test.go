package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBadResponseLoggingWriter(t *testing.T) {
	for _, tc := range []struct {
		statusCode int
	}{
		{http.StatusOK},
		{http.StatusOK},
		{http.StatusUnprocessableEntity},
		{http.StatusInternalServerError},
	} {
		w := httptest.NewRecorder()
		wrapped := newResponseWriterSniffer(w)
		http.Error(wrapped, "", tc.statusCode)
		if wrapped.statusCode != tc.statusCode {
			t.Errorf("Wrong status code: have %d want %d", wrapped.statusCode, tc.statusCode)
		}
	}
}
