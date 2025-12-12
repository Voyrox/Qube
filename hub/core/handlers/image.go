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
	"strconv"
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

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is required"})
		return
	}

	if file.Size > h.cfg.MaxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large"})
		return
	}

	if !strings.HasSuffix(file.Filename, ".tar") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only .tar files are allowed"})
		return
	}

	imageID := gocql.TimeUUID()
	filename := fmt.Sprintf("%s_%s.tar", req.Name, req.Tag)
	filePath := filepath.Join(h.cfg.StoragePath, imageID.String(), filename)

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create storage directory"})
		return
	}

	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	digest := fmt.Sprintf("sha256:%x", imageID.String()) // Simplified for now

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
		Category:    req.Category,
		IsPublic:    req.IsPublic,
		FilePath:    filePath,
		LogoPath:    logoPath,
		CreatedAt:   now,
		UpdatedAt:   now,
		LastUpdated: now,
	}

	if err := h.db.Session().Query(
		`INSERT INTO images (id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, category, is_public, file_path, logo_path, created_at, updated_at, last_updated) 
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		image.ID, image.Name, image.Tag, image.OwnerID, image.Description, image.Digest,
		image.Size, image.Downloads, image.Pulls, image.Stars, image.Category, image.IsPublic, image.FilePath, image.LogoPath,
		image.CreatedAt, image.UpdatedAt, image.LastUpdated,
	).Exec(); err != nil {
		os.Remove(filePath) // Clean up file on error
		if logoPath != "" {
			os.Remove(logoPath)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create image record"})
		return
	}

	if err := h.db.Session().Query(
		`INSERT INTO image_tags (image_id, tag, created_at) VALUES (?, ?, ?)`,
		image.ID, image.Tag, time.Now(),
	).Exec(); err != nil {
		fmt.Printf("Warning: Failed to insert primary tag: %v\n", err)
	}
	for _, t := range extraTags {
		if err := h.db.Session().Query(
			`INSERT INTO image_tags (image_id, tag, created_at) VALUES (?, ?, ?)`,
			image.ID, t, time.Now(),
		).Exec(); err != nil {
			fmt.Printf("Warning: Failed to insert extra tag '%s': %v\n", t, err)
		}
	}

	c.JSON(http.StatusCreated, image)
}

func (h *ImageHandler) Download(c *gin.Context) {
	name := c.Param("name")
	tag := c.Param("tag")

	var image models.Image
	if err := h.db.Session().Query(
		`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, category, is_public, file_path, logo_path, created_at, updated_at, last_updated 
		 FROM images WHERE name = ? AND tag = ? LIMIT 1 ALLOW FILTERING`,
		name, tag,
	).Scan(&image.ID, &image.Name, &image.Tag, &image.OwnerID, &image.Description, &image.Digest,
		&image.Size, &image.Downloads, &image.Pulls, &image.Stars, &image.Category, &image.IsPublic, &image.FilePath, &image.LogoPath,
		&image.CreatedAt, &image.UpdatedAt, &image.LastUpdated); err != nil {

		iter := h.db.Session().Query(
			`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, category, is_public, file_path, logo_path, created_at, updated_at, last_updated FROM images WHERE name = ? ALLOW FILTERING`,
			name,
		).Iter()
		var cand models.Image
		var found bool
		for iter.Scan(&cand.ID, &cand.Name, &cand.Tag, &cand.OwnerID, &cand.Description, &cand.Digest,
			&cand.Size, &cand.Downloads, &cand.Pulls, &cand.Stars, &cand.Category, &cand.IsPublic, &cand.FilePath, &cand.LogoPath,
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
		`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, category, is_public, file_path, logo_path, created_at, updated_at, last_updated 
		 FROM images WHERE name = ? ALLOW FILTERING`,
		name,
	).Iter()

	var latest models.Image
	var found bool
	var img models.Image
	for iter.Scan(&img.ID, &img.Name, &img.Tag, &img.OwnerID, &img.Description, &img.Digest,
		&img.Size, &img.Downloads, &img.Pulls, &img.Stars, &img.Category, &img.IsPublic, &img.FilePath, &img.LogoPath,
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
		`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, category, is_public, file_path, logo_path, created_at, updated_at, last_updated 
		 FROM images WHERE name = ? AND tag = ? LIMIT 1 ALLOW FILTERING`,
		name, tag,
	).Scan(&image.ID, &image.Name, &image.Tag, &image.OwnerID, &image.Description, &image.Digest,
		&image.Size, &image.Downloads, &image.Pulls, &image.Stars, &image.Category, &image.IsPublic, &image.FilePath, &image.LogoPath,
		&image.CreatedAt, &image.UpdatedAt, &image.LastUpdated); err != nil {

		iter := h.db.Session().Query(
			`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, category, is_public, file_path, logo_path, created_at, updated_at, last_updated 
			 FROM images WHERE name = ? ALLOW FILTERING`,
			name,
		).Iter()
		var cand models.Image
		var found bool
		for iter.Scan(&cand.ID, &cand.Name, &cand.Tag, &cand.OwnerID, &cand.Description, &cand.Digest,
			&cand.Size, &cand.Downloads, &cand.Pulls, &cand.Stars, &cand.Category, &cand.IsPublic, &cand.FilePath, &cand.LogoPath,
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

	var ownerUsername string
	if err := h.db.Session().Query(
		`SELECT username FROM users WHERE id = ? LIMIT 1`, image.OwnerID,
	).Scan(&ownerUsername); err != nil {
		ownerUsername = "user"
	}

	userID, authenticated := middleware.GetUserID(c)
	isOwner := authenticated && userID == image.OwnerID

	aliases := h.getAliases(image.ID, image.Tag)
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

	descHTML := renderMarkdown(image.Description)

	isStarred := false
	if authenticated {
		var tmp gocql.UUID
		if err := h.db.Session().Query(
			`SELECT user_id FROM stars WHERE user_id = ? AND image_id = ?`,
			userID, image.ID,
		).Scan(&tmp); err == nil {
			isStarred = true
		}
	}

	c.HTML(http.StatusOK, "image.html", gin.H{
		"title":            fmt.Sprintf("%s/%s", ownerUsername, image.Name),
		"image":            image,
		"owner_username":   ownerUsername,
		"recent_tags":      h.getRecentTags(image.Name, 8),
		"all_versions":     h.getAllVersions(image.Name),
		"size_formatted":   formatSize(image.Size),
		"is_owner":         isOwner,
		"is_starred":       isStarred,
		"aliases":          aliases,
		"description_html": descHTML,
	})
}

func (h *ImageHandler) DetailLatest(c *gin.Context) {
	name := c.Param("name")
	iter := h.db.Session().Query(
		`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, category, is_public, file_path, logo_path, created_at, updated_at, last_updated 
		 FROM images WHERE name = ? ALLOW FILTERING`,
		name,
	).Iter()

	var latest models.Image
	var found bool
	var img models.Image
	for iter.Scan(&img.ID, &img.Name, &img.Tag, &img.OwnerID, &img.Description, &img.Digest,
		&img.Size, &img.Downloads, &img.Pulls, &img.Stars, &img.Category, &img.IsPublic, &img.FilePath, &img.LogoPath,
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

	aliases := h.getAliases(latest.ID, latest.Tag)
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

	descHTML := renderMarkdown(latest.Description)

	// Check if current user has starred this image
	isStarred := false
	if authenticated {
		var tmp gocql.UUID
		if err := h.db.Session().Query(
			`SELECT user_id FROM stars WHERE user_id = ? AND image_id = ?`,
			userID, latest.ID,
		).Scan(&tmp); err == nil {
			isStarred = true
		}
	}

	c.HTML(http.StatusOK, "image.html", gin.H{
		"title":            fmt.Sprintf("%s/%s", ownerUsername, latest.Name),
		"image":            latest,
		"owner_username":   ownerUsername,
		"recent_tags":      h.getRecentTags(latest.Name, 8),
		"size_formatted":   formatSize(latest.Size),
		"is_owner":         isOwner,
		"is_starred":       isStarred,
		"aliases":          aliases,
		"description_html": descHTML,
	})
}

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

func (h *ImageHandler) EditForm(c *gin.Context) {
	name := c.Param("name")
	tag := c.Param("tag")

	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}

	var image models.Image
	if err := h.db.Session().Query(
		`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, category, is_public, file_path, logo_path, created_at, updated_at, last_updated 
		 FROM images WHERE name = ? AND tag = ? LIMIT 1 ALLOW FILTERING`,
		name, tag,
	).Scan(&image.ID, &image.Name, &image.Tag, &image.OwnerID, &image.Description, &image.Digest,
		&image.Size, &image.Downloads, &image.Pulls, &image.Stars, &image.Category, &image.IsPublic, &image.FilePath, &image.LogoPath,
		&image.CreatedAt, &image.UpdatedAt, &image.LastUpdated); err != nil {
		c.String(http.StatusNotFound, "Image not found")
		return
	}

	if image.OwnerID != userID {
		c.String(http.StatusForbidden, "Access denied")
		return
	}

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
		`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, category, is_public, file_path, logo_path, created_at, updated_at, last_updated 
		 FROM images WHERE name = ? AND tag = ? LIMIT 1 ALLOW FILTERING`,
		name, tag,
	).Scan(&image.ID, &image.Name, &image.Tag, &image.OwnerID, &image.Description, &image.Digest,
		&image.Size, &image.Downloads, &image.Pulls, &image.Stars, &image.Category, &image.IsPublic, &image.FilePath, &image.LogoPath,
		&image.CreatedAt, &image.UpdatedAt, &image.LastUpdated); err != nil {

		iter := h.db.Session().Query(
			`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, category, is_public, file_path, logo_path, created_at, updated_at, last_updated 
			 FROM images WHERE name = ? ALLOW FILTERING`,
			name,
		).Iter()
		var cand models.Image
		var found bool
		for iter.Scan(&cand.ID, &cand.Name, &cand.Tag, &cand.OwnerID, &cand.Description, &cand.Digest,
			&cand.Size, &cand.Downloads, &cand.Pulls, &cand.Stars, &cand.Category, &cand.IsPublic, &cand.FilePath, &cand.LogoPath,
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

	if image.OwnerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	var req struct {
		Description   string `form:"description" json:"description"`
		Category      string `form:"category" json:"category"`
		IsPublic      *bool  `form:"is_public" json:"is_public"`
		NewTag        string `form:"new_tag" json:"new_tag"`
		Tags          string `form:"tags" json:"tags"`
		CreateVersion string `form:"create_version" json:"create_version"`
	}
	if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/") {
		_ = c.ShouldBind(&req)
	} else {
		_ = c.ShouldBindJSON(&req)
	}

	// Handle creating a new version (separate image record)
	if req.CreateVersion == "true" && req.NewTag != "" {
		// Check if version already exists
		var tmp models.Image
		if err := h.db.Session().Query(
			`SELECT id FROM images WHERE name = ? AND tag = ? LIMIT 1 ALLOW FILTERING`,
			image.Name, req.NewTag,
		).Scan(&tmp.ID); err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Version already exists"})
			return
		}

		// Get the uploaded file for the new version
		newFile, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "File is required for new version"})
			return
		}

		if !strings.HasSuffix(newFile.Filename, ".tar") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Only .tar files are allowed"})
			return
		}

		// Create a NEW image record with new ID, but store in SAME folder as parent
		newImageID := gocql.TimeUUID()
		newFilename := fmt.Sprintf("%s_%s.tar", image.Name, req.NewTag)
		// Use the SAME storage folder as the parent image
		parentStorageDir := filepath.Dir(image.FilePath)
		newFilePath := filepath.Join(parentStorageDir, newFilename)

		if err := os.MkdirAll(filepath.Dir(newFilePath), 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create storage directory"})
			return
		}

		if err := c.SaveUploadedFile(newFile, newFilePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
			return
		}

		// Copy logo from old version if no new logo uploaded
		var newLogoPath string
		if logoFile, err := c.FormFile("logo"); err == nil {
			logoFilename := fmt.Sprintf("logo_%s.png", newImageID.String())
			newLogoPath = filepath.Join(parentStorageDir, logoFilename)
			_ = c.SaveUploadedFile(logoFile, newLogoPath)
		} else if image.LogoPath != "" {
			// Use the same logo as parent (no need to copy)
			newLogoPath = image.LogoPath
		}

		// Get file size
		fileInfo, _ := os.Stat(newFilePath)
		fileSize := int64(0)
		if fileInfo != nil {
			fileSize = fileInfo.Size()
		}

		now := time.Now()
		newImage := models.Image{
			ID:          newImageID,
			Name:        image.Name,
			Tag:         req.NewTag,
			OwnerID:     image.OwnerID,
			Description: image.Description, // Inherit from parent
			Category:    image.Category,    // Inherit from parent
			IsPublic:    image.IsPublic,
			FilePath:    newFilePath,
			LogoPath:    newLogoPath,
			Size:        fileSize,
			CreatedAt:   now,
			UpdatedAt:   now,
			LastUpdated: now,
		}

		// Insert new version as separate image
		if err := h.db.Session().Query(
			`INSERT INTO images (id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, category, is_public, file_path, logo_path, created_at, updated_at, last_updated) 
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newImage.ID, newImage.Name, newImage.Tag, newImage.OwnerID, newImage.Description, "",
			newImage.Size, 0, 0, 0, newImage.Category, newImage.IsPublic, newImage.FilePath, newImage.LogoPath,
			newImage.CreatedAt, newImage.UpdatedAt, newImage.LastUpdated,
		).Exec(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create new version"})
			return
		}

		// Add tag entry
		_ = h.db.Session().Query(
			`INSERT INTO image_tags (image_id, tag, created_at) VALUES (?, ?, ?)`,
			newImage.ID, newImage.Tag, now,
		).Exec()

		c.JSON(http.StatusOK, gin.H{
			"message": "New version created successfully",
			"image":   newImage,
		})
		return
	}

	if req.Description != "" {
		image.Description = req.Description
	}
	if req.Category != "" {
		image.Category = req.Category
	}
	if req.IsPublic != nil {
		image.IsPublic = *req.IsPublic
	}

	if req.NewTag != "" && req.NewTag != image.Tag {
		var tmp models.Image
		if err := h.db.Session().Query(
			`SELECT id FROM images WHERE name = ? AND tag = ? LIMIT 1 ALLOW FILTERING`,
			image.Name, req.NewTag,
		).Scan(&tmp.ID); err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Tag already exists for this image name"})
			return
		}

		if image.FilePath != "" {
			dir := filepath.Dir(image.FilePath)
			newFile := filepath.Join(dir, fmt.Sprintf("%s_%s.tar", image.Name, req.NewTag))
			if err := os.Rename(image.FilePath, newFile); err == nil {
				image.FilePath = newFile
			}
		}

		oldTag := image.Tag
		image.Tag = req.NewTag

		_ = h.db.Session().Query(
			`INSERT INTO image_tags (image_id, tag, created_at) VALUES (?, ?, ?)`,
			image.ID, image.Tag, time.Now(),
		).Exec()
		_ = h.db.Session().Query(
			`DELETE FROM image_tags WHERE image_id = ? AND tag = ?`,
			image.ID, oldTag,
		).Exec()
	}

	if logoFile, err := c.FormFile("logo"); err == nil {
		logoFilename := fmt.Sprintf("logo_%s.png", image.ID.String())
		logoPath := filepath.Join(h.cfg.StoragePath, image.ID.String(), logoFilename)
		if err := c.SaveUploadedFile(logoFile, logoPath); err == nil {
			image.LogoPath = logoPath
		}
	}

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
		`UPDATE images SET description = ?, category = ?, is_public = ?, logo_path = ?, file_path = ?, tag = ?, updated_at = ?, last_updated = ? WHERE id = ?`,
		image.Description, image.Category, image.IsPublic, image.LogoPath, image.FilePath, image.Tag, image.UpdatedAt, image.LastUpdated, image.ID,
	).Exec(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update image"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Image updated", "image": image})
}

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

