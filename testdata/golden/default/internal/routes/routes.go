package routes

import (
	"net/http"
)

func New() http.Handler {
	router := http.NewServeMux()
	router.HandleFunc("GET /health", health())
	return router
}
