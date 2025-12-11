package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Voyrox/Qube/hub/core/config"
	"github.com/Voyrox/Qube/hub/core/database"
	"github.com/Voyrox/Qube/hub/core/middleware"
	"github.com/Voyrox/Qube/hub/core/models"
	"github.com/gin-gonic/gin"
	"github.com/gocql/gocql"
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

	c.JSON(http.StatusCreated, image)
}

func (h *ImageHandler) Download(c *gin.Context) {
	name := c.Param("name")
	tag := c.Param("tag")

	// Find image
	var image models.Image
	if err := h.db.Session().Query(
		`SELECT id, name, tag, owner_id, description, digest, size, downloads, pulls, stars, is_public, file_path, logo_path, created_at, updated_at, last_updated 
		 FROM images WHERE name = ? AND tag = ? LIMIT 1 ALLOW FILTERING`,
		name, tag,
	).Scan(&image.ID, &image.Name, &image.Tag, &image.OwnerID, &image.Description, &image.Digest,
		&image.Size, &image.Downloads, &image.Pulls, &image.Stars, &image.IsPublic, &image.FilePath, &image.LogoPath,
		&image.CreatedAt, &image.UpdatedAt, &image.LastUpdated); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
		return
	}

	// Check if image is public or user is the owner
	userID, authenticated := middleware.GetUserID(c)
	if !image.IsPublic && (!authenticated || userID != image.OwnerID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Increment download and pull counters
	if err := h.db.Session().Query(
		`UPDATE images SET downloads = downloads + 1, pulls = pulls + 1 WHERE id = ?`,
		image.ID,
	).Exec(); err != nil {
		fmt.Printf("Warning: Failed to increment counters: %v\n", err)
	}

	// Serve file
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_%s.tar", image.Name, image.Tag))
	c.Header("Content-Type", "application/x-tar")
	c.File(image.FilePath)
}

// DownloadLatest resolves the latest tag for a given image name and serves the file
func (h *ImageHandler) DownloadLatest(c *gin.Context) {
	name := c.Param("name")

	// Fetch all tags for the image name and pick the latest by last_updated
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

	// Access control
	userID, authenticated := middleware.GetUserID(c)
	if !latest.IsPublic && (!authenticated || userID != latest.OwnerID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Increment counters
	if err := h.db.Session().Query(
		`UPDATE images SET downloads = downloads + 1, pulls = pulls + 1 WHERE id = ?`,
		latest.ID,
	).Exec(); err != nil {
		fmt.Printf("Warning: Failed to increment counters: %v\n", err)
	}

	// Serve file
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_%s.tar", latest.Name, latest.Tag))
	c.Header("Content-Type", "application/x-tar")
	c.File(latest.FilePath)
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

	c.JSON(http.StatusOK, gin.H{"images": images})
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

	c.JSON(http.StatusOK, gin.H{"images": images})
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