func (h *ImageHandler) getAllVersions(name string) []string {
	iter := h.db.Session().Query(
		`SELECT tag, last_updated FROM images WHERE name = ? ALLOW FILTERING`,
		name,
	).Iter()

	var tag string
	var last time.Time
	var versions []struct {
		Tag  string
		Last time.Time
	}
	for iter.Scan(&tag, &last) {
		// Only include valid version numbers (numbers and dots)
		tag = strings.TrimSpace(tag)
		if tag != "" && strings.ContainsAny(tag, "0123456789") {
			// Simple check: if it looks like a version number
			isVersion := true
			for _, ch := range tag {
				if ch != '.' && (ch < '0' || ch > '9') {
					isVersion = false
					break
				}
			}
			if isVersion {
				versions = append(versions, struct {
					Tag  string
					Last time.Time
				}{Tag: tag, Last: last})
			}
		}
	}
	_ = iter.Close()

	// Sort by last updated (newest first)
	sort.Slice(versions, func(i, j int) bool { return versions[i].Last.After(versions[j].Last) })

	out := make([]string, 0, len(versions))
	seen := map[string]bool{}
	for _, v := range versions {
		if !seen[v.Tag] {
			seen[v.Tag] = true
			out = append(out, v.Tag)
		}
	}
	return out
}

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
	limit := 200

	var images []models.Image

	if query != "" {
		iter := h.db.Session().Query(
			`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, category, is_public, file_path, logo_path, created_at, updated_at, last_updated 
			 FROM images WHERE name = ? LIMIT ? ALLOW FILTERING`,
			query, limit,
		).Iter()

		var image models.Image
		for iter.Scan(&image.ID, &image.Name, &image.Tag, &image.OwnerID, &image.Description, &image.Digest,
			&image.Size, &image.Downloads, &image.Pulls, &image.Stars, &image.Category, &image.IsPublic, &image.FilePath, &image.LogoPath,
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
		iter := h.db.Session().Query(
			`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, category, is_public, file_path, logo_path, created_at, updated_at, last_updated 
			 FROM images LIMIT ? ALLOW FILTERING`,
			limit,
		).Iter()

		var image models.Image
		for iter.Scan(&image.ID, &image.Name, &image.Tag, &image.OwnerID, &image.Description, &image.Digest,
			&image.Size, &image.Downloads, &image.Pulls, &image.Stars, &image.Category, &image.IsPublic, &image.FilePath, &image.LogoPath,
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
			"category":       img.Category,
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
		`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, category, is_public, file_path, logo_path, created_at, updated_at, last_updated 
		 FROM images WHERE name = ? ALLOW FILTERING`,
		name,
	).Iter()

	var image models.Image
	for iter.Scan(&image.ID, &image.Name, &image.Tag, &image.OwnerID, &image.Description, &image.Digest,
		&image.Size, &image.Downloads, &image.Pulls, &image.Stars, &image.Category, &image.IsPublic, &image.FilePath, &image.LogoPath,
		&image.CreatedAt, &image.UpdatedAt, &image.LastUpdated) {
		if image.IsPublic {
			images = append(images, image)
		}
	}

	if err := iter.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch images"})
		return
	}

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
			"category":       img.Category,
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

	var image models.Image
	if err := h.db.Session().Query(
		`SELECT id, name, tag, owner_id, file_path, logo_path FROM images WHERE id = ?`,
		imageID,
	).Scan(&image.ID, &image.Name, &image.Tag, &image.OwnerID, &image.FilePath, &image.LogoPath); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
		return
	}

	if image.OwnerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := os.Remove(image.FilePath); err != nil {
		fmt.Printf("Warning: Failed to delete file: %v\n", err)
	}

	if image.LogoPath != "" {
		if err := os.Remove(image.LogoPath); err != nil {
			fmt.Printf("Warning: Failed to delete logo: %v\n", err)
		}
	}

	os.Remove(filepath.Dir(image.FilePath))

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

	limit := 20
	offset := 0
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if v := strings.TrimSpace(c.Query("offset")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	var images []models.Image
	iter := h.db.Session().Query(
		`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, category, is_public, file_path, logo_path, created_at, updated_at, last_updated 
		 FROM images WHERE owner_id = ? ALLOW FILTERING`,
		userID,
	).Iter()

	var image models.Image
	for iter.Scan(&image.ID, &image.Name, &image.Tag, &image.OwnerID, &image.Description, &image.Digest,
		&image.Size, &image.Downloads, &image.Pulls, &image.Stars, &image.Category, &image.IsPublic, &image.FilePath, &image.LogoPath,
		&image.CreatedAt, &image.UpdatedAt, &image.LastUpdated) {
		images = append(images, image)
	}
	if err := iter.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch images"})
		return
	}

	start := offset
	if start > len(images) {
		start = len(images)
	}
	end := start + limit
	if end > len(images) {
		end = len(images)
	}

	page := images[start:end]

	enriched := make([]gin.H, 0, len(page))
	for _, img := range page {
		logoURL := ""
		if img.LogoPath != "" {
			logoURL = "/api/images/" + img.Name + "/" + img.Tag + "/logo"
		}
		enriched = append(enriched, gin.H{
			"id":           img.ID,
			"name":         img.Name,
			"tag":          img.Tag,
			"description":  img.Description,
			"is_public":    img.IsPublic,
			"logo_path":    logoURL,
			"created_at":   img.CreatedAt,
			"updated_at":   img.UpdatedAt,
			"last_updated": img.LastUpdated,
		})
	}

	c.JSON(http.StatusOK, gin.H{"images": enriched, "total": len(images), "limit": limit, "offset": offset})
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

	var existingUserID gocql.UUID
	if err := h.db.Session().Query(
		`SELECT user_id FROM stars WHERE user_id = ? AND image_id = ?`,
		userID, imageID,
	).Scan(&existingUserID); err == nil {
		var currentStars int64
		if err := h.db.Session().Query(
			`SELECT stars FROM images WHERE id = ?`,
			imageID,
		).Scan(&currentStars); err != nil {
			c.JSON(http.StatusOK, gin.H{"message": "Already starred", "starred": true})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Already starred", "stars": currentStars, "starred": true})
		return
	}

	if err := h.db.Session().Query(
		`INSERT INTO stars (user_id, image_id, created_at) VALUES (?, ?, ?)`,
		userID, imageID, time.Now(),
	).Exec(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to star image"})
		return
	}

	// Read-modify-write for non-counter column
	var currentStars int64
	if err := h.db.Session().Query(
		`SELECT stars FROM images WHERE id = ?`,
		imageID,
	).Scan(&currentStars); err != nil {
		fmt.Printf("Warning: Failed to read stars: %v\n", err)
		c.JSON(http.StatusOK, gin.H{"message": "Image starred successfully"})
		return
	}
	if err := h.db.Session().Query(
		`UPDATE images SET stars = ? WHERE id = ?`,
		currentStars+1, imageID,
	).Exec(); err != nil {
		fmt.Printf("Warning: Failed to increment star counter: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Image starred successfully", "stars": currentStars + 1, "starred": true})
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

	if err := h.db.Session().Query(
		`DELETE FROM stars WHERE user_id = ? AND image_id = ?`,
		userID, imageID,
	).Exec(); err != nil {
		// If not starred, respond idempotently
		var currentStars int64
		if err2 := h.db.Session().Query(
			`SELECT stars FROM images WHERE id = ?`,
			imageID,
		).Scan(&currentStars); err2 == nil {
			c.JSON(http.StatusOK, gin.H{"message": "Not starred", "stars": currentStars, "starred": false})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unstar image"})
		return
	}

	// Read-modify-write and clamp at zero
	var currentStars int64
	if err := h.db.Session().Query(
		`SELECT stars FROM images WHERE id = ?`,
		imageID,
	).Scan(&currentStars); err != nil {
		fmt.Printf("Warning: Failed to read stars: %v\n", err)
		c.JSON(http.StatusOK, gin.H{"message": "Image unstarred successfully"})
		return
	}
	newStars := currentStars - 1
	if newStars < 0 {
		newStars = 0
	}
	if err := h.db.Session().Query(
		`UPDATE images SET stars = ? WHERE id = ?`,
		newStars, imageID,
	).Exec(); err != nil {
		fmt.Printf("Warning: Failed to decrement star counter: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Image unstarred successfully", "stars": newStars, "starred": false})
}

func (h *ImageHandler) StarStatus(c *gin.Context) {
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
	var currentStars int64
	_ = h.db.Session().Query(`SELECT stars FROM images WHERE id = ?`, imageID).Scan(&currentStars)

	var tmp gocql.UUID
	starred := false
	if err := h.db.Session().Query(`SELECT user_id FROM stars WHERE user_id = ? AND image_id = ?`, userID, imageID).Scan(&tmp); err == nil {
		starred = true
	}
	c.JSON(http.StatusOK, gin.H{"stars": currentStars, "starred": starred})
}

func (h *ImageHandler) DownloadFile(c *gin.Context) {
	filename := c.Param("filename")

	filePath := filepath.Join(h.cfg.StoragePath, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

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

func renderMarkdown(s string) template.HTML {
	if strings.TrimSpace(s) == "" {
		return template.HTML("")
	}
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(
			mdhtml.WithHardWraps(),
			mdhtml.WithUnsafe(),
		),
	)
	var buf bytes.Buffer
	if err := md.Convert([]byte(s), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(s))
	}

	policy := bluemonday.UGCPolicy()
	policy.AllowElements("pre", "code")
	safe := policy.SanitizeBytes(buf.Bytes())
	return template.HTML(safe)
}
