package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/gin-gonic/gin"
	"github.com/m-adawi/swarm-cd/swarmcd"
)

// mockServiceAPI implements swarmcd.ServiceAPI for testing.
type mockServiceAPI struct {
	services   []swarm.Service
	listErr    error
	updateErr  error
	updateCalls []serviceUpdateCall
}

type serviceUpdateCall struct {
	ServiceID string
	Version   swarm.Version
	Spec      swarm.ServiceSpec
}

func (m *mockServiceAPI) ServiceList(_ context.Context, opts types.ServiceListOptions) ([]swarm.Service, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}

	// Extract stack namespace from label filter, if present
	const labelPrefix = "com.docker.stack.namespace="
	labelFilter := ""
	if opts.Filters.Len() > 0 {
		for _, l := range opts.Filters.Get("label") {
			if strings.HasPrefix(l, labelPrefix) {
				labelFilter = strings.TrimPrefix(l, labelPrefix)
			}
		}
	}

	if labelFilter == "" {
		return m.services, nil
	}

	var filtered []swarm.Service
	for _, svc := range m.services {
		if svc.Spec.Labels["com.docker.stack.namespace"] == labelFilter {
			filtered = append(filtered, svc)
		}
	}
	return filtered, nil
}

func (m *mockServiceAPI) ServiceUpdate(_ context.Context, serviceID string, version swarm.Version, service swarm.ServiceSpec, _ types.ServiceUpdateOptions) (swarm.ServiceUpdateResponse, error) {
	if m.updateErr != nil {
		return swarm.ServiceUpdateResponse{}, m.updateErr
	}
	m.updateCalls = append(m.updateCalls, serviceUpdateCall{
		ServiceID: serviceID,
		Version:   version,
		Spec:      service,
	})
	return swarm.ServiceUpdateResponse{}, nil
}

// setupTestState populates the swarmcd package's internal state for testing.
// Returns a cleanup function that restores the original state.
func setupTestState(stacks map[string]*swarmcd.StackStatus) func() {
	// Use the exported functions to check existing state, but we need
	// to set up state through the internal mechanism.
	// We'll use SetStackStatusForTest which we'll add.
	cleanup := swarmcd.SetStackStatusForTest(stacks)
	return cleanup
}

// newRestartTestRouter creates a Gin engine with restart routes + auth middleware.
func newRestartTestRouter(mock *mockServiceAPI) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Override the package-level dockerServiceAPI
	dockerServiceAPI = mock

	// Write endpoints behind auth
	write := r.Group("/")
	write.Use(authMiddleware())
	write.POST("/stacks/:name/restart", restartStack)
	write.POST("/stacks/:name/services/:service/restart", restartService)
	write.POST("/restart", restartAll)

	return r
}

// authedRequest creates an HTTP request with a valid Bearer token.
func authedRequest(method, path string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer test-secret")
	return req
}

// ---------- Test cases from the issue ----------

func TestRestartStack_NotFound(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	cleanup := setupTestState(map[string]*swarmcd.StackStatus{
		"alpha": {Revision: "aaa"},
	})
	defer cleanup()

	mock := &mockServiceAPI{}
	r := newRestartTestRouter(mock)

	req := authedRequest(http.MethodPost, "/stacks/nonexistent/restart")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["error"] != "stack 'nonexistent' not found" {
		t.Errorf("unexpected error: %v", body["error"])
	}
}

func TestRestartService_ResolvesPrefix(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	cleanup := setupTestState(map[string]*swarmcd.StackStatus{
		"blueshift": {Revision: "bbb"},
	})
	defer cleanup()

	mock := &mockServiceAPI{
		services: []swarm.Service{
			{
				ID: "svc-1",
				Meta: swarm.Meta{
					Version: swarm.Version{Index: 10},
				},
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{
						Name:   "blueshift_worker",
						Labels: map[string]string{"com.docker.stack.namespace": "blueshift"},
					},
					TaskTemplate: swarm.TaskSpec{ForceUpdate: 5},
				},
			},
			{
				ID: "svc-2",
				Meta: swarm.Meta{
					Version: swarm.Version{Index: 20},
				},
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{
						Name:   "blueshift_api",
						Labels: map[string]string{"com.docker.stack.namespace": "blueshift"},
					},
					TaskTemplate: swarm.TaskSpec{ForceUpdate: 3},
				},
			},
		},
	}

	r := newRestartTestRouter(mock)

	// Use short name "worker" — should resolve to "blueshift_worker"
	req := authedRequest(http.MethodPost, "/stacks/blueshift/services/worker/restart")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["message"] != "restart initiated for service 'blueshift_worker'" {
		t.Errorf("unexpected message: %v", body["message"])
	}

	// Verify the correct service was updated
	if len(mock.updateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(mock.updateCalls))
	}
	if mock.updateCalls[0].ServiceID != "svc-1" {
		t.Errorf("expected service ID 'svc-1', got %q", mock.updateCalls[0].ServiceID)
	}

	// Also verify the full name works (blueshift_worker)
	mock.updateCalls = nil
	req2 := authedRequest(http.MethodPost, "/stacks/blueshift/services/blueshift_worker/restart")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected status 200 with full name, got %d", w2.Code)
	}
	if len(mock.updateCalls) != 1 {
		t.Fatalf("expected 1 update call with full name, got %d", len(mock.updateCalls))
	}
}

