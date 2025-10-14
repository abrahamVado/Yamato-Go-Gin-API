package respond

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// 1.- TestSuccessProducesCanonicalEnvelope validates the success helper's JSON structure.
func TestSuccessProducesCanonicalEnvelope(t *testing.T) {
	// 2.- Silence Gin logging to keep test output deterministic.
	gin.SetMode(gin.TestMode)

	// 3.- Prepare a recorder-backed context for issuing the response.
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	// 4.- Invoke the helper with representative data and no metadata.
	payload := map[string]string{"message": "created"}
	Success(ctx, http.StatusCreated, payload, nil)

	// 5.- Confirm the HTTP status propagates as expected.
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, recorder.Code)
	}

	// 6.- Decode the JSON body for fine-grained assertions.
	var envelope SuccessEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// 7.- Assert the canonical status flag and payload fields.
	if envelope.Status != "success" {
		t.Fatalf("expected status field to be success, got %q", envelope.Status)
	}
	data, ok := envelope.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to decode into a map, got %T", envelope.Data)
	}
	if data["message"] != payload["message"] {
		t.Fatalf("expected message %q, got %v", payload["message"], data["message"])
	}
	if len(envelope.Meta) != 0 {
		t.Fatalf("expected empty meta map, got %v", envelope.Meta)
	}
}

// 1.- TestPaginatedWrapsMetadata ensures pagination helpers embed metadata consistently.
func TestPaginatedWrapsMetadata(t *testing.T) {
	// 2.- Configure Gin for testing to avoid extraneous output.
	gin.SetMode(gin.TestMode)

	// 3.- Create a recorder-backed context to capture the response.
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	// 4.- Execute the pagination helper with sample data and pagination info.
	Paginated(ctx, http.StatusOK, []int{1, 2, 3}, PaginationMeta{Page: 2, PerPage: 10, Total: 30, TotalPages: 3})

	// 5.- Expect the helper to set the provided status code.
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	// 6.- Decode the envelope to inspect metadata placement.
	var envelope SuccessEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// 7.- Validate pagination lives under meta.pagination with the correct values.
	pagination, ok := envelope.Meta["pagination"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected pagination metadata, got %T", envelope.Meta["pagination"])
	}
	if pagination["page"].(float64) != 2 {
		t.Fatalf("expected page to be 2, got %v", pagination["page"])
	}
	if pagination["per_page"].(float64) != 10 {
		t.Fatalf("expected per_page to be 10, got %v", pagination["per_page"])
	}
	if pagination["total"].(float64) != 30 {
		t.Fatalf("expected total to be 30, got %v", pagination["total"])
	}
	if pagination["total_pages"].(float64) != 3 {
		t.Fatalf("expected total_pages to be 3, got %v", pagination["total_pages"])
	}
}

// 1.- TestWriteErrorProducesCanonicalEnvelope validates JSON produced by WriteError.
func TestWriteErrorProducesCanonicalEnvelope(t *testing.T) {
	// 2.- Switch Gin into test mode for quiet output.
	gin.SetMode(gin.TestMode)

	// 3.- Prepare a test context backed by an HTTP recorder.
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	// 4.- Write a structured error using the helper to capture its envelope.
	WriteError(ctx, NewError(http.StatusBadRequest, "validation failed", map[string]interface{}{"email": "invalid"}))

	// 5.- Assert the response status reflects the encoded error.
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	// 6.- Decode the error payload for structural verification.
	var envelope ErrorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	// 7.- Confirm the envelope matches the canonical error schema.
	if envelope.Status != "error" {
		t.Fatalf("expected status field to be error, got %q", envelope.Status)
	}
	if envelope.Message != "validation failed" {
		t.Fatalf("expected message to be validation failed, got %q", envelope.Message)
	}
	if envelope.Errors["email"] != "invalid" {
		t.Fatalf("expected email error to be invalid, got %v", envelope.Errors["email"])
	}
}
