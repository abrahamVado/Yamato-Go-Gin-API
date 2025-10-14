package joinrequests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	internalauth "github.com/example/Yamato-Go-Gin-API/internal/auth"
)

// 1.- memoryService provides an in-memory implementation for handler tests.
type memoryService struct {
	requests map[string]JoinRequest
	order    []string
	nextID   int
}

// 1.- newMemoryService constructs a memory-backed service instance.
func newMemoryService() *memoryService {
	return &memoryService{requests: map[string]JoinRequest{}, order: []string{}, nextID: 1}
}

// 1.- Submit stores the join request and seeds the audit trail.
func (m *memoryService) Submit(_ context.Context, submission Submission) (JoinRequest, error) {
	// 2.- Create the join request with deterministic timestamps for testing.
	now := time.Unix(int64(m.nextID), 0).UTC()
	id := m.nextIdentifier()
	req := JoinRequest{
		ID:        id,
		User:      submission.User,
		Email:     submission.Email,
		Status:    StatusPending,
		Payload:   submission.Payload,
		CreatedAt: now,
		UpdatedAt: now,
		AuditTrail: []AuditEntry{{
			ActorID:    "system",
			Action:     "submitted",
			OccurredAt: now,
		}},
	}
	m.requests[id] = req
	m.order = append(m.order, id)
	return req, nil
}

// 1.- List returns join requests honoring the status filter when provided.
func (m *memoryService) List(_ context.Context, filter Filter) ([]JoinRequest, error) {
	results := make([]JoinRequest, 0, len(m.order))
	for _, id := range m.order {
		req := m.requests[id]
		if filter.Status != "" && req.Status != filter.Status {
			continue
		}
		results = append(results, req)
	}
	return results, nil
}

// 1.- Approve updates the status and appends an audit entry.
func (m *memoryService) Approve(_ context.Context, id string, decision Decision) (JoinRequest, error) {
	req, ok := m.requests[id]
	if !ok {
		return JoinRequest{}, ErrJoinRequestNotFound
	}
	if req.Status != StatusPending {
		return JoinRequest{}, ErrInvalidStatusTransition
	}
	now := time.Unix(int64(len(req.AuditTrail)+m.nextID), 0).UTC()
	req.Status = StatusApproved
	req.UpdatedAt = now
	req.AuditTrail = append(req.AuditTrail, AuditEntry{
		ActorID:    decision.ActorID,
		Action:     "approved",
		Note:       decision.Note,
		OccurredAt: now,
	})
	m.requests[id] = req
	return req, nil
}

// 1.- Decline updates the status and appends an audit entry.
func (m *memoryService) Decline(_ context.Context, id string, decision Decision) (JoinRequest, error) {
	req, ok := m.requests[id]
	if !ok {
		return JoinRequest{}, ErrJoinRequestNotFound
	}
	if req.Status != StatusPending {
		return JoinRequest{}, ErrInvalidStatusTransition
	}
	now := time.Unix(int64(len(req.AuditTrail)+m.nextID+1), 0).UTC()
	req.Status = StatusDeclined
	req.UpdatedAt = now
	req.AuditTrail = append(req.AuditTrail, AuditEntry{
		ActorID:    decision.ActorID,
		Action:     "declined",
		Note:       decision.Note,
		OccurredAt: now,
	})
	m.requests[id] = req
	return req, nil
}

// 1.- nextIdentifier creates unique identifiers for each request.
func (m *memoryService) nextIdentifier() string {
	id := m.nextID
	m.nextID++
	return "jr-" + strconv.Itoa(id)
}

// 1.- successPayload helps decode standard success responses in tests.
type successPayload[T any] struct {
	Data T              `json:"data"`
	Meta map[string]any `json:"meta"`
}

// 1.- TestSubmitJoinRequestPersistsPayload verifies the submission flow.
func TestSubmitJoinRequestPersistsPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newMemoryService()
	handler := NewHandler(service)

	body := map[string]any{
		"user":    "Sakura",
		"email":   "SAKURA@example.com",
		"payload": map[string]any{"team": "crimson"},
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/join-requests", bytes.NewReader(raw))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.Submit(ctx)
	require.Equal(t, http.StatusCreated, recorder.Code)

	var payload successPayload[JoinRequest]
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, StatusPending, payload.Data.Status)
	require.Equal(t, "sakura@example.com", payload.Data.Email)
	require.Len(t, payload.Data.AuditTrail, 1)
	require.Equal(t, "submitted", payload.Data.AuditTrail[0].Action)
}

// 1.- TestApproveJoinRequestTransitionsStatus ensures approvals update state and audit trail.
func TestApproveJoinRequestTransitionsStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newMemoryService()
	handler := NewHandler(service)

	created, err := service.Submit(context.Background(), Submission{User: "Touya", Email: "touya@example.com"})
	require.NoError(t, err)

	decision := map[string]any{"note": "Welcome aboard"}
	raw, err := json.Marshal(decision)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = append(ctx.Params, gin.Param{Key: "id", Value: created.ID})
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/join-requests/"+created.ID+"/approve", bytes.NewReader(raw))
	ctx.Request.Header.Set("Content-Type", "application/json")
	internalauth.SetPrincipal(ctx, internalauth.Principal{Subject: "admin-1"})

	handler.Approve(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload successPayload[JoinRequest]
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, StatusApproved, payload.Data.Status)
	require.Equal(t, 2, len(payload.Data.AuditTrail))
	require.Equal(t, "approved", payload.Data.AuditTrail[1].Action)
	require.Equal(t, "admin-1", payload.Data.AuditTrail[1].ActorID)
	require.Equal(t, "Welcome aboard", payload.Data.AuditTrail[1].Note)
}

// 1.- TestDeclineJoinRequestTransitionsStatus ensures rejections update state and audit trail.
func TestDeclineJoinRequestTransitionsStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newMemoryService()
	handler := NewHandler(service)

	created, err := service.Submit(context.Background(), Submission{User: "Yukito", Email: "yukito@example.com"})
	require.NoError(t, err)

	decision := map[string]any{"note": "Team is full"}
	raw, err := json.Marshal(decision)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = append(ctx.Params, gin.Param{Key: "id", Value: created.ID})
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/join-requests/"+created.ID+"/decline", bytes.NewReader(raw))
	ctx.Request.Header.Set("Content-Type", "application/json")
	internalauth.SetPrincipal(ctx, internalauth.Principal{Subject: "admin-2"})

	handler.Decline(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload successPayload[JoinRequest]
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, StatusDeclined, payload.Data.Status)
	require.Equal(t, 2, len(payload.Data.AuditTrail))
	require.Equal(t, "declined", payload.Data.AuditTrail[1].Action)
	require.Equal(t, "admin-2", payload.Data.AuditTrail[1].ActorID)
	require.Equal(t, "Team is full", payload.Data.AuditTrail[1].Note)
}

// 1.- TestSubmitJoinRequestValidatesRequiredFields covers validation failures.
func TestSubmitJoinRequestValidatesRequiredFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newMemoryService()
	handler := NewHandler(service)

	body := map[string]any{
		"user":  " ",
		"email": "invalid-email",
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/join-requests", bytes.NewReader(raw))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.Submit(ctx)
	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
}
