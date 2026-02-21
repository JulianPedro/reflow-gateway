package docs

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var OpenAPISpec []byte

const scalarHTML = `<!DOCTYPE html>
<html>
<head>
  <title>Reflow Gateway - API Reference</title>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
</head>
<body>
  <script id="api-reference" data-url="/docs/openapi.yaml"></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`

// Handler returns an http.Handler that serves the Scalar API reference UI
// and the raw OpenAPI spec.
//
//	GET /docs            → Scalar UI
//	GET /docs/openapi.yaml → raw YAML spec
func Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(scalarHTML))
	})

	mux.HandleFunc("/docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(OpenAPISpec)
	})

	return mux
}
