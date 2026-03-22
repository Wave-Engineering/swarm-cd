package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// newTestRouter creates a minimal Gin engine with the auth middleware applied
// to a dummy write endpoint (POST /write) and an unprotected read endpoint
// (GET /read). This isolates the middleware behaviour from the rest of the app.
func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Read endpoint — no auth
	r.GET("/read", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Write endpoint — behind auth middleware
	write := r.Group("/")
	write.Use(authMiddleware())
	write.POST("/write", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	return r
}

// helper to decode a JSON response body into a map
func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	return body
}

// ---------- Auth behaviour tests ----------

func TestAuth_NoTokenConfigured_Returns403(t *testing.T) {
	// Ensure no token is configured
	origToken := apiToken
	apiToken = ""
	defer func() { apiToken = origToken }()

	r := newTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/write", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}

	body := decodeBody(t, w)
	expectedMsg := "mutation API is disabled — set SWARMCD_API_TOKEN to enable"
	if body["error"] != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, body["error"])
	}
}

func TestAuth_WrongToken_Returns401(t *testing.T) {
	origToken := apiToken
	apiToken = "correct-secret"
	defer func() { apiToken = origToken }()

	r := newTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/write", nil)
	req.Header.Set("Authorization", "Bearer wrong-secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}

	body := decodeBody(t, w)
	expectedMsg := "invalid or missing authorization token"
	if body["error"] != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, body["error"])
	}
}

func TestAuth_MissingHeader_Returns401(t *testing.T) {
	origToken := apiToken
	apiToken = "correct-secret"
	defer func() { apiToken = origToken }()

	r := newTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/write", nil)
	// No Authorization header
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}

	body := decodeBody(t, w)
	expectedMsg := "invalid or missing authorization token"
	if body["error"] != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, body["error"])
	}
}

func TestAuth_ValidToken_PassesThrough(t *testing.T) {
	origToken := apiToken
	apiToken = "correct-secret"
	defer func() { apiToken = origToken }()

	r := newTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/write", nil)
	req.Header.Set("Authorization", "Bearer correct-secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["ok"] != true {
		t.Errorf("expected ok=true, got %v", body["ok"])
	}
}

func TestAuth_ReadEndpoints_NoAuthRequired(t *testing.T) {
	// Even with no token configured, read endpoints must work
	origToken := apiToken
	apiToken = ""
	defer func() { apiToken = origToken }()

	r := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/read", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Also verify with a configured token but no header — should still pass
	apiToken = "some-secret"
	r2 := newTestRouter()
	req2 := httptest.NewRequest(http.MethodGet, "/read", nil)
	w2 := httptest.NewRecorder()
	r2.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected status 200 with token configured, got %d", w2.Code)
	}
}

// ---------- Health endpoint test ----------

func TestGetHealth_ReturnsMutationApiEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Case 1: token not set → mutation_api_enabled = false
	origToken := apiToken
	apiToken = ""
	defer func() { apiToken = origToken }()

	r := gin.New()
	r.GET("/health", getHealth)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["mutation_api_enabled"] != false {
		t.Errorf("expected mutation_api_enabled=false when token not set, got %v", body["mutation_api_enabled"])
	}
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body["status"])
	}

	// Case 2: token set → mutation_api_enabled = true
	apiToken = "some-secret"

	req2 := httptest.NewRequest(http.MethodGet, "/health", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w2.Code)
	}

	body2 := decodeBody(t, w2)
	if body2["mutation_api_enabled"] != true {
		t.Errorf("expected mutation_api_enabled=true when token set, got %v", body2["mutation_api_enabled"])
	}
}
