package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOk(t *testing.T) {
	rr := httptest.NewRecorder()
	Ok(rr, map[string]string{"key": "value"})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("Content-Type") != "application/json" {
		t.Fatal("expected JSON content type")
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["key"] != "value" {
		t.Fatalf("expected value, got %s", body["key"])
	}
}

func TestCreated(t *testing.T) {
	rr := httptest.NewRecorder()
	Created(rr, map[string]string{"id": "abc"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
}

func TestNoContent(t *testing.T) {
	rr := httptest.NewRecorder()
	NoContent(rr)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if rr.Body.Len() != 0 {
		t.Fatal("expected empty body for 204")
	}
}

func TestNotFound(t *testing.T) {
	rr := httptest.NewRecorder()
	NotFound(rr, "Order", "req-123")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
	var body errorBody
	json.Unmarshal(rr.Body.Bytes(), &body)
	if body.Error.Code != "NOT_FOUND" {
		t.Fatalf("expected NOT_FOUND code, got %s", body.Error.Code)
	}
	if body.Error.RequestID != "req-123" {
		t.Fatalf("expected request_id req-123, got %s", body.Error.RequestID)
	}
}

func TestUnauthorized(t *testing.T) {
	rr := httptest.NewRecorder()
	Unauthorized(rr, "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestForbidden(t *testing.T) {
	rr := httptest.NewRecorder()
	Forbidden(rr, "")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestConflict(t *testing.T) {
	rr := httptest.NewRecorder()
	Conflict(rr, "already exists", "")
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestInternalError(t *testing.T) {
	rr := httptest.NewRecorder()
	InternalError(rr, "rid")
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestTooManyRequests(t *testing.T) {
	rr := httptest.NewRecorder()
	TooManyRequests(rr, "")
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestPaginated(t *testing.T) {
	rr := httptest.NewRecorder()
	cur := "abc123"
	Paginated(rr, []string{"a", "b"}, &Meta{Cursor: &cur, HasMore: true, Total: 100})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body map[string]json.RawMessage
	json.Unmarshal(rr.Body.Bytes(), &body)
	if _, ok := body["data"]; !ok {
		t.Fatal("expected data field")
	}
	if _, ok := body["meta"]; !ok {
		t.Fatal("expected meta field")
	}
}
