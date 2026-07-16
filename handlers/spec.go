package handlers

import (
	"net/http"
	"os"
)

// SpecHandler exposes the raw OpenAPI spec as YAML.
type SpecHandler struct{}

func (h *SpecHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("docs/openapi.yaml")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/yaml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
