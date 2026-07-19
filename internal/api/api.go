package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/relentlessworks/hookrelay/internal/auth"
	"github.com/relentlessworks/hookrelay/internal/models"
	"github.com/relentlessworks/hookrelay/internal/store"
)

// Server is the API server.
type Server struct {
	store *store.Store
	auth  *auth.AuthService
	http  *http.Client
}

// NewServer creates a new API server.
func NewServer(s *store.Store, a *auth.AuthService) *Server {
	return &Server{
		store: s,
		auth:  a,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Router is the main HTTP router.
func (s *Server) Router(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// --- Public routes ---

	if path == "/help" || path == "/.well-known/agent.md" {
		s.handleHelp(w, r)
		return
	}

	if path == "/health" {
		s.handleHealth(w, r)
		return
	}

	if path == "/auth/request" && r.Method == "POST" {
		s.handleRequestOTP(w, r)
		return
	}

	if path == "/auth/verify" && r.Method == "POST" {
		s.handleVerifyOTP(w, r)
		return
	}

	if strings.HasPrefix(path, "/hook/") {
		s.handleWebhook(w, r)
		return
	}

	// --- Authenticated routes ---

	if strings.HasPrefix(path, "/api/") {
		s.handleAPI(w, r)
		return
	}

	if path == "/" {
		s.handleHelp(w, r)
		return
	}

	s.errorResponse(w, r, http.StatusNotFound, "not found", "GET /help to see available endpoints")
}

// --- Helpers ---

func (s *Server) wantsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return true
	}
	if q := r.URL.Query().Get("format"); q == "json" {
		return true
	}
	return false
}

func (s *Server) writeResponse(w http.ResponseWriter, r *http.Request, status int, text string, data interface{}) {
	if s.wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(data)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintln(w, text)
}

func (s *Server) errorResponse(w http.ResponseWriter, r *http.Request, status int, msg, hint string) {
	if s.wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]string{"error": msg, "hint": hint})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, "error: %s\nhint: %s\n", msg, hint)
}

func (s *Server) getBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func (s *Server) authenticate(r *http.Request) (string, error) {
	token := s.getBearerToken(r)
	if token == "" {
		return "", fmt.Errorf("missing bearer token")
	}
	t, err := s.store.GetToken(token)
	if err != nil {
		return "", fmt.Errorf("invalid or expired token")
	}
	if time.Now().After(t.ExpiresAt) {
		return "", fmt.Errorf("token expired")
	}
	return t.Workspace, nil
}

func isValidURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func truncateBody(body string, max int) string {
	if len(body) > max {
		return body[:max] + "...(truncated)"
	}
	return body
}

// --- Handlers ---

func (s *Server) handleHelp(w http.ResponseWriter, r *http.Request) {
	help := `HookRelay — Agentic-First Webhook Relay & Inspection
=====================================================

HookRelay is a webhook relay and inspection service designed for AI agents.
Create endpoints, receive webhooks, forward them to your services, and inspect deliveries.
The API is the product. No UI, no SDK. Plain text by default, JSON on demand.

AUTHENTICATION
--------------
1. POST /auth/request   body: email=<email>&workspace=<handle>
   → Sends a 6-digit OTP code (returned in plain text for local dev).
2. POST /auth/verify     body: email=<email>&code=<code>
   → Returns a long-lived bearer token. Use it in Authorization: Bearer <token>.

CREATE AN ENDPOINT
------------------
POST /api/endpoints     body: target_url=<https://your-service.com/webhook>&description=<optional>
   → Returns: handle=hook_a1b2c target_url=https://your-service.com/webhook

LIST ENDPOINTS
--------------
GET /api/endpoints      → One endpoint per line: handle=hook_a1b2c target_url=https://... deliveries=3

GET AN ENDPOINT
---------------
GET /api/endpoints/<handle>  → handle=hook_a1b2c target_url=https://... deliveries=3

DELETE AN ENDPOINT
------------------
DELETE /api/endpoints/<handle>

LIST DELIVERIES FOR AN ENDPOINT
-------------------------------
GET /api/endpoints/<handle>/deliveries  → handle=del_x1y2z endpoint=hook_a1b2c status=200 delivered=true

GET A DELIVERY
--------------
GET /api/deliveries/<handle>  → handle=del_x1y2z endpoint=hook_a1b2c method=POST status=200 delivered=true

RECEIVE A WEBHOOK (PUBLIC)
--------------------------
POST /hook/<handle>     → Forwards the request body to the endpoint's target_url.
                           Returns: handle=del_x1y2z status=200 delivered=true
                           Any HTTP method is accepted (GET, POST, PUT, DELETE, PATCH).

WORKSPACE INFO
--------------
GET /api/workspace       → name=My Workspace plan=free endpoints=3

FORMATS
-------
- Plain text (default): one labeled, grepable line per record.
- JSON: add Accept: application/json or ?format=json to any request.

ERRORS
------
4xx responses include an "error" and a "hint" field to guide you.

EXAMPLES
--------
  curl -X POST http://localhost:8080/auth/request -d 'email=me@example.com&workspace=ws_demo'
  curl -X POST http://localhost:8080/auth/verify -d 'email=me@example.com&code=123456'
  curl -X POST http://localhost:8080/api/endpoints -H 'Authorization: Bearer hr_xxx' -d 'target_url=https://my-service.com/webhook'
  curl http://localhost:8080/api/endpoints -H 'Authorization: Bearer hr_xxx'
  curl -X POST http://localhost:8080/hook/hook_a1b2c -d '{"event":"test"}'
  curl http://localhost:8080/api/endpoints/hook_a1b2c/deliveries -H 'Authorization: Bearer hr_xxx'

STORAGE
-------
Data is persisted to a JSON file (default: hookrelay.json). Zero external dependencies.
`
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, help)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeResponse(w, r, http.StatusOK, "ok", map[string]string{"status": "ok"})
}

