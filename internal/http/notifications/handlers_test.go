package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	internalauth "github.com/example/Yamato-Go-Gin-API/internal/auth"
)

// 1.- memoryService provides an in-memory implementation of the notifications service.
type memoryService struct {
	notifications map[string][]Notification
}

// 1.- newMemoryService seeds notifications for deterministic testing.
func newMemoryService() *memoryService {
	return &memoryService{notifications: map[string][]Notification{}}
}

// 1.- List returns a paginated slice of notifications for the stored user.
func (m *memoryService) List(_ context.Context, userID string, page int, perPage int) (Page, error) {
	// 2.- Fetch the notifications for the requested user.
	notifications, ok := m.notifications[userID]
	if !ok {
		return Page{Items: []Notification{}, Total: 0, Page: page, PerPage: perPage}, nil
	}

	// 3.- Calculate pagination boundaries.
	start := (page - 1) * perPage
	if start > len(notifications) {
		start = len(notifications)
	}
	end := start + perPage
	if end > len(notifications) {
		end = len(notifications)
	}

	// 4.- Slice the data to the requested page.
	pageItems := make([]Notification, end-start)
	copy(pageItems, notifications[start:end])

	return Page{Items: pageItems, Total: len(notifications), Page: page, PerPage: perPage}, nil
}

// 1.- MarkRead updates the ReadAt timestamp for the targeted notification.
func (m *memoryService) MarkRead(_ context.Context, userID string, notificationID string) error {
	// 2.- Retrieve the user's notifications; return not found when missing.
	notifications, ok := m.notifications[userID]
	if !ok {
		return ErrNotificationNotFound
	}

	// 3.- Search for the notification and assign a timestamp when found.
	for idx := range notifications {
		if notifications[idx].ID == notificationID {
			now := time.Now().UTC()
			notifications[idx].ReadAt = &now
			m.notifications[userID] = notifications
			return nil
		}
	}

	return ErrNotificationNotFound
}

// 1.- successPayload decodes success envelopes for assertions.
type successPayload[T any] struct {
	Data T              `json:"data"`
	Meta map[string]any `json:"meta"`
}

type listBody struct {
	Items []Notification `json:"items"`
}

// 1.- TestListNotificationsProvidesPaginationMeta validates pagination output.
func TestListNotificationsProvidesPaginationMeta(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newMemoryService()

	// 2.- Seed 25 notifications to exercise pagination logic.
	userID := "user-123"
	for i := 0; i < 25; i++ {
		service.notifications[userID] = append(service.notifications[userID], Notification{
			ID:        fmt.Sprintf("notif-%02d", i+1),
			UserID:    userID,
			Title:     fmt.Sprintf("Notification %d", i+1),
			Body:      "test",
			CreatedAt: time.Unix(int64(i+1), 0).UTC(),
		})
	}

	handler := NewHandler(service)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/notifications?page=2&per_page=10", nil)
	internalauth.SetPrincipal(ctx, internalauth.Principal{Subject: userID})

	handler.List(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload successPayload[listBody]
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))

	require.Len(t, payload.Data.Items, 10)
	require.Equal(t, "notif-11", payload.Data.Items[0].ID)

	pagination, ok := payload.Meta["pagination"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(2), pagination["page"])
	require.Equal(t, float64(10), pagination["per_page"])
	require.Equal(t, float64(25), pagination["total"])
	require.Equal(t, float64(3), pagination["total_pages"])
}

// 1.- TestMarkReadTransitionEnsuresNotificationIsMarkedRead validates state updates.
func TestMarkReadTransitionEnsuresNotificationIsMarkedRead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newMemoryService()

	// 2.- Seed a single unread notification for the target user.
	userID := "user-456"
	notificationID := "notif-1"
	service.notifications[userID] = []Notification{{
		ID:        notificationID,
		UserID:    userID,
		Title:     "Welcome",
		Body:      "hello",
		CreatedAt: time.Unix(10, 0).UTC(),
	}}

	handler := NewHandler(service)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = append(ctx.Params, gin.Param{Key: "id", Value: notificationID})
	ctx.Request = httptest.NewRequest(http.MethodPatch, "/v1/notifications/"+notificationID, nil)
	internalauth.SetPrincipal(ctx, internalauth.Principal{Subject: userID})

	handler.MarkRead(ctx)
	require.Equal(t, http.StatusNoContent, recorder.Code)

	// 3.- List notifications again and ensure the ReadAt field is populated.
	listRecorder := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRecorder)
	listCtx.Request = httptest.NewRequest(http.MethodGet, "/v1/notifications", nil)
	internalauth.SetPrincipal(listCtx, internalauth.Principal{Subject: userID})

	handler.List(listCtx)
	require.Equal(t, http.StatusOK, listRecorder.Code)

	var payload successPayload[listBody]
	require.NoError(t, json.Unmarshal(listRecorder.Body.Bytes(), &payload))

	require.Len(t, payload.Data.Items, 1)
	require.NotNil(t, payload.Data.Items[0].ReadAt)
}
