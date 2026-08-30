package response

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
)

// ── envelope types ────────────────────────────────────────────────────────────

type Meta struct {
	Cursor  *string `json:"cursor"`
	HasMore bool    `json:"has_more"`
	Total   int64   `json:"total"`
}

type errorBody struct {
	Error apiErr `json:"error"`
}

type apiErr struct {
	Code      string   `json:"code"`
	Message   string   `json:"message"`
	Details   []detail `json:"details,omitempty"`
	RequestID string   `json:"request_id,omitempty"`
}

type detail struct {
	Field string `json:"field"`
	Issue string `json:"issue"`
}

// ── writers ───────────────────────────────────────────────────────────────────

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func Ok(w http.ResponseWriter, v any)      { JSON(w, http.StatusOK, v) }
func Created(w http.ResponseWriter, v any) { JSON(w, http.StatusCreated, v) }
func NoContent(w http.ResponseWriter)      { w.WriteHeader(http.StatusNoContent) }

func Paginated(w http.ResponseWriter, data any, meta *Meta) {
	JSON(w, http.StatusOK, map[string]any{"data": data, "meta": meta})
}

func Err(w http.ResponseWriter, status int, code, msg, rid string) {
	JSON(w, status, errorBody{Error: apiErr{Code: code, Message: msg, RequestID: rid}})
}

func BadRequest(w http.ResponseWriter, msg, rid string) {
	Err(w, http.StatusBadRequest, "BAD_REQUEST", msg, rid)
}
func Unauthorized(w http.ResponseWriter, rid string) {
	Err(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired token", rid)
}
func Forbidden(w http.ResponseWriter, rid string) {
	Err(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", rid)
}
func NotFound(w http.ResponseWriter, resource, rid string) {
	Err(w, http.StatusNotFound, "NOT_FOUND", resource+" not found", rid)
}
func Conflict(w http.ResponseWriter, msg, rid string) {
	Err(w, http.StatusConflict, "CONFLICT", msg, rid)
}
func Unprocessable(w http.ResponseWriter, msg, rid string) {
	Err(w, http.StatusUnprocessableEntity, "UNPROCESSABLE", msg, rid)
}
func InternalError(w http.ResponseWriter, rid string) {
	Err(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred", rid)
}
func TooManyRequests(w http.ResponseWriter, rid string) {
	w.Header().Set("Retry-After", "60")
	Err(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests", rid)
}

func ValidationError(w http.ResponseWriter, err error, rid string) {
	var details []detail
	if ve, ok := err.(validator.ValidationErrors); ok {
		for _, fe := range ve {
			details = append(details, detail{Field: snakeCase(fe.Field()), Issue: tagMsg(fe)})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(errorBody{Error: apiErr{
		Code: "VALIDATION_ERROR", Message: "Validation failed",
		Details: details, RequestID: rid,
	}})
}

func snakeCase(s string) string {
	out := make([]rune, 0, len(s)+4)
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				out = append(out, '_')
			}
			out = append(out, r+32)
		} else {
			out = append(out, r)
		}
	}
	return string(out)
}

func tagMsg(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "required"
	case "email":
		return "must be a valid email"
	case "min":
		return "too short (min " + fe.Param() + ")"
	case "max":
		return "too long (max " + fe.Param() + ")"
	case "e164":
		return "must be E.164 format e.g. +237612345678"
	case "uuid":
		return "must be a valid UUID"
	case "oneof":
		return "must be one of: " + fe.Param()
	case "gte":
		return "must be >= " + fe.Param()
	case "lte":
		return "must be <= " + fe.Param()
	default:
		return "failed: " + fe.Tag()
	}
}