func (s *Server) handleRequestOTP(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	workspace := r.FormValue("workspace")

	if email == "" || workspace == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing email or workspace",
			"POST with email=<your-email>&workspace=<handle> (e.g. ws_demo)")
		return
	}

	exists, err := s.store.WorkspaceExists(workspace)
	if err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "database error", "try again")
		return
	}
	if !exists {
		if err := s.store.CreateWorkspace(workspace, workspace); err != nil {
			s.errorResponse(w, r, http.StatusInternalServerError, "failed to create workspace", "try a different workspace handle")
			return
		}
	}

	code := s.auth.GenerateOTP()
	if err := s.store.SaveOTP(email, code, workspace, auth.OTPExpiry()); err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "failed to save OTP", "try again")
		return
	}

	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("otp_sent=true email=%s code=%s", email, code),
		map[string]string{"status": "otp_sent", "email": email, "code": code, "hint": "use POST /auth/verify with this code to get a token"},
	)
}

func (s *Server) handleVerifyOTP(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	code := r.FormValue("code")

	if email == "" || code == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing email or code",
			"POST with email=<your-email>&code=<6-digit-code>")
		return
	}

	workspace, expiresAt, err := s.store.GetOTP(email, code)
	if err != nil {
		s.errorResponse(w, r, http.StatusUnauthorized, "invalid OTP code",
			"request a new OTP via POST /auth/request")
		return
	}

	if time.Now().After(expiresAt) {
		s.store.DeleteOTP(email, code)
		s.errorResponse(w, r, http.StatusUnauthorized, "OTP expired",
			"request a new OTP via POST /auth/request")
		return
	}

	s.store.DeleteOTP(email, code)

	token := s.auth.GenerateToken(workspace)
	if err := s.store.CreateToken(token, workspace, auth.TokenExpiry()); err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "failed to create token", "try again")
		return
	}

	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("token=%s workspace=%s", token, workspace),
		map[string]string{"token": token, "workspace": workspace, "hint": "use this token in Authorization: Bearer header"},
	)
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	workspace, err := s.authenticate(r)
	if err != nil {
		s.errorResponse(w, r, http.StatusUnauthorized, err.Error(),
			"POST /auth/request with email and workspace, then POST /auth/verify with the code")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api")

	switch {
	case path == "/endpoints" && r.Method == "POST":
		s.handleCreateEndpoint(w, r, workspace)
	case path == "/endpoints" && r.Method == "GET":
		s.handleListEndpoints(w, r, workspace)
	case strings.HasPrefix(path, "/endpoints/") && r.Method == "GET":
		rest := strings.TrimPrefix(path, "/endpoints/")
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) == 2 && parts[1] == "deliveries" {
			s.handleListDeliveries(w, r, workspace, parts[0])
		} else {
			s.handleGetEndpoint(w, r, workspace)
		}
	case strings.HasPrefix(path, "/endpoints/") && r.Method == "DELETE":
		s.handleDeleteEndpoint(w, r, workspace)
	case strings.HasPrefix(path, "/deliveries/") && r.Method == "GET":
		s.handleGetDelivery(w, r, workspace)
	case path == "/workspace" && r.Method == "GET":
		s.handleGetWorkspace(w, r, workspace)
	default:
		s.errorResponse(w, r, http.StatusNotFound, "endpoint not found",
			"GET /help to see available endpoints")
	}
}

