package httpserver

import (
	"net/http"

	"nusantara/internal/httpserver/handlers"
	"nusantara/internal/platform/oscheck"
)

func NewRouter(osResult oscheck.Result, api *API) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", handlers.Health)
	mux.Handle("GET /v1/system/compatibility", handlers.SystemCompatibility(osResult))
	api.RegisterRoutes(mux)

	return withJSONContentType(mux)
}

func withJSONContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}

