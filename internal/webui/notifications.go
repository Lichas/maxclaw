package webui

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID        string                 `json:"id"`
	Title     string                 `json:"title"`
	Body      string                 `json:"body"`
	Data      map[string]interface{} `json:"data"`
	CreatedAt time.Time              `json:"createdAt"`
	Delivered bool                   `json:"delivered"`
}

type NotificationStore struct {
	mu            sync.RWMutex
	notifications []Notification
	maxSize       int
}

func NewNotificationStore() *NotificationStore {
	return &NotificationStore{
		notifications: make([]Notification, 0),
		maxSize:       100,
	}
}

func (s *NotificationStore) Add(title, body string, data map[string]interface{}) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	notif := Notification{
		ID:        uuid.New().String()[:8],
		Title:     title,
		Body:      body,
		Data:      data,
		CreatedAt: time.Now(),
		Delivered: false,
	}

	s.notifications = append(s.notifications, notif)

	// Trim old notifications
	if len(s.notifications) > s.maxSize {
		s.notifications = s.notifications[len(s.notifications)-s.maxSize:]
	}

	return notif.ID
}

func (s *NotificationStore) GetPending() []Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pending := make([]Notification, 0)
	for _, n := range s.notifications {
		if !n.Delivered {
			pending = append(pending, n)
		}
	}
	return pending
}

func (s *NotificationStore) MarkDelivered(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.notifications {
		if s.notifications[i].ID == id {
			s.notifications[i].Delivered = true
			return
		}
	}
}

// HTTP Handlers
func (s *Server) handleGetPendingNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	pending := s.notificationStore.GetPending()
	writeJSON(w, pending)
}

func (s *Server) handleMarkNotificationDelivered(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path: /api/notifications/{id}/delivered
	path := strings.TrimPrefix(r.URL.Path, "/api/notifications/")
	path = strings.TrimSuffix(path, "/delivered")
	id := strings.TrimSpace(path)

	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	s.notificationStore.MarkDelivered(id)
	writeJSON(w, map[string]bool{"ok": true})
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