func (s *Server) handleCreateEndpoint(w http.ResponseWriter, r *http.Request, workspace string) {
	targetURL := r.FormValue("target_url")
	description := r.FormValue("description")

	if targetURL == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing target_url",
			"POST with target_url=<https://your-service.com/webhook>&description=<optional>")
		return
	}

	if !isValidURL(targetURL) {
		s.errorResponse(w, r, http.StatusBadRequest, "invalid target_url",
			"provide a full http:// or https:// URL")
		return
	}

	handle := auth.GenerateHandle("hook")
	if err := s.store.CreateEndpoint(handle, targetURL, description, workspace); err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "failed to create endpoint", "try again")
		return
	}

	ep := &models.Endpoint{Handle: handle, TargetURL: targetURL, Description: description, Workspace: workspace}
	s.writeResponse(w, r, http.StatusCreated,
		fmt.Sprintf("handle=%s target_url=%s", handle, targetURL),
		ep,
	)
}

func (s *Server) handleListEndpoints(w http.ResponseWriter, r *http.Request, workspace string) {
	endpoints, err := s.store.ListEndpoints(workspace, 50)
	if err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "database error", "try again")
		return
	}

	if s.wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if endpoints == nil {
			endpoints = []*models.Endpoint{}
		}
		json.NewEncoder(w).Encode(endpoints)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if len(endpoints) == 0 {
		fmt.Fprintln(w, "no endpoints found. POST /api/endpoints with target_url=<https://your-service.com/webhook> to create one.")
		return
	}
	for _, ep := range endpoints {
		deliveryCount := s.store.CountDeliveries(ep.Handle, workspace)
		desc := ep.Description
		if desc == "" {
			desc = "-"
		}
		fmt.Fprintf(w, "handle=%s target_url=%s description=%s deliveries=%d\n", ep.Handle, ep.TargetURL, desc, deliveryCount)
	}
}

func (s *Server) handleGetEndpoint(w http.ResponseWriter, r *http.Request, workspace string) {
	handle := strings.TrimPrefix(r.URL.Path, "/api/endpoints/")
	if idx := strings.Index(handle, "/"); idx != -1 {
		handle = handle[:idx]
	}
	if handle == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing handle", "GET /api/endpoints/<handle>")
		return
	}

	ep, err := s.store.GetEndpoint(handle)
	if err != nil {
		s.errorResponse(w, r, http.StatusNotFound, "endpoint not found",
			"GET /api/endpoints to list all endpoints")
		return
	}

	if ep.Workspace != workspace {
		s.errorResponse(w, r, http.StatusNotFound, "endpoint not found",
			"this endpoint belongs to a different workspace")
		return
	}

	deliveryCount := s.store.CountDeliveries(ep.Handle, workspace)
	desc := ep.Description
	if desc == "" {
		desc = "-"
	}

	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("handle=%s target_url=%s description=%s deliveries=%d", ep.Handle, ep.TargetURL, desc, deliveryCount),
		ep,
	)
}

func (s *Server) handleDeleteEndpoint(w http.ResponseWriter, r *http.Request, workspace string) {
	handle := strings.TrimPrefix(r.URL.Path, "/api/endpoints/")
	if handle == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing handle", "DELETE /api/endpoints/<handle>")
		return
	}

	if err := s.store.DeleteEndpoint(handle, workspace); err != nil {
		s.errorResponse(w, r, http.StatusNotFound, "endpoint not found",
			"GET /api/endpoints to list all endpoints")
		return
	}

	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("deleted handle=%s", handle),
		map[string]string{"status": "deleted", "handle": handle},
	)
}

func (s *Server) handleListDeliveries(w http.ResponseWriter, r *http.Request, workspace string, endpointHandle string) {
	ep, err := s.store.GetEndpoint(endpointHandle)
	if err != nil {
		s.errorResponse(w, r, http.StatusNotFound, "endpoint not found",
			"GET /api/endpoints to list all endpoints")
		return
	}
	if ep.Workspace != workspace {
		s.errorResponse(w, r, http.StatusNotFound, "endpoint not found",
			"this endpoint belongs to a different workspace")
		return
	}

	deliveries, err := s.store.ListDeliveries(endpointHandle, workspace, 50)
	if err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "database error", "try again")
		return
	}

	if s.wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if deliveries == nil {
			deliveries = []*models.Delivery{}
		}
		json.NewEncoder(w).Encode(deliveries)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if len(deliveries) == 0 {
		fmt.Fprintln(w, "no deliveries found. Send a webhook to /hook/"+endpointHandle+" to create one.")
		return
	}
	for _, d := range deliveries {
		fmt.Fprintf(w, "handle=%s endpoint=%s method=%s status=%d delivered=%t\n", d.Handle, d.EndpointHandle, d.Method, d.StatusCode, d.Delivered)
	}
}

