package middleware

import (
	"net/http"

	"github.com/go-kit/log"

	"github.com/weaveworks/common/user"
)

type HTTPAuth struct {
	log log.Logger
}

func NewHTTPAuth(log log.Logger) *HTTPAuth {
	return &HTTPAuth{
		log: log,
	}
}

func (h HTTPAuth) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ctx, err := user.ExtractOrgIDFromHTTPRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			logRequest(h.log, r, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
