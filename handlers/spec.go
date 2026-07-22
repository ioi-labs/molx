package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
)

// SpecHandler exposes the OpenAPI spec as YAML or JSON depending on the path.
type SpecHandler struct{}

func (h *SpecHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("docs/openapi.yaml")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	wantJSON := strings.HasSuffix(r.URL.Path, ".json")
	if !wantJSON {
		w.Header().Set("Content-Type", "text/yaml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}

	// ponytail: convert YAML to JSON on the fly instead of maintaining two spec files.
	var spec any
	if err := yaml.Unmarshal(data, &spec); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(spec)
}