func (s *Server) handleGetDelivery(w http.ResponseWriter, r *http.Request, workspace string) {
	handle := strings.TrimPrefix(r.URL.Path, "/api/deliveries/")
	if handle == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing handle", "GET /api/deliveries/<handle>")
		return
	}

	d, err := s.store.GetDelivery(handle)
	if err != nil {
		s.errorResponse(w, r, http.StatusNotFound, "delivery not found",
			"GET /api/endpoints/<handle>/deliveries to list deliveries")
		return
	}

	if d.Workspace != workspace {
		s.errorResponse(w, r, http.StatusNotFound, "delivery not found",
			"this delivery belongs to a different workspace")
		return
	}

	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("handle=%s endpoint=%s method=%s status=%d delivered=%t\nbody=%s\nresponse=%s",
			d.Handle, d.EndpointHandle, d.Method, d.StatusCode, d.Delivered, truncateBody(d.Body, 500), truncateBody(d.ResponseBody, 500)),
		d,
	)
}

func (s *Server) handleGetWorkspace(w http.ResponseWriter, r *http.Request, workspace string) {
	ws, err := s.store.GetWorkspace(workspace)
	if err != nil {
		s.errorResponse(w, r, http.StatusNotFound, "workspace not found", "create it via POST /auth/request")
		return
	}

	endpoints, _ := s.store.ListEndpoints(workspace, 10000)
	endpointCount := len(endpoints)

	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("handle=%s name=%s plan=%s endpoints=%d", ws.Handle, ws.Name, ws.Plan, endpointCount),
		map[string]string{"handle": ws.Handle, "name": ws.Name, "plan": ws.Plan, "endpoints": fmt.Sprintf("%d", endpointCount)},
	)
}

// handleWebhook receives an incoming webhook and forwards it to the endpoint's target URL.
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	handle := strings.TrimPrefix(r.URL.Path, "/hook/")
	if handle == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing handle", "POST /hook/<handle>")
		return
	}

	ep, err := s.store.GetEndpoint(handle)
	if err != nil {
		s.errorResponse(w, r, http.StatusNotFound, "endpoint not found",
			"check the handle or GET /api/endpoints to list endpoints")
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		s.errorResponse(w, r, http.StatusBadRequest, "failed to read request body", "try again")
		return
	}
	bodyStr := truncateBody(string(bodyBytes), 10000)

	deliveryHandle := auth.GenerateHandle("del")
	statusCode := 0
	responseBody := ""
	delivered := false

	req, err := http.NewRequest(r.Method, ep.TargetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		s.store.CreateDelivery(deliveryHandle, handle, ep.Workspace, r.Method, bodyStr, 0, fmt.Sprintf("failed to create request: %v", err), false)
		s.writeResponse(w, r, http.StatusBadGateway,
			fmt.Sprintf("handle=%s status=0 delivered=false error=failed_to_forward", deliveryHandle),
			map[string]interface{}{"handle": deliveryHandle, "status": 0, "delivered": false, "error": "failed to forward webhook"},
		)
		return
	}

	for _, h := range []string{"Content-Type", "User-Agent", "X-GitHub-Event", "X-GitHub-Delivery", "X-Hub-Signature", "X-Hub-Signature-256", "X-Request-ID"} {
		if v := r.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}

	resp, err := s.http.Do(req)
	if err != nil {
		s.store.CreateDelivery(deliveryHandle, handle, ep.Workspace, r.Method, bodyStr, 0, fmt.Sprintf("forward failed: %v", err), false)
		s.writeResponse(w, r, http.StatusBadGateway,
			fmt.Sprintf("handle=%s status=0 delivered=false error=forward_failed", deliveryHandle),
			map[string]interface{}{"handle": deliveryHandle, "status": 0, "delivered": false, "error": "forward failed"},
		)
		return
	}
	defer resp.Body.Close()

	statusCode = resp.StatusCode
	respBody, _ := io.ReadAll(resp.Body)
	responseBody = truncateBody(string(respBody), 10000)
	delivered = statusCode >= 200 && statusCode < 400

	s.store.CreateDelivery(deliveryHandle, handle, ep.Workspace, r.Method, bodyStr, statusCode, responseBody, delivered)

	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("handle=%s status=%d delivered=%t", deliveryHandle, statusCode, delivered),
		map[string]interface{}{"handle": deliveryHandle, "status": statusCode, "delivered": delivered},
	)
}
