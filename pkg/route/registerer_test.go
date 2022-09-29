package route

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gorilla/mux"
)

func TestMuxRegisterer(t *testing.T) {
	router := mux.NewRouter()
	reg := NewMuxRegisterer(router)

	reg.RegisterRoute("/test_path", nil, "GET")
	reg.RegisterRoutesWithPrefix("/test_prefix", nil, "POST")

	t.Run("register route match", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/test_path", http.NoBody)
		require.NoError(t, err)
		require.True(t, router.Match(req, &mux.RouteMatch{}))
	})

	t.Run("register route no match", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/test_path", http.NoBody)
		require.NoError(t, err)
		require.False(t, router.Match(req, &mux.RouteMatch{}))
	})

	t.Run("register route with prefix match", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/test_prefix_path", http.NoBody)
		require.NoError(t, err)
		require.True(t, router.Match(req, &mux.RouteMatch{}))
	})

	t.Run("register route with prefix no match", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/test_prefi", http.NoBody)
		require.NoError(t, err)
		require.False(t, router.Match(req, &mux.RouteMatch{}))
	})
}

func TestFuncRegisterer(t *testing.T) {
	var routeLog []string
	reg := NewFuncRegisterer(
		func(path string, _ http.Handler, methods ...string) {
			routeLog = append(routeLog, fmt.Sprintf("path: %s, methods: %+v", path, methods))
		},
		func(prefix string, handler http.Handler, methods ...string) {
			routeLog = append(routeLog, fmt.Sprintf("prefix: %s, methods: %+v", prefix, methods))
		})

	reg.RegisterRoute("/test_path", nil, "GET")
	reg.RegisterRoutesWithPrefix("/test_prefix", nil, "POST")

	require.Equal(t, []string{"path: /test_path, methods: [GET]", "prefix: /test_prefix, methods: [POST]"}, routeLog)
}
