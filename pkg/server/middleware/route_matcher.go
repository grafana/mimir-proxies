package middleware

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/grafana/mimir-proxies/pkg/util/bytereplacer"

	"github.com/gorilla/mux"
)

// RouteMatcher matches routes
type RouteMatcher interface {
	Match(*http.Request, *mux.RouteMatch) bool
}

func getRouteName(routeMatcher RouteMatcher, r *http.Request) string {
	var routeMatch mux.RouteMatch
	if routeMatcher == nil || !routeMatcher.Match(r, &routeMatch) {
		return ""
	}

	if routeMatch.MatchErr == mux.ErrNotFound {
		return "notfound"
	}

	if routeMatch.Route == nil {
		return ""
	}

	if name := routeMatch.Route.GetName(); name != "" {
		return name
	}

	tmpl, err := routeMatch.Route.GetPathTemplate()
	if err == nil {
		return MakeLabelValue(tmpl)
	}

	return ""
}

var invalidCharsReplacer = bytereplacer.New(regexp.MustCompile(`[^a-zA-Z0-9]`), '_')

// MakeLabelValue converts a Gorilla mux path to a string suitable for use in
// a Prometheus label value.
func MakeLabelValue(path string) string {
	// Convert non-alnums to underscores.
	result := invalidCharsReplacer.Replace(path)

	// Trim leading and trailing underscores.
	result = strings.Trim(result, "_")

	// Make it all lowercase
	result = strings.ToLower(result)

	// Special case.
	if result == "" {
		result = "root"
	}
	return result
}
