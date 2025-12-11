package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Voyrox/Qube/hub/core/config"
	"github.com/Voyrox/Qube/hub/core/database"
	"github.com/Voyrox/Qube/hub/core/middleware"
	"github.com/Voyrox/Qube/hub/core/models"
	"github.com/gin-gonic/gin"
	"github.com/gocql/gocql"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	mdhtml "github.com/yuin/goldmark/renderer/html"
)

type ImageHandler struct {
	db  *database.ScyllaDB
	cfg *config.Config
}

func NewImageHandler(db *database.ScyllaDB, cfg *config.Config) *ImageHandler {
	return &ImageHandler{db: db, cfg: cfg}
}

func (h *ImageHandler) Upload(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req models.ImageUploadRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Sanitize tag: support comma-separated input; first token is primary
	req.Tag = strings.TrimSpace(req.Tag)
	var extraTags []string
	if strings.Contains(req.Tag, ",") {
		parts := strings.Split(req.Tag, ",")
		primary := strings.TrimSpace(parts[0])
		req.Tag = primary
		for _, p := range parts[1:] {
			t := strings.TrimSpace(p)
			if t != "" && t != primary {
				extraTags = append(extraTags, t)
			}
		}
	}

	{
		var existing models.Image
		if err := h.db.Session().Query(
			`SELECT id, name FROM images WHERE name = ? LIMIT 1 ALLOW FILTERING`,
			req.Name,
		).Scan(&existing.ID, &existing.Name); err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Image name already exists"})
			return
		}
	}

	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is required"})
		return
	}

	// Check file size
	if file.Size > h.cfg.MaxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large"})
		return
	}

	// Validate file extension
	if !strings.HasSuffix(file.Filename, ".tar") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only .tar files are allowed"})
		return
	}

	// Create image record
	imageID := gocql.TimeUUID()
	filename := fmt.Sprintf("%s_%s.tar", req.Name, req.Tag)
	filePath := filepath.Join(h.cfg.StoragePath, imageID.String(), filename)

	// Create directory
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create storage directory"})
		return
	}

	// Save file
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	// Calculate digest (SHA256 of file)
	digest := fmt.Sprintf("sha256:%x", imageID.String()) // Simplified for now

	// Handle optional logo upload
	var logoPath string
	if logoFile, err := c.FormFile("logo"); err == nil {
		logoFilename := fmt.Sprintf("logo_%s.png", imageID.String())
		logoPath = filepath.Join(h.cfg.StoragePath, imageID.String(), logoFilename)
		if err := c.SaveUploadedFile(logoFile, logoPath); err != nil {
			fmt.Printf("Warning: Failed to save logo: %v\n", err)
			logoPath = ""
		}
	}

	now := time.Now()
	// Create database record
	image := models.Image{
		ID:          imageID,
		Name:        req.Name,
		Tag:         req.Tag,
		OwnerID:     userID,
		Description: req.Description,
		Digest:      digest,
		Size:        file.Size,
		Downloads:   0,
		Pulls:       0,
		Stars:       0,
		IsPublic:    req.IsPublic,
		FilePath:    filePath,
		LogoPath:    logoPath,
		CreatedAt:   now,
		UpdatedAt:   now,
		LastUpdated: now,
	}

	if err := h.db.Session().Query(
		`INSERT INTO images (id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, is_public, file_path, logo_path, created_at, updated_at, last_updated) 
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		image.ID, image.Name, image.Tag, image.OwnerID, image.Description, image.Digest,
		image.Size, image.Downloads, image.Pulls, image.Stars, image.IsPublic, image.FilePath, image.LogoPath,
		image.CreatedAt, image.UpdatedAt, image.LastUpdated,
	).Exec(); err != nil {
		os.Remove(filePath) // Clean up file on error
		if logoPath != "" {
			os.Remove(logoPath)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create image record"})
		return
	}

	// Add tag to image_tags table
	if err := h.db.Session().Query(
		`INSERT INTO image_tags (image_id, tag, created_at) VALUES (?, ?, ?)`,
		image.ID, image.Tag, time.Now(),
	).Exec(); err != nil {
		// Non-critical error, just log it
		fmt.Printf("Warning: Failed to add tag: %v\n", err)
	}

	// Insert any extra alias tags
	for _, t := range extraTags {
		_ = h.db.Session().Query(
			`INSERT INTO image_tags (image_id, tag, created_at) VALUES (?, ?, ?)`,
			image.ID, t, time.Now(),
		).Exec()
	}

	c.JSON(http.StatusCreated, image)
}

func (h *ImageHandler) Download(c *gin.Context) {
	name := c.Param("name")
	tag := c.Param("tag")

	var image models.Image
	if err := h.db.Session().Query(
		`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, is_public, file_path, logo_path, created_at, updated_at, last_updated 
		 FROM images WHERE name = ? AND tag = ? LIMIT 1 ALLOW FILTERING`,
		name, tag,
	).Scan(&image.ID, &image.Name, &image.Tag, &image.OwnerID, &image.Description, &image.Digest,
		&image.Size, &image.Downloads, &image.Pulls, &image.Stars, &image.IsPublic, &image.FilePath, &image.LogoPath,
		&image.CreatedAt, &image.UpdatedAt, &image.LastUpdated); err != nil {
		// Fallback: resolve via legacy comma-separated tag or alias in image_tags
		iter := h.db.Session().Query(
			`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, is_public, file_path, logo_path, created_at, updated_at, last_updated 
			 FROM images WHERE name = ? ALLOW FILTERING`,
			name,
		).Iter()
		var cand models.Image
		var found bool
		for iter.Scan(&cand.ID, &cand.Name, &cand.Tag, &cand.OwnerID, &cand.Description, &cand.Digest,
			&cand.Size, &cand.Downloads, &cand.Pulls, &cand.Stars, &cand.IsPublic, &cand.FilePath, &cand.LogoPath,
			&cand.CreatedAt, &cand.UpdatedAt, &cand.LastUpdated) {
			if strings.Contains(cand.Tag, ",") {
				parts := strings.Split(cand.Tag, ",")
				for _, p := range parts {
					if strings.TrimSpace(p) == tag {
						image = cand
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			var alias string
			if err2 := h.db.Session().Query(
				`SELECT tag FROM image_tags WHERE image_id = ? AND tag = ?`,
				cand.ID, tag,
			).Scan(&alias); err2 == nil {
				image = cand
				found = true
				break
			}
		}
		_ = iter.Close()
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
			return
		}
	}

	userID, authenticated := middleware.GetUserID(c)
	if !image.IsPublic && (!authenticated || userID != image.OwnerID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.db.Session().Query(
		`UPDATE image_downloads SET downloads = downloads + 1 WHERE image_id = ?`,
		image.ID,
	).Exec(); err != nil {
		fmt.Printf("Warning: Failed to increment download counter: %v\n", err)
	}

	if err := h.db.Session().Query(
		`UPDATE images SET pulls = ? WHERE id = ?`,
		image.Pulls+1, image.ID,
	).Exec(); err != nil {
		fmt.Printf("Warning: Failed to increment pulls: %v\n", err)
	}

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_%s.tar", image.Name, image.Tag))
	c.Header("Content-Type", "application/x-tar")
	c.File(image.FilePath)
}

func (h *ImageHandler) Logo(c *gin.Context) {
	name := c.Param("name")
	tag := c.Param("tag")

	var image models.Image
	if err := h.db.Session().Query(
		`SELECT id, tag, logo_path FROM images WHERE name = ? AND tag = ? LIMIT 1 ALLOW FILTERING`,
		name, tag,
	).Scan(&image.ID, &image.Tag, &image.LogoPath); err != nil {
		iter := h.db.Session().Query(
			`SELECT id, tag, logo_path FROM images WHERE name = ? ALLOW FILTERING`,
			name,
		).Iter()
		var cand models.Image
		var found bool
		for iter.Scan(&cand.ID, &cand.Tag, &cand.LogoPath) {
			if strings.Contains(cand.Tag, ",") {
				parts := strings.Split(cand.Tag, ",")
				for _, p := range parts {
					if strings.TrimSpace(p) == tag {
						image = cand
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			// Alias lookup in image_tags
			var alias string
			if err2 := h.db.Session().Query(
				`SELECT tag FROM image_tags WHERE image_id = ? AND tag = ?`,
				cand.ID, tag,
			).Scan(&alias); err2 == nil {
				image = cand
				found = true
				break
			}
		}
		_ = iter.Close()
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
			return
		}
	}

	if image.LogoPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Logo not set"})
		return
	}

	// Hint content type for browsers
	c.Header("Content-Type", "image/png")
	c.File(image.LogoPath)
}

func (h *ImageHandler) DownloadLatest(c *gin.Context) {
	name := c.Param("name")

	iter := h.db.Session().Query(
		`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, is_public, file_path, logo_path, created_at, updated_at, last_updated 
		 FROM images WHERE name = ? ALLOW FILTERING`,
		name,
	).Iter()

	var latest models.Image
	var found bool
	var img models.Image
	for iter.Scan(&img.ID, &img.Name, &img.Tag, &img.OwnerID, &img.Description, &img.Digest,
		&img.Size, &img.Downloads, &img.Pulls, &img.Stars, &img.IsPublic, &img.FilePath, &img.LogoPath,
		&img.CreatedAt, &img.UpdatedAt, &img.LastUpdated) {
		if !found || img.LastUpdated.After(latest.LastUpdated) {
			latest = img
			found = true
		}
	}
	if err := iter.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch image tags"})
		return
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
		return
	}

	userID, authenticated := middleware.GetUserID(c)
	if !latest.IsPublic && (!authenticated || userID != latest.OwnerID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.db.Session().Query(
		`UPDATE image_downloads SET downloads = downloads + 1 WHERE image_id = ?`,
		latest.ID,
	).Exec(); err != nil {
		fmt.Printf("Warning: Failed to increment download counter: %v\n", err)
	}

	if err := h.db.Session().Query(
		`UPDATE images SET pulls = ? WHERE id = ?`,
		latest.Pulls+1, latest.ID,
	).Exec(); err != nil {
		fmt.Printf("Warning: Failed to increment pulls: %v\n", err)
	}

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_%s.tar", latest.Name, latest.Tag))
	c.Header("Content-Type", "application/x-tar")
	c.File(latest.FilePath)
}

func (h *ImageHandler) Detail(c *gin.Context) {
	name := c.Param("name")
	tag := c.Param("tag")

	var image models.Image
	if err := h.db.Session().Query(
		`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, is_public, file_path, logo_path, created_at, updated_at, last_updated 
		 FROM images WHERE name = ? AND tag = ? LIMIT 1 ALLOW FILTERING`,
		name, tag,
	).Scan(&image.ID, &image.Name, &image.Tag, &image.OwnerID, &image.Description, &image.Digest,
		&image.Size, &image.Downloads, &image.Pulls, &image.Stars, &image.IsPublic, &image.FilePath, &image.LogoPath,
		&image.CreatedAt, &image.UpdatedAt, &image.LastUpdated); err != nil {
		// Fallback: resolve via legacy comma-separated tags or alias table
		iter := h.db.Session().Query(
			`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, is_public, file_path, logo_path, created_at, updated_at, last_updated 
			 FROM images WHERE name = ? ALLOW FILTERING`,
			name,
		).Iter()
		var cand models.Image
		var found bool
		for iter.Scan(&cand.ID, &cand.Name, &cand.Tag, &cand.OwnerID, &cand.Description, &cand.Digest,
			&cand.Size, &cand.Downloads, &cand.Pulls, &cand.Stars, &cand.IsPublic, &cand.FilePath, &cand.LogoPath,
			&cand.CreatedAt, &cand.UpdatedAt, &cand.LastUpdated) {
			if strings.Contains(cand.Tag, ",") {
				parts := strings.Split(cand.Tag, ",")
				for _, p := range parts {
					if strings.TrimSpace(p) == tag {
						image = cand
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			var alias string
			if err2 := h.db.Session().Query(
				`SELECT tag FROM image_tags WHERE image_id = ? AND tag = ?`,
				cand.ID, tag,
			).Scan(&alias); err2 == nil {
				image = cand
				found = true
				break
			}
		}
		_ = iter.Close()
		if !found {
			c.String(http.StatusNotFound, "Image not found")
			return
		}
	}

	// Optional: get owner username
	var ownerUsername string
	if err := h.db.Session().Query(
		`SELECT username FROM users WHERE id = ? LIMIT 1`, image.OwnerID,
	).Scan(&ownerUsername); err != nil {
		ownerUsername = "user"
	}

	// Check ownership
	userID, authenticated := middleware.GetUserID(c)
	isOwner := authenticated && userID == image.OwnerID

	// Fetch alias tags for this image id (excluding primary)
	aliases := h.getAliases(image.ID, image.Tag)
	// Normalize legacy comma-separated tag strings for display
	if strings.Contains(image.Tag, ",") {
		parts := strings.Split(image.Tag, ",")
		primary := strings.TrimSpace(parts[0])
		extras := []string{}
		for _, p := range parts[1:] {
			t := strings.TrimSpace(p)
			if t != "" {
				extras = append(extras, t)
			}
		}
		// Merge extras into aliases without duplicates
		seen := map[string]bool{primary: true}
		merged := make([]string, 0, len(aliases)+len(extras))
		for _, a := range aliases {
			if a != "" && !seen[a] {
				seen[a] = true
				merged = append(merged, a)
			}
		}
		for _, e := range extras {
			if e != "" && !seen[e] {
				seen[e] = true
				merged = append(merged, e)
			}
		}
		aliases = merged
		image.Tag = primary
	}

	// Render description as sanitized Markdown HTML
	descHTML := renderMarkdown(image.Description)

	c.HTML(http.StatusOK, "image.html", gin.H{
		"title":            fmt.Sprintf("%s/%s", ownerUsername, image.Name),
		"image":            image,
		"owner_username":   ownerUsername,
		"recent_tags":      h.getRecentTags(image.Name, 8),
		"size_formatted":   formatSize(image.Size),
		"is_owner":         isOwner,
		"aliases":          aliases,
		"description_html": descHTML,
	})
}

// DetailLatest finds the latest tag for the name and renders detail
func (h *ImageHandler) DetailLatest(c *gin.Context) {
	name := c.Param("name")
	iter := h.db.Session().Query(
		`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, is_public, file_path, logo_path, created_at, updated_at, last_updated 
		 FROM images WHERE name = ? ALLOW FILTERING`,
		name,
	).Iter()

	var latest models.Image
	var found bool
	var img models.Image
	for iter.Scan(&img.ID, &img.Name, &img.Tag, &img.OwnerID, &img.Description, &img.Digest,
		&img.Size, &img.Downloads, &img.Pulls, &img.Stars, &img.IsPublic, &img.FilePath, &img.LogoPath,
		&img.CreatedAt, &img.UpdatedAt, &img.LastUpdated) {
		if !found || img.LastUpdated.After(latest.LastUpdated) {
			latest = img
			found = true
		}
	}
	if err := iter.Close(); err != nil || !found {
		c.String(http.StatusNotFound, "Image not found")
		return
	}

	var ownerUsername string
	if err := h.db.Session().Query(
		`SELECT username FROM users WHERE id = ? LIMIT 1`, latest.OwnerID,
	).Scan(&ownerUsername); err != nil {
		ownerUsername = "user"
	}

	userID, authenticated := middleware.GetUserID(c)
	isOwner := authenticated && userID == latest.OwnerID

	// Fetch alias tags for this image id (excluding primary)
	aliases := h.getAliases(latest.ID, latest.Tag)
	// Normalize legacy comma-separated tag strings for display
	if strings.Contains(latest.Tag, ",") {
		parts := strings.Split(latest.Tag, ",")
		primary := strings.TrimSpace(parts[0])
		extras := []string{}
		for _, p := range parts[1:] {
			t := strings.TrimSpace(p)
			if t != "" {
				extras = append(extras, t)
			}
		}
		seen := map[string]bool{primary: true}
		merged := make([]string, 0, len(aliases)+len(extras))
		for _, a := range aliases {
			if a != "" && !seen[a] {
				seen[a] = true
				merged = append(merged, a)
			}
		}
		for _, e := range extras {
			if e != "" && !seen[e] {
				seen[e] = true
				merged = append(merged, e)
			}
		}
		aliases = merged
		latest.Tag = primary
	}

	// Render description as sanitized Markdown HTML
	descHTML := renderMarkdown(latest.Description)

	c.HTML(http.StatusOK, "image.html", gin.H{
		"title":            fmt.Sprintf("%s/%s", ownerUsername, latest.Name),
		"image":            latest,
		"owner_username":   ownerUsername,
		"recent_tags":      h.getRecentTags(latest.Name, 8),
		"size_formatted":   formatSize(latest.Size),
		"is_owner":         isOwner,
		"aliases":          aliases,
		"description_html": descHTML,
	})
}

// getAliases returns alias tags for an image id excluding the primary
func (h *ImageHandler) getAliases(imageID gocql.UUID, primary string) []string {
	iter := h.db.Session().Query(
		`SELECT tag FROM image_tags WHERE image_id = ?`,
		imageID,
	).Iter()
	var t string
	var out []string
	seen := map[string]bool{}
	seen[primary] = true
	for iter.Scan(&t) {
		if t != "" && !seen[t] {
			out = append(out, t)
			seen[t] = true
		}
	}
	_ = iter.Close()
	return out
}

// EditForm renders the edit form for an image (owner only)
func (h *ImageHandler) EditForm(c *gin.Context) {
	name := c.Param("name")
	tag := c.Param("tag")

	// Require auth
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}

	var image models.Image
	if err := h.db.Session().Query(
		`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, is_public, file_path, logo_path, created_at, updated_at, last_updated 
		 FROM images WHERE name = ? AND tag = ? LIMIT 1 ALLOW FILTERING`,
		name, tag,
	).Scan(&image.ID, &image.Name, &image.Tag, &image.OwnerID, &image.Description, &image.Digest,
		&image.Size, &image.Downloads, &image.Pulls, &image.Stars, &image.IsPublic, &image.FilePath, &image.LogoPath,
		&image.CreatedAt, &image.UpdatedAt, &image.LastUpdated); err != nil {
		c.String(http.StatusNotFound, "Image not found")
		return
	}

	if image.OwnerID != userID {
		c.String(http.StatusForbidden, "Access denied")
		return
	}

	// Optional: get owner username
	var ownerUsername string
	if err := h.db.Session().Query(
		`SELECT username FROM users WHERE id = ? LIMIT 1`, image.OwnerID,
	).Scan(&ownerUsername); err != nil {
		ownerUsername = "user"
	}

	c.HTML(http.StatusOK, "image_edit.html", gin.H{
		"title":          fmt.Sprintf("Edit %s/%s:%s", ownerUsername, image.Name, image.Tag),
		"image":          image,
		"owner_username": ownerUsername,
	})
}

// UpdateImage updates editable fields (owner only)
func (h *ImageHandler) UpdateImage(c *gin.Context) {
	name := c.Param("name")
	tag := c.Param("tag")

	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var image models.Image
	if err := h.db.Session().Query(
		`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, is_public, file_path, logo_path, created_at, updated_at, last_updated 
		 FROM images WHERE name = ? AND tag = ? LIMIT 1 ALLOW FILTERING`,
		name, tag,
	).Scan(&image.ID, &image.Name, &image.Tag, &image.OwnerID, &image.Description, &image.Digest,
		&image.Size, &image.Downloads, &image.Pulls, &image.Stars, &image.IsPublic, &image.FilePath, &image.LogoPath,
		&image.CreatedAt, &image.UpdatedAt, &image.LastUpdated); err != nil {
		// Fallback: resolve legacy comma-separated tags or alias tags
		iter := h.db.Session().Query(
			`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, is_public, file_path, logo_path, created_at, updated_at, last_updated 
			 FROM images WHERE name = ? ALLOW FILTERING`,
			name,
		).Iter()
		var cand models.Image
		var found bool
		for iter.Scan(&cand.ID, &cand.Name, &cand.Tag, &cand.OwnerID, &cand.Description, &cand.Digest,
			&cand.Size, &cand.Downloads, &cand.Pulls, &cand.Stars, &cand.IsPublic, &cand.FilePath, &cand.LogoPath,
			&cand.CreatedAt, &cand.UpdatedAt, &cand.LastUpdated) {
			// legacy comma tag match
			if strings.Contains(cand.Tag, ",") {
				parts := strings.Split(cand.Tag, ",")
				for _, p := range parts {
					if strings.TrimSpace(p) == tag {
						image = cand
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			// alias table match
			var alias string
			if err2 := h.db.Session().Query(
				`SELECT tag FROM image_tags WHERE image_id = ? AND tag = ?`,
				cand.ID, tag,
			).Scan(&alias); err2 == nil {
				image = cand
				found = true
				break
			}
		}
		_ = iter.Close()
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
			return
		}
	}

	if image.OwnerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Bind input (description, is_public; optional new logo)
	var req struct {
		Description string `form:"description" json:"description"`
		IsPublic    *bool  `form:"is_public" json:"is_public"`
		NewTag      string `form:"new_tag" json:"new_tag"`
		Tags        string `form:"tags" json:"tags"`
	}
	// Accept multipart or JSON
	if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/") {
		_ = c.ShouldBind(&req)
	} else {
		_ = c.ShouldBindJSON(&req)
	}

	// Apply changes
	if req.Description != "" {
		image.Description = req.Description
	}
	if req.IsPublic != nil {
		image.IsPublic = *req.IsPublic
	}

	// Handle tag change
	if req.NewTag != "" && req.NewTag != image.Tag {
		// Check for conflict
		var tmp models.Image
		if err := h.db.Session().Query(
			`SELECT id FROM images WHERE name = ? AND tag = ? LIMIT 1 ALLOW FILTERING`,
			image.Name, req.NewTag,
		).Scan(&tmp.ID); err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Tag already exists for this image name"})
			return
		}
		// Rename file on disk to reflect new tag if present
		if image.FilePath != "" {
			dir := filepath.Dir(image.FilePath)
			newFile := filepath.Join(dir, fmt.Sprintf("%s_%s.tar", image.Name, req.NewTag))
			if err := os.Rename(image.FilePath, newFile); err == nil {
				image.FilePath = newFile
			}
		}
		// Update tag
		oldTag := image.Tag
		image.Tag = req.NewTag
		// Update tag table: add new, remove old (best-effort)
		_ = h.db.Session().Query(
			`INSERT INTO image_tags (image_id, tag, created_at) VALUES (?, ?, ?)`,
			image.ID, image.Tag, time.Now(),
		).Exec()
		_ = h.db.Session().Query(
			`DELETE FROM image_tags WHERE image_id = ? AND tag = ?`,
			image.ID, oldTag,
		).Exec()
	}

	// Handle optional logo upload
	if logoFile, err := c.FormFile("logo"); err == nil {
		logoFilename := fmt.Sprintf("logo_%s.png", image.ID.String())
		logoPath := filepath.Join(h.cfg.StoragePath, image.ID.String(), logoFilename)
		if err := c.SaveUploadedFile(logoFile, logoPath); err == nil {
			image.LogoPath = logoPath
		}
	}

	// Add any additional tags provided (comma-separated)
	if strings.TrimSpace(req.Tags) != "" {
		parts := strings.Split(req.Tags, ",")
		seen := map[string]bool{}
		for _, p := range parts {
			t := strings.TrimSpace(p)
			if t == "" || seen[t] {
				continue
			}
			seen[t] = true
			if t == image.Tag {
				continue
			}
			_ = h.db.Session().Query(
				`INSERT INTO image_tags (image_id, tag, created_at) VALUES (?, ?, ?)`,
				image.ID, t, time.Now(),
			).Exec()
		}
	}

	image.UpdatedAt = time.Now()
	image.LastUpdated = image.UpdatedAt

	if err := h.db.Session().Query(
		`UPDATE images SET description = ?, is_public = ?, logo_path = ?, file_path = ?, tag = ?, updated_at = ?, last_updated = ? WHERE id = ?`,
		image.Description, image.IsPublic, image.LogoPath, image.FilePath, image.Tag, image.UpdatedAt, image.LastUpdated, image.ID,
	).Exec(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update image"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Image updated", "image": image})
}

// getRecentTags returns a slice of recent tag strings for an image name
func (h *ImageHandler) getRecentTags(name string, limit int) []string {
	iter := h.db.Session().Query(
		`SELECT tag, last_updated FROM images WHERE name = ? LIMIT ? ALLOW FILTERING`,
		name, limit,
	).Iter()

	var tag string
	var last time.Time
	var tags []struct {
		Tag  string
		Last time.Time
	}
	for iter.Scan(&tag, &last) {
		tags = append(tags, struct {
			Tag  string
			Last time.Time
		}{Tag: tag, Last: last})
	}
	_ = iter.Close()

	// sort by last_updated desc
	sort.Slice(tags, func(i, j int) bool { return tags[i].Last.After(tags[j].Last) })

	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, t := range tags {
		ts := t.Tag
		if strings.Contains(ts, ",") {
			parts := strings.Split(ts, ",")
			for _, p := range parts {
				v := strings.TrimSpace(p)
				if v == "" || seen[v] {
					continue
				}
				seen[v] = true
				out = append(out, v)
				if limit > 0 && len(out) >= limit {
					return out
				}
			}
		} else {
			v := strings.TrimSpace(ts)
			if v != "" && !seen[v] {
				seen[v] = true
				out = append(out, v)
				if limit > 0 && len(out) >= limit {
					return out
				}
			}
		}
	}
	return out
}

// formatSize converts bytes to a human-readable string (KB/MB/GB)
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	if bytes >= GB {
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	}
	if bytes >= MB {
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	}
	if bytes >= KB {
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	}
	return fmt.Sprintf("%d B", bytes)
}

func (h *ImageHandler) List(c *gin.Context) {
	query := c.Query("query")
	limit := 20

	var images []models.Image

	if query != "" {
		// Search by name
		iter := h.db.Session().Query(
			`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, is_public, file_path, logo_path, created_at, updated_at, last_updated 
			 FROM images WHERE name = ? LIMIT ? ALLOW FILTERING`,
			query, limit,
		).Iter()

		var image models.Image
		for iter.Scan(&image.ID, &image.Name, &image.Tag, &image.OwnerID, &image.Description, &image.Digest,
			&image.Size, &image.Downloads, &image.Pulls, &image.Stars, &image.IsPublic, &image.FilePath, &image.LogoPath,
			&image.CreatedAt, &image.UpdatedAt, &image.LastUpdated) {
			if image.IsPublic {
				images = append(images, image)
			}
		}

		if err := iter.Close(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch images"})
			return
		}
	} else {
		// List all public images
		iter := h.db.Session().Query(
			`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, is_public, file_path, logo_path, created_at, updated_at, last_updated 
			 FROM images LIMIT ? ALLOW FILTERING`,
			limit,
		).Iter()

		var image models.Image
		for iter.Scan(&image.ID, &image.Name, &image.Tag, &image.OwnerID, &image.Description, &image.Digest,
			&image.Size, &image.Downloads, &image.Pulls, &image.Stars, &image.IsPublic, &image.FilePath, &image.LogoPath,
			&image.CreatedAt, &image.UpdatedAt, &image.LastUpdated) {
			if image.IsPublic {
				images = append(images, image)
			}
		}

		if err := iter.Close(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch images"})
			return
		}
	}

	// Enrich with owner_username for frontend display
	enriched := make([]gin.H, 0, len(images))
	for _, img := range images {
		var ownerUsername string
		if err := h.db.Session().Query(
			`SELECT username FROM users WHERE id = ? LIMIT 1`, img.OwnerID,
		).Scan(&ownerUsername); err != nil {
			ownerUsername = "user"
		}
		logoURL := ""
		if img.LogoPath != "" {
			logoURL = "/api/images/" + img.Name + "/" + img.Tag + "/logo"
		}
		enriched = append(enriched, gin.H{
			"id":             img.ID,
			"name":           img.Name,
			"tag":            img.Tag,
			"owner_id":       img.OwnerID,
			"owner_username": ownerUsername,
			"description":    img.Description,
			"digest":         img.Digest,
			"size":           img.Size,
			"downloads":      img.Downloads,
			"pulls":          img.Pulls,
			"stars":          img.Stars,
			"is_public":      img.IsPublic,
			"logo_path":      logoURL,
			"created_at":     img.CreatedAt,
			"updated_at":     img.UpdatedAt,
			"last_updated":   img.LastUpdated,
		})
	}

	c.JSON(http.StatusOK, gin.H{"images": enriched})
}

func (h *ImageHandler) GetByName(c *gin.Context) {
	name := c.Param("name")

	var images []models.Image
	iter := h.db.Session().Query(
		`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, is_public, file_path, logo_path, created_at, updated_at, last_updated 
		 FROM images WHERE name = ? ALLOW FILTERING`,
		name,
	).Iter()

	var image models.Image
	for iter.Scan(&image.ID, &image.Name, &image.Tag, &image.OwnerID, &image.Description, &image.Digest,
		&image.Size, &image.Downloads, &image.Pulls, &image.Stars, &image.IsPublic, &image.FilePath, &image.LogoPath,
		&image.CreatedAt, &image.UpdatedAt, &image.LastUpdated) {
		if image.IsPublic {
			images = append(images, image)
		}
	}

	if err := iter.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch images"})
		return
	}

	// Enrich with owner_username for frontend display
	enriched := make([]gin.H, 0, len(images))
	for _, img := range images {
		var ownerUsername string
		if err := h.db.Session().Query(
			`SELECT username FROM users WHERE id = ? LIMIT 1`, img.OwnerID,
		).Scan(&ownerUsername); err != nil {
			ownerUsername = "user"
		}
		logoURL := ""
		if img.LogoPath != "" {
			logoURL = "/api/images/" + img.Name + "/" + img.Tag + "/logo"
		}
		enriched = append(enriched, gin.H{
			"id":             img.ID,
			"name":           img.Name,
			"tag":            img.Tag,
			"owner_id":       img.OwnerID,
			"owner_username": ownerUsername,
			"description":    img.Description,
			"digest":         img.Digest,
			"size":           img.Size,
			"downloads":      img.Downloads,
			"pulls":          img.Pulls,
			"stars":          img.Stars,
			"is_public":      img.IsPublic,
			"logo_path":      logoURL,
			"created_at":     img.CreatedAt,
			"updated_at":     img.UpdatedAt,
			"last_updated":   img.LastUpdated,
		})
	}

	c.JSON(http.StatusOK, gin.H{"images": enriched})
}

func (h *ImageHandler) Delete(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	imageIDStr := c.Param("id")
	imageID, err := gocql.ParseUUID(imageIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image ID"})
		return
	}

	// Get image details
	var image models.Image
	if err := h.db.Session().Query(
		`SELECT id, name, tag, owner_id, file_path, logo_path FROM images WHERE id = ?`,
		imageID,
	).Scan(&image.ID, &image.Name, &image.Tag, &image.OwnerID, &image.FilePath, &image.LogoPath); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
		return
	}

	// Check ownership
	if image.OwnerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Delete file
	if err := os.Remove(image.FilePath); err != nil {
		fmt.Printf("Warning: Failed to delete file: %v\n", err)
	}

	// Delete logo if exists
	if image.LogoPath != "" {
		if err := os.Remove(image.LogoPath); err != nil {
			fmt.Printf("Warning: Failed to delete logo: %v\n", err)
		}
	}

	// Delete directory if empty
	os.Remove(filepath.Dir(image.FilePath))

	// Delete from database
	if err := h.db.Session().Query(`DELETE FROM images WHERE id = ?`, imageID).Exec(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete image"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Image deleted successfully"})
}

func (h *ImageHandler) GetMyImages(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var images []models.Image
	iter := h.db.Session().Query(
		`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, is_public, file_path, logo_path, created_at, updated_at, last_updated 
		 FROM images WHERE owner_id = ? ALLOW FILTERING`,
		userID,
	).Iter()

	var image models.Image
	for iter.Scan(&image.ID, &image.Name, &image.Tag, &image.OwnerID, &image.Description, &image.Digest,
		&image.Size, &image.Downloads, &image.Pulls, &image.Stars, &image.IsPublic, &image.FilePath, &image.LogoPath,
		&image.CreatedAt, &image.UpdatedAt, &image.LastUpdated) {
		images = append(images, image)
	}

	if err := iter.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch images"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"images": images})
}

func (h *ImageHandler) Star(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	imageIDStr := c.Param("id")
	imageID, err := gocql.ParseUUID(imageIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image ID"})
		return
	}

	// Check if already starred
	var existingUserID gocql.UUID
	if err := h.db.Session().Query(
		`SELECT user_id FROM stars WHERE user_id = ? AND image_id = ?`,
		userID, imageID,
	).Scan(&existingUserID); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Already starred"})
		return
	}

	// Add star
	if err := h.db.Session().Query(
		`INSERT INTO stars (user_id, image_id, created_at) VALUES (?, ?, ?)`,
		userID, imageID, time.Now(),
	).Exec(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to star image"})
		return
	}

	// Increment star counter
	if err := h.db.Session().Query(
		`UPDATE images SET stars = stars + 1 WHERE id = ?`,
		imageID,
	).Exec(); err != nil {
		fmt.Printf("Warning: Failed to increment star counter: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Image starred successfully"})
}

func (h *ImageHandler) Unstar(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	imageIDStr := c.Param("id")
	imageID, err := gocql.ParseUUID(imageIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image ID"})
		return
	}

	// Remove star
	if err := h.db.Session().Query(
		`DELETE FROM stars WHERE user_id = ? AND image_id = ?`,
		userID, imageID,
	).Exec(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unstar image"})
		return
	}

	// Decrement star counter
	if err := h.db.Session().Query(
		`UPDATE images SET stars = stars - 1 WHERE id = ?`,
		imageID,
	).Exec(); err != nil {
		fmt.Printf("Warning: Failed to decrement star counter: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Image unstarred successfully"})
}

func (h *ImageHandler) DownloadFile(c *gin.Context) {
	filename := c.Param("filename")

	// Serve file from storage
	filePath := filepath.Join(h.cfg.StoragePath, filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Open and serve file
	file, err := os.Open(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer file.Close()

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/x-tar")

	io.Copy(c.Writer, file)
}

// renderMarkdown converts user-provided Markdown to sanitized HTML for safe display
func renderMarkdown(s string) template.HTML {
	if strings.TrimSpace(s) == "" {
		return template.HTML("")
	}
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(
			mdhtml.WithHardWraps(),
			mdhtml.WithUnsafe(), // needed to emit code/pre tags before sanitization
		),
	)
	var buf bytes.Buffer
	if err := md.Convert([]byte(s), &buf); err != nil {
		// Fallback to escaped text if parsing fails
		return template.HTML(template.HTMLEscapeString(s))
	}
	// Sanitize HTML output for UGC
	policy := bluemonday.UGCPolicy()
	// Allow common code-related tags
	policy.AllowElements("pre", "code")
	safe := policy.SanitizeBytes(buf.Bytes())
	return template.HTML(safe)
}
