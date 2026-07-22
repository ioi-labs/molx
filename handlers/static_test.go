package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReferenceHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/reference", nil)
	req.Host = "localhost:8080"
	rr := httptest.NewRecorder()

	Reference(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Scalar.createApiReference") {
		t.Errorf("body missing scalar script")
	}
	if !strings.Contains(body, "http://localhost:8080") {
		t.Errorf("body missing server url")
	}
}

func TestScalarJSHandler(t *testing.T) {
	dir := t.TempDir()
	depsDir := filepath.Join(dir, "deps")
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		t.Fatalf("mkdir deps: %v", err)
	}
	jsPath := filepath.Join(depsDir, "scalar-standalone.js")
	if err := os.WriteFile(jsPath, []byte("console.log(1);"), 0644); err != nil {
		t.Fatalf("write js: %v", err)
	}

	// ponytail: temporarily change working dir so the relative read succeeds.
	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	req := httptest.NewRequest(http.MethodGet, "/scalar-standalone.js", nil)
	rr := httptest.NewRecorder()

	ScalarJS(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Header().Get("Content-Type") != "application/javascript; charset=utf-8" {
		t.Errorf("content-type = %q", rr.Header().Get("Content-Type"))
	}
	if rr.Body.String() != "console.log(1);" {
		t.Errorf("body = %q", rr.Body.String())
	}
}

func TestSpecHandler(t *testing.T) {
	dir := t.TempDir()
	docsPath := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docsPath, 0755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docsPath, "openapi.yaml"), []byte("openapi: 3.0.0"), 0644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	// ponytail: temporarily change working dir so the relative read succeeds.
	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	h := &SpecHandler{}

	for _, path := range []string{"/openapi.yaml", "/openapi.json"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("%s status = %d, want %d", path, rr.Code, http.StatusOK)
		}
		if path == "/openapi.json" {
			if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("%s content-type = %q, want application/json", path, ct)
			}
			if !strings.Contains(rr.Body.String(), `"openapi"`) {
				t.Errorf("%s body missing openapi key", path)
			}
			continue
		}
		if !strings.Contains(rr.Body.String(), "openapi: 3.0.0") {
			t.Errorf("%s body = %q", path, rr.Body.String())
		}
	}
}

func TestSpecHandler_MissingFile(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	h := &SpecHandler{}
	req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}
