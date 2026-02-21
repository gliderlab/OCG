package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gliderlab/cogate/pkg/config"
)

func TestGatewayStruct(t *testing.T) {
	cfg := config.GatewayConfig{
		Port:        55003,
		Host:        "127.0.0.1",
		UIAuthToken: "",
	}
	gateway := New(cfg)
	
	if gateway == nil {
		t.Fatal("New() returned nil")
	}
	
	if gateway.cfg.Port != 55003 {
		t.Errorf("Expected port 55003, got %d", gateway.cfg.Port)
	}
}

func TestGatewayWithOptions(t *testing.T) {
	cfg := config.GatewayConfig{
		Port:        55003,
		Host:        "127.0.0.1",
		UIAuthToken: "",
	}
	gateway := New(cfg)
	
	// Test WithHTTPClient
	gateway = gateway.WithHTTPClient(&defaultHTTPClient{})
	
	// Test WithIDGenerator
	gateway = gateway.WithIDGenerator(&defaultIDGenerator{})
	
	// Test WithTimeProvider
	gateway = gateway.WithTimeProvider(&defaultTimeProvider{})
	
	// Test Config()
	gotCfg := gateway.Config()
	if gotCfg.Port != 55003 {
		t.Errorf("Expected port 55003, got %d", gotCfg.Port)
	}
}

func TestDefaultHTTPClient(t *testing.T) {
	client := &defaultHTTPClient{}
	
	// Test that it implements interface
	var _ HTTPClient = client
	
	// Get should return error (invalid URL)
	_, err := client.Get("http://invalid.url.that.does.not.exist")
	if err == nil {
		t.Error("Expected error for invalid URL")
	}
}

func TestDefaultIDGenerator(t *testing.T) {
	gen := &defaultIDGenerator{}
	
	// Test uniqueness
	id1 := gen.New()
	id2 := gen.New()
	
	if id1 == "" {
		t.Error("ID should not be empty")
	}
	
	if id1 == id2 {
		t.Error("IDs should be unique")
	}
}

func TestDefaultTimeProvider(t *testing.T) {
	tp := &defaultTimeProvider{}
	
	now := tp.Now()
	if now.IsZero() {
		t.Error("Now() should not return zero time")
	}
}

func TestWriteJSON(t *testing.T) {
	// Create a mock response writer
	rec := httptest.NewRecorder()
	
	// Test writing a map
	writeJSON(rec, map[string]string{"key": "value"})
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	
	// Test writing an error
	rec = httptest.NewRecorder()
	writeJSON(rec, map[string]string{"error": "test error"})
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestWriteJSONError(t *testing.T) {
	// Create a response writer that fails
	rec := httptest.NewRecorder()
	
	// Write invalid data (func cannot be JSON marshaled)
	writeJSON(rec, map[string]interface{}{"fn": func() {}})
	
	// Should still not panic, but may have issues
	_ = rec.Code
}

func TestValidateTokenEmpty(t *testing.T) {
	cfg := config.GatewayConfig{
		Port:        55003,
		UIAuthToken: "",
	}
	gateway := New(cfg)
	
	req := httptest.NewRequest("GET", "/test", nil)
	
	// With empty token configured, validation returns false (requires auth)
	// This is by design - empty token means auth is required
	if gateway.validateToken(req) {
		t.Error("Expected validation to fail with empty token config")
	}
}

func TestValidateTokenMismatch(t *testing.T) {
	cfg := config.GatewayConfig{
		Port:        55003,
		UIAuthToken: "secret-token",
	}
	gateway := New(cfg)
	
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	
	// With wrong token, should fail
	if gateway.validateToken(req) {
		t.Error("Expected validation to fail with wrong token")
	}
}

func TestValidateTokenMatch(t *testing.T) {
	cfg := config.GatewayConfig{
		Port:        55003,
		UIAuthToken: "secret-token",
	}
	gateway := New(cfg)
	
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	
	// With correct token, should pass
	if !gateway.validateToken(req) {
		t.Error("Expected validation to pass with correct token")
	}
}

func TestValidateTokenBearerCase(t *testing.T) {
	cfg := config.GatewayConfig{
		Port:        55003,
		UIAuthToken: "secret-token",
	}
	gateway := New(cfg)
	
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "BEARER secret-token")
	
	// Case insensitive bearer should work
	if !gateway.validateToken(req) {
		t.Error("Expected validation to pass with case-insensitive bearer")
	}
}

func TestValidateTokenQueryParam(t *testing.T) {
	cfg := config.GatewayConfig{
		Port:        55003,
		UIAuthToken: "secret-token",
	}
	gateway := New(cfg)
	
	// Test with token in query param
	req := httptest.NewRequest("GET", "/test?token=secret-token", nil)
	
	if !gateway.validateToken(req) {
		t.Error("Expected validation to pass with token in query")
	}
}

func TestRequireAuthMiddleware(t *testing.T) {
	cfg := config.GatewayConfig{
		Port:        55003,
		UIAuthToken: "secret-token",
	}
	gateway := New(cfg)
	
	// Create handler that requires auth
	handler := gateway.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("success"))
	})
	
	// Test without auth
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	handler.ServeHTTP(rec, req)
	
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", rec.Code)
	}
	
	// Test with auth
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	handler.ServeHTTP(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
	
	if rec.Body.String() != "success" {
		t.Error("Expected body 'success'")
	}
}

func TestAddCORS(t *testing.T) {
	cfg := config.GatewayConfig{
		Port: 55003,
	}
	gateway := New(cfg)
	
	handler := gateway.addCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	
	// Test OPTIONS request
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	handler.ServeHTTP(rec, req)
	
	// Should have CORS headers
	if rec.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("Expected CORS headers")
	}
	
	// Test normal request
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/test", nil)
	handler.ServeHTTP(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	cfg := config.GatewayConfig{
		Port: 55003,
	}
	gateway := New(cfg)
	
	// Create handler with rate limit
	handler := gateway.rateLimit(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	
	// First request should pass
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	handler.ServeHTTP(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
}
