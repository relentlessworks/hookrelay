package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/relentlessworks/hookrelay/internal/auth"
	"github.com/relentlessworks/hookrelay/internal/store"
)

func setupTestServer(t *testing.T) *Server {
	tmpFile, err := os.CreateTemp("", "hookrelay-test-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	s, err := store.New(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	a := auth.New("test-secret")
	return NewServer(s, a)
}

func TestHelp(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/help", nil)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "HookRelay") {
		t.Error("help text should contain 'HookRelay'")
	}
}

func TestHealth(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthFlow(t *testing.T) {
	srv := setupTestServer(t)

	form := url.Values{"email": {"agent@test.com"}, "workspace": {"ws_test"}}
	req := httptest.NewRequest("POST", "/auth/request", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "code=") {
		t.Fatalf("expected code in response, got: %s", w.Body.String())
	}

	codeStr := w.Body.String()
	idx := strings.Index(codeStr, "code=")
	if idx == -1 {
		t.Fatal("could not find code in response")
	}
	code := strings.TrimSpace(codeStr[idx+5:])

	form2 := url.Values{"email": {"agent@test.com"}, "code": {code}}
	req2 := httptest.NewRequest("POST", "/auth/verify", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "token=hr_") {
		t.Fatalf("expected token in response, got: %s", w2.Body.String())
	}
}

func TestCreateAndGetEndpoint(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	form := url.Values{"target_url": {"https://example.com/webhook"}, "description": {"test endpoint"}}
	req := httptest.NewRequest("POST", "/api/endpoints", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "handle=hook_") {
		t.Fatalf("expected handle in response, got: %s", w.Body.String())
	}

	body := w.Body.String()
	idx := strings.Index(body, "handle=")
	handle := strings.TrimSpace(body[idx+7:])
	if sp := strings.Index(handle, " "); sp != -1 {
		handle = handle[:sp]
	}

	req2 := httptest.NewRequest("GET", "/api/endpoints/"+handle, nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "https://example.com/webhook") {
		t.Errorf("expected target_url in response, got: %s", w2.Body.String())
	}
}

func TestListEndpoints(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	form := url.Values{"target_url": {"https://example.com/webhook"}}
	req := httptest.NewRequest("POST", "/api/endpoints", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	req2 := httptest.NewRequest("GET", "/api/endpoints", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "handle=hook_") {
		t.Errorf("expected endpoint in list, got: %s", w2.Body.String())
	}
}

func TestDeleteEndpoint(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	form := url.Values{"target_url": {"https://example.com/webhook"}}
	req := httptest.NewRequest("POST", "/api/endpoints", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	body := w.Body.String()
	idx := strings.Index(body, "handle=")
	handle := strings.TrimSpace(body[idx+7:])
	if sp := strings.Index(handle, " "); sp != -1 {
		handle = handle[:sp]
	}

	req2 := httptest.NewRequest("DELETE", "/api/endpoints/"+handle, nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "deleted") {
		t.Errorf("expected 'deleted' in response, got: %s", w2.Body.String())
	}
}

func TestWebhookForwarding(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	targetReceived := false
	var targetBody string
	targetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetReceived = true
		body := make([]byte, 1024)
		n, _ := r.Body.Read(body)
		targetBody = string(body[:n])
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer targetSrv.Close()

	form := url.Values{"target_url": {targetSrv.URL}}
	req := httptest.NewRequest("POST", "/api/endpoints", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	body := w.Body.String()
	idx := strings.Index(body, "handle=")
	handle := strings.TrimSpace(body[idx+7:])
	if sp := strings.Index(handle, " "); sp != -1 {
		handle = handle[:sp]
	}

	webhookBody := `{"event":"test","data":"hello"}`
	req2 := httptest.NewRequest("POST", "/hook/"+handle, strings.NewReader(webhookBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "handle=del_") {
		t.Errorf("expected delivery handle in response, got: %s", w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "status=200") {
		t.Errorf("expected status=200 in response, got: %s", w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "delivered=true") {
		t.Errorf("expected delivered=true in response, got: %s", w2.Body.String())
	}

	if !targetReceived {
		t.Error("target server should have received the webhook")
	}
	if targetBody != webhookBody {
		t.Errorf("target server received wrong body: got %s, want %s", targetBody, webhookBody)
	}

	deliveryBody := w2.Body.String()
	delIdx := strings.Index(deliveryBody, "handle=")
	deliveryHandle := strings.TrimSpace(deliveryBody[delIdx+7:])
	if sp := strings.Index(deliveryHandle, " "); sp != -1 {
		deliveryHandle = deliveryHandle[:sp]
	}

	req3 := httptest.NewRequest("GET", "/api/endpoints/"+handle+"/deliveries", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	w3 := httptest.NewRecorder()
	srv.Router(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w3.Code, w3.Body.String())
	}
	if !strings.Contains(w3.Body.String(), "handle=del_") {
		t.Errorf("expected delivery in list, got: %s", w3.Body.String())
	}

	req4 := httptest.NewRequest("GET", "/api/deliveries/"+deliveryHandle, nil)
	req4.Header.Set("Authorization", "Bearer "+token)
	w4 := httptest.NewRecorder()
	srv.Router(w4, req4)

	if w4.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w4.Code, w4.Body.String())
	}
	if !strings.Contains(w4.Body.String(), "delivered=true") {
		t.Errorf("expected delivered=true in delivery details, got: %s", w4.Body.String())
	}
}

func TestInvalidURL(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	form := url.Values{"target_url": {"not-a-url"}}
	req := httptest.NewRequest("POST", "/api/endpoints", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "hint:") {
		t.Errorf("expected hint in error response, got: %s", w.Body.String())
	}
}

func TestNoAuth(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/endpoints", nil)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "hint:") {
		t.Errorf("expected hint in error response, got: %s", w.Body.String())
	}
}

func TestJSONFormat(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	form := url.Values{"target_url": {"https://example.com/webhook"}}
	req := httptest.NewRequest("POST", "/api/endpoints", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
		t.Errorf("expected JSON content type, got: %s", w.Header().Get("Content-Type"))
	}
	if !strings.Contains(w.Body.String(), "\"handle\"") {
		t.Errorf("expected JSON with handle field, got: %s", w.Body.String())
	}
}

func TestWebhookNotFound(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("POST", "/hook/hook_nonexist", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "hint:") {
		t.Errorf("expected hint in error response, got: %s", w.Body.String())
	}
}

func TestWorkspaceInfo(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	form := url.Values{"target_url": {"https://example.com/webhook"}}
	req := httptest.NewRequest("POST", "/api/endpoints", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	req2 := httptest.NewRequest("GET", "/api/workspace", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "endpoints=1") {
		t.Errorf("expected endpoints=1 in response, got: %s", w2.Body.String())
	}
}

func getTestToken(t *testing.T, srv *Server) string {
	form := url.Values{"email": {"agent@test.com"}, "workspace": {"ws_test"}}
	req := httptest.NewRequest("POST", "/auth/request", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("auth/request failed: %d %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	idx := strings.Index(body, "code=")
	if idx == -1 {
		t.Fatalf("no code in response: %s", body)
	}
	code := strings.TrimSpace(body[idx+5:])

	form2 := url.Values{"email": {"agent@test.com"}, "code": {code}}
	req2 := httptest.NewRequest("POST", "/auth/verify", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("auth/verify failed: %d %s", w2.Code, w2.Body.String())
	}

	body2 := w2.Body.String()
	idx2 := strings.Index(body2, "token=")
	if idx2 == -1 {
		t.Fatalf("no token in response: %s", body2)
	}
	token := strings.TrimSpace(body2[idx2+6:])
	if sp := strings.Index(token, " "); sp != -1 {
		token = token[:sp]
	}

	return token
}
