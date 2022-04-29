package appcommon

import (
	"net/http"

	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/weaveworks/common/user"

	opentracing "github.com/opentracing/opentracing-go"
)

// TracerTransport is a RoundTripper that records opentracing information
type TracerTransport struct {
	http.RoundTripper
	// name is the name of the Operation being performed for the span.
	name string
}

func (t *TracerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if parentSpan := opentracing.SpanFromContext(req.Context()); parentSpan != nil {
		var ht *nethttp.Tracer
		req, ht = nethttp.TraceRequest(
			parentSpan.Tracer(),
			req,
			nethttp.OperationName(t.name),
			nethttp.ClientTrace(false),
		)
		defer ht.Finish()
	}

	return t.RoundTripper.RoundTrip(req)
}

type AuthTransport struct {
	http.RoundTripper
}

// AuthTransport is a RoundTripper that injects the org id from the context into
// the http request.
func (t *AuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := user.InjectOrgIDIntoHTTPRequest(req.Context(), req); err != nil {
		return nil, err
	}
	return t.RoundTripper.RoundTrip(req)
}

// NewTracedAuthRoundTripper creates a RoundTripper that does both tracing
// and org ID injection.
func NewTracedAuthRoundTripper(rt http.RoundTripper, name string) http.RoundTripper {
	return &TracerTransport{
		RoundTripper: &AuthTransport{
			RoundTripper: &nethttp.Transport{
				RoundTripper: rt,
			},
		},
		name: name,
	}
}
