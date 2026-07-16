package handlers

import (
	"encoding/json"
	"net/http"
	"os"

	"nexora-crawl/config"
	"nexora-crawl/models"
)

type HealthHandler struct {
	Config *config.Config
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	info := models.HealthResponse{Status: "ok"}
	if _, err := os.Stat(h.Config.ObscuraBinaryPath); err != nil {
		info.Status = "missing binary"
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}
