// Package blockmultipart temporarily rejects multipart requests using headers only.
package blockmultipart

import (
	"context"
	"net/http"
	"strings"
)

// Config has no switches: attaching the middleware enables the control.
type Config struct{}

// CreateConfig returns the middleware configuration.
func CreateConfig() *Config { return &Config{} }

// BlockMultipart rejects multipart Content-Type values on every HTTP method.
type BlockMultipart struct {
	next http.Handler
}

// New constructs the middleware without reading any request body.
func New(_ context.Context, next http.Handler, _ *Config, _ string) (http.Handler, error) {
	return &BlockMultipart{next: next}, nil
}

// ServeHTTP checks every Content-Type header value, including duplicate headers.
func (m *BlockMultipart) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	for _, value := range req.Header.Values("Content-Type") {
		if strings.Contains(strings.ToLower(value), "multipart") {
			http.Error(rw, "multipart requests are temporarily disabled", http.StatusForbidden)
			return
		}
	}
	m.next.ServeHTTP(rw, req)
}
