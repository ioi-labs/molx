package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"molx/config"
)

func TestHealthHandler_OK(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "obscura")
	if err := os.WriteFile(bin, []byte("x"), 0755); err != nil {
		t.Fatalf("create fake binary: %v", err)
	}

	cfg := &config.Config{ObscuraBinaryPath: bin, Timeout: 60 * time.Second}
	h := &HealthHandler{Config: cfg}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "ok") {
		t.Errorf("body = %q, want ok", rr.Body.String())
	}
}

func TestHealthHandler_MissingBinary(t *testing.T) {
	cfg := &config.Config{ObscuraBinaryPath: "/nonexistent/obscura", Timeout: 60 * time.Second}
	h := &HealthHandler{Config: cfg}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "missing binary") {
		t.Errorf("body = %q, want missing binary", rr.Body.String())
	}
}
