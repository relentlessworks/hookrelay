package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/relentlessworks/hookrelay/internal/models"
)

// Store is a file-backed persistent store using JSON.
type Store struct {
	mu       sync.RWMutex
	filePath string
	data     *storeData
}

type storeData struct {
	Workspaces map[string]*models.Workspace `json:"workspaces"`
	Endpoints  map[string]*models.Endpoint  `json:"endpoints"`
	Deliveries map[string]*models.Delivery  `json:"deliveries"`
	Tokens     map[string]*models.Token     `json:"tokens"`
	OTPs       map[string]*otpEntry         `json:"otps"`
}

type otpEntry struct {
	Email     string    `json:"email"`
	Code      string    `json:"code"`
	Workspace string    `json:"workspace"`
	ExpiresAt time.Time `json:"expires_at"`
}

// New opens (or creates) the store file.
func New(path string) (*Store, error) {
	s := &Store{
		filePath: path,
		data: &storeData{
			Workspaces: make(map[string]*models.Workspace),
			Endpoints:  make(map[string]*models.Endpoint),
			Deliveries: make(map[string]*models.Delivery),
			Tokens:     make(map[string]*models.Token),
			OTPs:       make(map[string]*otpEntry),
		},
	}

	if _, err := os.Stat(path); err == nil {
		if err := s.load(); err != nil {
			return nil, fmt.Errorf("load store: %w", err)
		}
	}

	return s, nil
}

// Close persists any pending changes.
func (s *Store) Close() error {
	return s.save()
}

func (s *Store) load() error {
	b, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	if len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, s.data)
}

func (s *Store) save() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.filePath)
	if dir == "" {
		dir = "."
	}
	tmpFile, err := os.CreateTemp(dir, ".hookrelay-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(b); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, s.filePath)
}

// --- Workspace operations ---

func (s *Store) CreateWorkspace(handle, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Workspaces[handle] = &models.Workspace{
		Handle: handle,
		Name:   name,
		Plan:   "free",
	}
	return s.save()
}

func (s *Store) GetWorkspace(handle string) (*models.Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ws, ok := s.data.Workspaces[handle]
	if !ok {
		return nil, fmt.Errorf("workspace not found")
	}
	return ws, nil
}

func (s *Store) WorkspaceExists(handle string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.data.Workspaces[handle]
	return ok, nil
}

// --- Endpoint operations ---

func (s *Store) CreateEndpoint(handle, targetURL, description, workspace string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Endpoints[handle] = &models.Endpoint{
		Handle:      handle,
		Workspace:   workspace,
		TargetURL:   targetURL,
		Description: description,
		CreatedAt:   time.Now(),
	}
	return s.save()
}

func (s *Store) GetEndpoint(handle string) (*models.Endpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ep, ok := s.data.Endpoints[handle]
	if !ok {
		return nil, fmt.Errorf("endpoint not found")
	}
	return ep, nil
}

func (s *Store) ListEndpoints(workspace string, limit int) ([]*models.Endpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var endpoints []*models.Endpoint
	for _, ep := range s.data.Endpoints {
		if ep.Workspace == workspace {
			endpoints = append(endpoints, ep)
		}
	}

	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].CreatedAt.After(endpoints[j].CreatedAt)
	})

	if len(endpoints) > limit {
		endpoints = endpoints[:limit]
	}
	return endpoints, nil
}

func (s *Store) DeleteEndpoint(handle, workspace string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ep, ok := s.data.Endpoints[handle]
	if !ok || ep.Workspace != workspace {
		return fmt.Errorf("endpoint not found")
	}
	delete(s.data.Endpoints, handle)
	return s.save()
}

// --- Delivery operations ---

func (s *Store) CreateDelivery(handle, endpointHandle, workspace, method, body string, statusCode int, responseBody string, delivered bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Deliveries[handle] = &models.Delivery{
		Handle:         handle,
		EndpointHandle: endpointHandle,
		Workspace:      workspace,
		Method:         method,
		Body:           body,
		StatusCode:     statusCode,
		ResponseBody:   responseBody,
		Delivered:      delivered,
		CreatedAt:      time.Now(),
	}
	return s.save()
}

func (s *Store) GetDelivery(handle string) (*models.Delivery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.data.Deliveries[handle]
	if !ok {
		return nil, fmt.Errorf("delivery not found")
	}
	return d, nil
}

func (s *Store) ListDeliveries(endpointHandle, workspace string, limit int) ([]*models.Delivery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var deliveries []*models.Delivery
	for _, d := range s.data.Deliveries {
		if d.Workspace == workspace && d.EndpointHandle == endpointHandle {
			deliveries = append(deliveries, d)
		}
	}

	sort.Slice(deliveries, func(i, j int) bool {
		return deliveries[i].CreatedAt.After(deliveries[j].CreatedAt)
	})

	if len(deliveries) > limit {
		deliveries = deliveries[:limit]
	}
	return deliveries, nil
}

// --- Token operations ---

func (s *Store) CreateToken(value, workspace string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Tokens[value] = &models.Token{
		Value:     value,
		Workspace: workspace,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}
	return s.save()
}

func (s *Store) GetToken(value string) (*models.Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	token, ok := s.data.Tokens[value]
	if !ok {
		return nil, fmt.Errorf("token not found")
	}
	return token, nil
}

func (s *Store) DeleteToken(value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Tokens, value)
	return s.save()
}

// --- OTP operations ---

func (s *Store) SaveOTP(email, code, workspace string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := email + ":" + code
	s.data.OTPs[key] = &otpEntry{
		Email:     email,
		Code:      code,
		Workspace: workspace,
		ExpiresAt: expiresAt,
	}
	return s.save()
}

func (s *Store) GetOTP(email, code string) (string, time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := email + ":" + code
	entry, ok := s.data.OTPs[key]
	if !ok {
		return "", time.Time{}, fmt.Errorf("OTP not found")
	}
	return entry.Workspace, entry.ExpiresAt, nil
}

func (s *Store) DeleteOTP(email, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := email + ":" + code
	delete(s.data.OTPs, key)
	return s.save()
}
