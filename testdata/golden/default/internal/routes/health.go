package routes

import (
	"context"
	"encoding/json"
	"net/http"
)

func health(dbPing, cachePing func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if (dbPing != nil && dbPing(r.Context()) != nil) || (cachePing != nil && cachePing(r.Context()) != nil) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
