// Package media issues signed Cloudinary upload credentials.
//
// The browser uploads straight to Cloudinary; the bytes never touch this
// server. We only sign the request parameters, which is what lets us constrain
// where an upload may land (folder) and for how long the permission is good
// (timestamp). The API secret signs server-side and is never serialised to the
// client.
package media

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/happyfeet/api/pkg/cache"
	"github.com/happyfeet/api/pkg/config"
	"github.com/happyfeet/api/pkg/middleware"
	"github.com/happyfeet/api/pkg/response"
	"github.com/happyfeet/api/pkg/validator"
)

// signatureTTL is how long Cloudinary will accept a signature for. Cloudinary
// rejects timestamps older than one hour; we stay well inside that.
const signatureTTL = 15 * time.Minute

type UploadStatus struct {
	UploadID string  `json:"upload_id"`
	Status   string  `json:"status"`
	ImageURL *string `json:"image_url,omitempty"`
	Error    *string `json:"error,omitempty"`
}

type Handler struct {
	cfg   *config.Config
	cache *cache.Client
}

func NewHandler(cfg *config.Config, c *cache.Client) *Handler { return &Handler{cfg, c} }

// RequestUpload godoc
// @Summary      Request signed Cloudinary upload credentials
// @Tags         media
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]any
// @Failure      503  {object}  map[string]any
// @Router       /media/upload [post]
func (h *Handler) RequestUpload(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())

	// Unconfigured means unavailable. Handing back credentials that cannot work
	// would surface as an opaque failure inside Cloudinary's response instead.
	if !h.cfg.Cloudinary.Enabled() {
		response.Err(w, http.StatusServiceUnavailable, "MEDIA_UNCONFIGURED",
			"Image uploads are not configured", rid)
		return
	}

	var req struct {
		Purpose     string `json:"purpose"      validate:"required,oneof=avatar review_image product_image"`
		ContentType string `json:"content_type" validate:"required,oneof=image/jpeg image/png image/webp"`
		SizeBytes   int64  `json:"size_bytes"   validate:"required,gte=1,lte=10485760"`
	}
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}

	uploadID := uuid.NewString()
	folder := strings.Trim(h.cfg.Cloudinary.Folder, "/") + "/" + req.Purpose
	timestamp := time.Now().Unix()

	// Cloudinary signs the alphabetically-sorted, URL-encoded parameter list
	// (excluding file, api_key and the signature itself) with the API secret
	// appended. Every signed parameter must be replayed verbatim by the client.
	params := map[string]string{
		"folder":    folder,
		"public_id": uploadID,
		"timestamp": fmt.Sprintf("%d", timestamp),
	}
	signature := signParams(params, h.cfg.Cloudinary.APISecret)

	_ = h.cache.SetJSON(r.Context(), fmt.Sprintf("upload:%s", uploadID),
		UploadStatus{UploadID: uploadID, Status: "pending"}, 30*time.Minute)

	response.Ok(w, map[string]any{
		"upload_id":  uploadID,
		"upload_url": fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/upload", h.cfg.Cloudinary.CloudName),
		// Replay these exactly as given, alongside the file itself.
		"fields": map[string]string{
			"api_key":   h.cfg.Cloudinary.APIKey,
			"folder":    folder,
			"public_id": uploadID,
			"timestamp": params["timestamp"],
			"signature": signature,
		},
		"expires_at":     time.Unix(timestamp, 0).Add(signatureTTL),
		"max_size_bytes": 10485760,
	})
}

// signParams builds Cloudinary's signature: the sorted "k=v" pairs joined by
// "&", with the API secret appended, hashed with SHA-1.
func signParams(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(params[k])
	}
	b.WriteString(secret)

	sum := sha1.Sum([]byte(b.String()))
	return hex.EncodeToString(sum[:])
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
