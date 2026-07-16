package handlers

import (
	"net/http"
	"os"
)

// Reference serves a Scalar API reference page using the local OpenAPI spec.
func Reference(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	serverURL := scheme + "://" + r.Host

	html := `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Nexora Crawl API Reference</title>
  </head>
  <body>
    <div id="app"></div>
    <script src="/scalar-standalone.js" type="text/javascript"></script>
    <script type="text/javascript">
      Scalar.createApiReference('#app', {
        url: '/openapi.yaml',
        servers: [{ url: '` + serverURL + `', description: 'Current server' }]
      });
    </script>
  </body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}

// ScalarJS serves the Scalar standalone JavaScript bundle.
func ScalarJS(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("deps/scalar-standalone.js")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