func TestRestartStack_Success(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	cleanup := setupTestState(map[string]*swarmcd.StackStatus{
		"blueshift": {Revision: "bbb"},
	})
	defer cleanup()

	mock := &mockServiceAPI{
		services: []swarm.Service{
			{
				ID: "svc-1",
				Meta: swarm.Meta{
					Version: swarm.Version{Index: 10},
				},
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{
						Name:   "blueshift_web",
						Labels: map[string]string{"com.docker.stack.namespace": "blueshift"},
					},
					TaskTemplate: swarm.TaskSpec{ForceUpdate: 1},
				},
			},
			{
				ID: "svc-2",
				Meta: swarm.Meta{
					Version: swarm.Version{Index: 20},
				},
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{
						Name:   "blueshift_worker",
						Labels: map[string]string{"com.docker.stack.namespace": "blueshift"},
					},
					TaskTemplate: swarm.TaskSpec{ForceUpdate: 2},
				},
			},
			{
				ID: "svc-3",
				Meta: swarm.Meta{
					Version: swarm.Version{Index: 30},
				},
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{
						Name:   "blueshift_db",
						Labels: map[string]string{"com.docker.stack.namespace": "blueshift"},
					},
					TaskTemplate: swarm.TaskSpec{ForceUpdate: 0},
				},
			},
		},
	}

	r := newRestartTestRouter(mock)

	req := authedRequest(http.MethodPost, "/stacks/blueshift/restart")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["message"] != "restart initiated for stack 'blueshift'" {
		t.Errorf("unexpected message: %v", body["message"])
	}
	// services_restarted is a JSON number — decoded as float64
	if body["services_restarted"] != float64(3) {
		t.Errorf("expected 3 services_restarted, got %v", body["services_restarted"])
	}

	// Verify all 3 services were updated with ForceUpdate incremented
	if len(mock.updateCalls) != 3 {
		t.Fatalf("expected 3 update calls, got %d", len(mock.updateCalls))
	}

	expectedForceUpdates := map[string]uint64{
		"svc-1": 2, // was 1
		"svc-2": 3, // was 2
		"svc-3": 1, // was 0
	}
	for _, call := range mock.updateCalls {
		expected, ok := expectedForceUpdates[call.ServiceID]
		if !ok {
			t.Errorf("unexpected service ID: %s", call.ServiceID)
			continue
		}
		if call.Spec.TaskTemplate.ForceUpdate != expected {
			t.Errorf("service %s: expected ForceUpdate=%d, got %d", call.ServiceID, expected, call.Spec.TaskTemplate.ForceUpdate)
		}
	}
}

func TestRestartService_Success(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	cleanup := setupTestState(map[string]*swarmcd.StackStatus{
		"blueshift": {Revision: "bbb"},
	})
	defer cleanup()

	mock := &mockServiceAPI{
		services: []swarm.Service{
			{
				ID: "svc-1",
				Meta: swarm.Meta{
					Version: swarm.Version{Index: 10},
				},
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{
						Name:   "blueshift_worker",
						Labels: map[string]string{"com.docker.stack.namespace": "blueshift"},
					},
					TaskTemplate: swarm.TaskSpec{ForceUpdate: 7},
				},
			},
		},
	}

	r := newRestartTestRouter(mock)

	req := authedRequest(http.MethodPost, "/stacks/blueshift/services/blueshift_worker/restart")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["message"] != "restart initiated for service 'blueshift_worker'" {
		t.Errorf("unexpected message: %v", body["message"])
	}

	// Verify ForceUpdate was incremented
	if len(mock.updateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(mock.updateCalls))
	}
	if mock.updateCalls[0].Spec.TaskTemplate.ForceUpdate != 8 {
		t.Errorf("expected ForceUpdate=8, got %d", mock.updateCalls[0].Spec.TaskTemplate.ForceUpdate)
	}
	if mock.updateCalls[0].Version.Index != 10 {
		t.Errorf("expected version index 10, got %d", mock.updateCalls[0].Version.Index)
	}
}

