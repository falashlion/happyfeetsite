package media

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/happyfeet/api/pkg/cache"
	"github.com/happyfeet/api/pkg/config"
	"github.com/happyfeet/api/pkg/middleware"
	"github.com/happyfeet/api/pkg/response"
	"github.com/happyfeet/api/pkg/validator"
)

type UploadStatus struct {
	UploadID string
	Status   string
	ImageURL *string
	Error    *string
}

type Handler struct {
	cfg   *config.Config
	cache *cache.Client
}

func NewHandler(cfg *config.Config, c *cache.Client) *Handler { return &Handler{cfg, c} }

// RequestUpload godoc
// @Summary      Request pre-signed upload URL
// @Tags         media
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /media/upload [post]
func (h *Handler) RequestUpload(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req struct {
		Purpose     string `json:"purpose"      validate:"required,oneof=avatar review_image product_image"`
		ContentType string `json:"content_type" validate:"required,oneof=image/jpeg image/png image/webp"`
		SizeBytes   int64  `json:"size_bytes"   validate:"required,gte=1,lte=52428800"`
	}
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}

	uploadID := uuid.NewString()
	// Key pattern: {purpose}/{first4}/{uploadID}.{ext}
	ext := extFromCT(req.ContentType)
	key := fmt.Sprintf("%s/%s/%s%s", req.Purpose, uploadID[:4], uploadID, ext)
	presignedURL := fmt.Sprintf("%s/%s/%s?X-Amz-Expires=900", h.cfg.AWS.S3Endpoint, h.cfg.AWS.S3Bucket, key)

	// Store pending status in cache
	_ = h.cache.SetJSON(r.Context(), fmt.Sprintf("upload:%s", uploadID),
		UploadStatus{UploadID: uploadID, Status: "pending"}, 30*time.Minute)

	response.Ok(w, map[string]any{
		"upload_id":     uploadID,
		"presigned_url": presignedURL,
		"key":           key,
		"expires_at":    time.Now().Add(15 * time.Minute),
		"max_size_bytes": 52428800,
	})
}

// GetUploadStatus godoc
// @Summary      Get upload processing status
// @Tags         media
// @Security     BearerAuth
// @Param        uploadId  path  string  true  "Upload UUID"
// @Success      200  {object}  map[string]any
// @Router       /media/upload/{uploadId}/status [get]
func (h *Handler) GetUploadStatus(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	uploadID := chi.URLParam(r, "uploadId")
	var status UploadStatus
	if err := h.cache.GetJSON(r.Context(), fmt.Sprintf("upload:%s", uploadID), &status); err != nil {
		response.NotFound(w, "Upload", rid)
		return
	}
	response.Ok(w, map[string]any{
		"upload_id": uploadID,
		"status":    status.Status,
		"image_url": status.ImageURL,
		"error":     status.Error,
	})
}

func extFromCT(ct string) string {
	switch ct {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ".jpg"
	}
}

// satisfy unused imports
var _ = context.Background
