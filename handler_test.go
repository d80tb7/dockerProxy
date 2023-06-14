package chart

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type mockRoundTripper struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
	callCount     int
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.roundTripFunc != nil {
		m.callCount++
		return m.roundTripFunc(req)
	}
	return nil, nil
}

func (m *mockRoundTripper) Reset() {
	m.callCount = 0
}

func TestHandler_ServeHTTP(t *testing.T) {

	// Define a mock RoundTripper to handle the requests
	mockTransport := &mockRoundTripper{}

	testCases := []struct {
		Name                       string
		Request                    *http.Request
		ExpectedStatusCode         int
		ExpectedResponseBody       string
		ExpectedRoundTripCallCount int
	}{
		{
			Name: "Request not served from cache the first time",
			Request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "https://example.com/manifests/test", nil)
				req.Host = "example.com"
				req.Header.Set("Authorization", "Bearer token")
				return req
			}(),
			ExpectedStatusCode:         http.StatusOK,
			ExpectedResponseBody:       "test manifest data",
			ExpectedRoundTripCallCount: 1,
		},
		{
			Name: "Request served from cache the second time",
			Request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "https://example.com/manifests/test", nil)
				req.Host = "example.com"
				req.Header.Set("Authorization", "Bearer token")
				return req
			}(),
			ExpectedStatusCode:         http.StatusOK,
			ExpectedResponseBody:       "test manifest data",
			ExpectedRoundTripCallCount: 0,
		},
		{
			Name: "Request denied the first time, cached deny returned the second time",
			Request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "https://example.com/manifests/denied", nil)
				req.Host = "example.com"
				req.Header.Set("Authorization", "Bearer denied")
				return req
			}(),
			ExpectedStatusCode:         http.StatusForbidden,
			ExpectedResponseBody:       "Access denied",
			ExpectedRoundTripCallCount: 1,
		},
		{
			Name: "Two different authorizations with the same path don't interfere",
			Request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "https://example.com/manifests/test", nil)
				req.Host = "example.com"
				req.Header.Set("Authorization", "Bearer token1")
				return req
			}(),
			ExpectedStatusCode:         http.StatusOK,
			ExpectedResponseBody:       "test manifest data",
			ExpectedRoundTripCallCount: 1,
		},
		{
			Name: "Same authorization with two different paths don't interfere",
			Request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "https://example.com/other", nil)
				req.Host = "example.com"
				req.Header.Set("Authorization", "Bearer token")
				return req
			}(),
			ExpectedStatusCode:         http.StatusNotFound,
			ExpectedResponseBody:       "Not Found",
			ExpectedRoundTripCallCount: 1,
		},
		{
			Name: "Same authorization and paths with two different hosts don't interfere",
			Request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "https://another.example.com/manifests/test", nil)
				req.Host = "another.example.com"
				req.Header.Set("Authorization", "Bearer token")
				return req
			}(),
			ExpectedStatusCode:         http.StatusOK,
			ExpectedResponseBody:       "test manifest data",
			ExpectedRoundTripCallCount: 1,
		},
	}

	// Create a new cache for testing
	testCache := cache.New(cache.NoExpiration, cache.NoExpiration)

	// Create a logger for testing
	testLogger := zap.NewNop().Sugar()

	// Create a new Handler instance with the custom RoundTripper
	handler := &Handler{
		cache:     testCache,
		logger:    testLogger,
		transport: mockTransport,
	}

	// Set the roundTripFunc for the mock RoundTripper
	mockTransport.roundTripFunc = func(req *http.Request) (*http.Response, error) {
		// Check if the request path matches the manifests pattern
		if req.URL.Path == "/manifests/test" {
			// Create a mock response
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("test manifest data")),
				Header:     make(http.Header),
			}
			return resp, nil
		} else if req.URL.Path == "/manifests/denied" {
			// Create a mock response for denied access
			resp := &http.Response{
				StatusCode: http.StatusForbidden,
				Body:       io.NopCloser(bytes.NewBufferString("Access denied")),
				Header:     make(http.Header),
			}
			return resp, nil
		}

		// Return 404 for other requests
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(bytes.NewBufferString("Not Found")),
			Header:     make(http.Header),
		}, nil
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {

			// Reset the mock RoundTripper before each test case
			mockTransport.Reset()

			// Create a test response recorder
			responseRecorder := httptest.NewRecorder()

			// Call the ServeHTTP method to handle the test request
			handler.ServeHTTP(responseRecorder, testCase.Request)

			// Check if the response status code is as expected
			assert.Equal(t, testCase.ExpectedStatusCode, responseRecorder.Code)

			// Check if the response body is as expected
			actualResponseBody, _ := io.ReadAll(responseRecorder.Body)
			assert.Equal(t, testCase.ExpectedResponseBody, string(actualResponseBody))

			// Check the number of times the RoundTrip function was called
			assert.Equal(t, testCase.ExpectedRoundTripCallCount, mockTransport.callCount)
		})
	}
}
