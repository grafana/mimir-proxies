package middleware

import (
	"net/http"

	"github.com/grafana/dskit/user"
)

type HTTPFakeAuth struct{}

func (h HTTPFakeAuth) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := user.InjectOrgID(r.Context(), "fake")
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
