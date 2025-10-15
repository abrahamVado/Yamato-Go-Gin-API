package memory

import (
	"context"
	"sync"
	"time"

	notifications "github.com/example/Yamato-Go-Gin-API/internal/http/notifications"
)

// 1.- NotificationService stores notification history in-memory for development environments.
type NotificationService struct {
	mu    sync.RWMutex
	seed  []notifications.Notification
	items map[string][]notifications.Notification
}

// 1.- NewNotificationService seeds the service with baseline notifications reused per user.
func NewNotificationService(seed []notifications.Notification) *NotificationService {
	clonedSeed := make([]notifications.Notification, len(seed))
	copy(clonedSeed, seed)
	return &NotificationService{seed: clonedSeed, items: map[string][]notifications.Notification{}}
}

// 1.- ensureUser provisions a copy of the seed notifications for new users.
func (s *NotificationService) ensureUser(userID string) {
	if _, exists := s.items[userID]; exists {
		return
	}
	cloned := make([]notifications.Notification, len(s.seed))
	for i, item := range s.seed {
		dup := item
		dup.UserID = userID
		s.resetReadState(&dup)
		cloned[i] = dup
	}
	s.items[userID] = cloned
}

// 1.- resetReadState clears the read timestamp ensuring new users start unread.
func (s *NotificationService) resetReadState(item *notifications.Notification) {
	item.ReadAt = nil
}

// 1.- List returns a paginated slice of notifications for the requested user.
func (s *NotificationService) List(_ context.Context, userID string, page int, perPage int) (notifications.Page, error) {
	if perPage <= 0 {
		perPage = 20
	}
	if page <= 0 {
		page = 1
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureUser(userID)
	records := s.items[userID]

	total := len(records)
	start := (page - 1) * perPage
	if start > total {
		start = total
	}
	end := start + perPage
	if end > total {
		end = total
	}
	slice := make([]notifications.Notification, end-start)
	copy(slice, records[start:end])

	return notifications.Page{Items: slice, Total: total, Page: page, PerPage: perPage}, nil
}

// 1.- MarkRead flags the provided notification as read, recording the timestamp.
func (s *NotificationService) MarkRead(_ context.Context, userID string, notificationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureUser(userID)
	notificationsForUser := s.items[userID]
	for i := range notificationsForUser {
		if notificationsForUser[i].ID == notificationID {
			now := time.Now().UTC()
			notificationsForUser[i].ReadAt = &now
			s.items[userID][i] = notificationsForUser[i]
			return nil
		}
	}
	return notifications.ErrNotificationNotFound
}

// 1.- DefaultNotifications returns a curated slice of baseline notifications.
func DefaultNotifications() []notifications.Notification {
	now := time.Now().UTC()
	return []notifications.Notification{
		{ID: "alert-1", Title: "Welcome to Yamato", Body: "You're ready to explore the cockpit.", CreatedAt: now.Add(-2 * time.Hour)},
		{ID: "alert-2", Title: "Security Review", Body: "Review the latest access policy updates.", CreatedAt: now.Add(-90 * time.Minute)},
		{ID: "alert-3", Title: "Team Sync", Body: "Your crew left comments on the mission brief.", CreatedAt: now.Add(-45 * time.Minute)},
	}
}
