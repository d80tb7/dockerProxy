package chart

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"go.uber.org/zap"
)

type Handler struct {
	cache     *cache.Cache
	logger    *zap.SugaredLogger
	transport http.RoundTripper
}

func NewHandler(cache *cache.Cache, logger *zap.SugaredLogger) http.Handler {
	return &Handler{
		cache:     cache,
		logger:    logger,
		transport: http.DefaultTransport,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// Create a unique key for this request, based on the host, path, and authorization
	cacheKey := r.Host + r.URL.Path + r.Header.Get("Authorization")

	// Check if the request is for a manifest and we have a cached response
	if strings.Contains(r.URL.Path, "/manifests/") {
		if manifestData, found := h.cache.Get(cacheKey); found {
			if _, err := w.Write(manifestData.([]byte)); err != nil {
				h.logger.Errorf("Failed to write response: %v", err)
				return
			}
			return
		}
	}

	if r.Host == "" {
		http.Error(w, "Missing host header", http.StatusBadRequest)
		return
	}

	// Create the target URL for the proxy
	targetUrl, err := url.Parse("https://" + r.Host + "/v2")
	if err != nil {
		http.Error(w, "Failed to parse target URL: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create a new reverse proxy for the target URL
	proxy := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			// Update the request before proxying
			r.URL.Host = targetUrl.Host
			r.URL.Scheme = targetUrl.Scheme
			r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
			r.Host = targetUrl.Host
		},
		Transport: h.transport,
		ModifyResponse: func(resp *http.Response) error {
			// Check if the request path matches the manifests pattern
			if strings.Contains(r.URL.Path, "/manifests/") {
				// Read the response body (manifest data)
				manifestData, err := io.ReadAll(resp.Body)
				if err != nil {
					return err
				}

				// Replace the response body as it was consumed
				resp.Body = io.NopCloser(bytes.NewBuffer(manifestData))

				// Cache the manifest data
				h.cache.Set(cacheKey, manifestData, cache.DefaultExpiration)
			}
			return nil
		},
	}

	// Proxy the request
	proxy.ServeHTTP(w, r)
}