func TestRestartAll_Success(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	cleanup := setupTestState(map[string]*swarmcd.StackStatus{
		"alpha": {Revision: "aaa"},
		"beta":  {Revision: "bbb"},
	})
	defer cleanup()

	mock := &mockServiceAPI{
		services: []swarm.Service{
			{
				ID: "svc-a1",
				Meta: swarm.Meta{
					Version: swarm.Version{Index: 1},
				},
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{
						Name:   "alpha_web",
						Labels: map[string]string{"com.docker.stack.namespace": "alpha"},
					},
					TaskTemplate: swarm.TaskSpec{ForceUpdate: 0},
				},
			},
			{
				ID: "svc-a2",
				Meta: swarm.Meta{
					Version: swarm.Version{Index: 2},
				},
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{
						Name:   "alpha_worker",
						Labels: map[string]string{"com.docker.stack.namespace": "alpha"},
					},
					TaskTemplate: swarm.TaskSpec{ForceUpdate: 0},
				},
			},
			{
				ID: "svc-b1",
				Meta: swarm.Meta{
					Version: swarm.Version{Index: 3},
				},
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{
						Name:   "beta_api",
						Labels: map[string]string{"com.docker.stack.namespace": "beta"},
					},
					TaskTemplate: swarm.TaskSpec{ForceUpdate: 0},
				},
			},
		},
	}

	r := newRestartTestRouter(mock)

	req := authedRequest(http.MethodPost, "/restart")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["message"] != "restart initiated for all stacks" {
		t.Errorf("unexpected message: %v", body["message"])
	}
	if body["stacks_restarted"] != float64(2) {
		t.Errorf("expected 2 stacks_restarted, got %v", body["stacks_restarted"])
	}
	if body["services_restarted"] != float64(3) {
		t.Errorf("expected 3 services_restarted, got %v", body["services_restarted"])
	}

	// Verify all services were updated
	if len(mock.updateCalls) != 3 {
		t.Fatalf("expected 3 update calls, got %d", len(mock.updateCalls))
	}

	// Each service should have ForceUpdate incremented to 1
	for _, call := range mock.updateCalls {
		if call.Spec.TaskTemplate.ForceUpdate != 1 {
			t.Errorf("service %s: expected ForceUpdate=1, got %d", call.ServiceID, call.Spec.TaskTemplate.ForceUpdate)
		}
	}
}

func TestRestartStack_NoAuth_Returns401(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	cleanup := setupTestState(map[string]*swarmcd.StackStatus{
		"alpha": {Revision: "aaa"},
	})
	defer cleanup()

	mock := &mockServiceAPI{}
	r := newRestartTestRouter(mock)

	// No Authorization header
	req := httptest.NewRequest(http.MethodPost, "/stacks/alpha/restart", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}

	body := decodeBody(t, w)
	if body["error"] != "invalid or missing authorization token" {
		t.Errorf("unexpected error: %v", body["error"])
	}

	// Verify no Docker API calls were made
	if len(mock.updateCalls) != 0 {
		t.Errorf("expected 0 update calls without auth, got %d", len(mock.updateCalls))
	}

	// Also verify the other restart endpoints require auth
	endpoints := []string{
		"/stacks/alpha/services/worker/restart",
		"/restart",
	}
	for _, ep := range endpoints {
		req := httptest.NewRequest(http.MethodPost, ep, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("endpoint %s: expected 401, got %d", ep, w.Code)
		}
	}
}

// TestRestartService_NotFound_Service verifies 404 when the service doesn't exist in the stack.
func TestRestartService_NotFound_Service(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	cleanup := setupTestState(map[string]*swarmcd.StackStatus{
		"blueshift": {Revision: "bbb"},
	})
	defer cleanup()

	mock := &mockServiceAPI{
		services: []swarm.Service{
			{
				ID: "svc-1",
				Meta: swarm.Meta{
					Version: swarm.Version{Index: 10},
				},
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{
						Name:   "blueshift_worker",
						Labels: map[string]string{"com.docker.stack.namespace": "blueshift"},
					},
					TaskTemplate: swarm.TaskSpec{ForceUpdate: 1},
				},
			},
		},
	}

	r := newRestartTestRouter(mock)

	req := authedRequest(http.MethodPost, "/stacks/blueshift/services/nonexistent/restart")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	body := decodeBody(t, w)
	expected := fmt.Sprintf("service '%s' not found in stack '%s'", "blueshift_nonexistent", "blueshift")
	if body["error"] != expected {
		t.Errorf("expected error %q, got %q", expected, body["error"])
	}
}
