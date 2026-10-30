package routes

import (
	"context"
	"net/http"
)

func New(dbPing, cachePing func(context.Context) error) http.Handler {
	router := http.NewServeMux()
	router.HandleFunc("GET /health", health(dbPing, cachePing))
	return router
}
