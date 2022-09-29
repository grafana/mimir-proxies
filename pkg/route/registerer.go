package route

import (
	"net/http"

	"github.com/gorilla/mux"
)

// Registerer provides a set of methods for registering API routes. An interface is defined as route registration
// differs depending on where the proxy is running.
type Registerer interface {
	RegisterRoute(path string, handler http.Handler, methods ...string)
	RegisterRoutesWithPrefix(prefix string, handler http.Handler, methods ...string)
}

// MuxRegisterer wraps around a mux router. This is used for proxies deployed in Grafana Cloud.
type MuxRegisterer struct {
	router *mux.Router
}

func NewMuxRegisterer(router *mux.Router) *MuxRegisterer {
	return &MuxRegisterer{router: router}
}

func (r *MuxRegisterer) RegisterRoute(path string, handler http.Handler, methods ...string) {
	r.router.Path(path).Handler(handler).Methods(methods...)
}

func (r *MuxRegisterer) RegisterRoutesWithPrefix(prefix string, handler http.Handler, methods ...string) {
	r.router.NewRoute().PathPrefix(prefix).Handler(handler).Methods(methods...)
}

// FuncRegisterer allows a registerer to be defined by passing in functions. This is used for proxies.
type FuncRegisterer struct {
	route            RegisterRouteFunc
	routesWithPrefix RegisterRoutesWithPrefixFunc
}

type RegisterRouteFunc func(path string, handler http.Handler, methods ...string)
type RegisterRoutesWithPrefixFunc func(prefix string, handler http.Handler, methods ...string)

func NewFuncRegisterer(route RegisterRouteFunc, routesWithPrefix RegisterRoutesWithPrefixFunc) *FuncRegisterer {
	return &FuncRegisterer{
		route:            route,
		routesWithPrefix: routesWithPrefix,
	}
}

func (r FuncRegisterer) RegisterRoute(path string, handler http.Handler, methods ...string) {
	r.route(path, handler, methods...)
}

func (r FuncRegisterer) RegisterRoutesWithPrefix(path string, handler http.Handler, methods ...string) {
	r.routesWithPrefix(path, handler, methods...)
}
