package handlers

import (
	"net/http"
	"time"

	"github.com/Voyrox/Qube/hub/core/config"
	"github.com/Voyrox/Qube/hub/core/database"
	"github.com/Voyrox/Qube/hub/core/models"
	"github.com/gin-gonic/gin"
	"github.com/gocql/gocql"
)

type ReportHandler struct {
	db  *database.ScyllaDB
	cfg *config.Config
}

func NewReportHandler(db *database.ScyllaDB, cfg *config.Config) *ReportHandler {
	return &ReportHandler{db: db, cfg: cfg}
}

func (h *ReportHandler) SubmitReport(c *gin.Context) {
	userID, exists := c.Get("userID")
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

	var req models.ReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if image exists
	var imageName string
	err = h.db.Session().Query(
		"SELECT name FROM images WHERE id = ? LIMIT 1",
		imageID,
	).Scan(&imageName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
		return
	}

	// Check if user already reported this image
	var existingID gocql.UUID
	err = h.db.Session().Query(
		"SELECT id FROM reports WHERE image_id = ? AND user_id = ? LIMIT 1",
		imageID, userID,
	).Scan(&existingID)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "You have already reported this image"})
		return
	}

	report := models.Report{
		ID:        gocql.TimeUUID(),
		ImageID:   imageID,
		UserID:    userID.(gocql.UUID),
		Reason:    req.Reason,
		CreatedAt: time.Now(),
	}

	err = h.db.Session().Query(
		`INSERT INTO reports (id, image_id, user_id, reason, created_at) 
		 VALUES (?, ?, ?, ?, ?)`,
		report.ID, report.ImageID, report.UserID, report.Reason, report.CreatedAt,
	).Exec()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit report"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Report submitted successfully"})
}

// GetReports returns all reports (admin only)
func (h *ReportHandler) GetReports(c *gin.Context) {
	userEmail, exists := c.Get("userEmail")
	if !exists || userEmail.(string) != h.cfg.AdminEmail {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	type ReportWithDetails struct {
		Report           models.Report `json:"report"`
		ImageName        string        `json:"image_name"`
		ImageTag         string        `json:"image_tag"`
		ImageOwner       string        `json:"image_owner"`
		ImageOwnerID     gocql.UUID    `json:"image_owner_id"`
		ReporterUsername string        `json:"reporter_username"`
	}

	// Get all unique image_ids with reports
	iter := h.db.Session().Query(
		`SELECT DISTINCT image_id FROM reports`,
	).Iter()

	var imageID gocql.UUID
	var reportsMap = make(map[string][]ReportWithDetails)

	for iter.Scan(&imageID) {
		// Get image details
		var imageName, imageTag, imageOwnerUsername string
		var imageOwnerID gocql.UUID
		err := h.db.Session().Query(
			"SELECT name, tag, owner_id FROM images WHERE id = ? LIMIT 1",
			imageID,
		).Scan(&imageName, &imageTag, &imageOwnerID)
		if err != nil {
			continue
		}

		// Get owner username
		err = h.db.Session().Query(
			"SELECT username FROM users WHERE id = ? LIMIT 1",
			imageOwnerID,
		).Scan(&imageOwnerUsername)
		if err != nil {
			imageOwnerUsername = "Unknown"
		}

		// Get all reports for this image
		reportIter := h.db.Session().Query(
			"SELECT id, image_id, user_id, reason, created_at FROM reports WHERE image_id = ?",
			imageID,
		).Iter()

		var report models.Report
		for reportIter.Scan(&report.ID, &report.ImageID, &report.UserID, &report.Reason, &report.CreatedAt) {
			// Get reporter username
			var reporterUsername string
			err := h.db.Session().Query(
				"SELECT username FROM users WHERE id = ? LIMIT 1",
				report.UserID,
			).Scan(&reporterUsername)
			if err != nil {
				reporterUsername = "Unknown"
			}

			reportDetail := ReportWithDetails{
				Report:           report,
				ImageName:        imageName,
				ImageTag:         imageTag,
				ImageOwner:       imageOwnerUsername,
				ImageOwnerID:     imageOwnerID,
				ReporterUsername: reporterUsername,
			}

			key := imageID.String()
			reportsMap[key] = append(reportsMap[key], reportDetail)
		}
		if err := reportIter.Close(); err != nil {
			continue
		}
	}
	if err := iter.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reports"})
		return
	}

	c.JSON(http.StatusOK, reportsMap)
}

// DeleteReportedImage deletes an image and all its reports (admin only)
func (h *ReportHandler) DeleteReportedImage(c *gin.Context) {
	userEmail, exists := c.Get("userEmail")
	if !exists || userEmail.(string) != h.cfg.AdminEmail {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	imageIDStr := c.Param("id")
	imageID, err := gocql.ParseUUID(imageIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image ID"})
		return
	}

	// Delete the image
	err = h.db.Session().Query(
		"DELETE FROM images WHERE id = ?",
		imageID,
	).Exec()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete image"})
		return
	}

	// Delete all reports for this image
	iter := h.db.Session().Query(
		"SELECT user_id FROM reports WHERE image_id = ?",
		imageID,
	).Iter()

	var userID gocql.UUID
	for iter.Scan(&userID) {
		h.db.Session().Query(
			"DELETE FROM reports WHERE image_id = ? AND user_id = ?",
			imageID, userID,
		).Exec()
	}
	iter.Close()

	// Delete stars for this image
	starIter := h.db.Session().Query(
		"SELECT user_id FROM stars WHERE image_id = ?",
		imageID,
	).Iter()

	for starIter.Scan(&userID) {
		h.db.Session().Query(
			"DELETE FROM stars WHERE image_id = ? AND user_id = ?",
			imageID, userID,
		).Exec()
	}
	starIter.Close()

	c.JSON(http.StatusOK, gin.H{"message": "Image deleted successfully"})
}

// BanUser bans a user and deletes all their images (admin only)
func (h *ReportHandler) BanUser(c *gin.Context) {
	userEmail, exists := c.Get("userEmail")
	if !exists || userEmail.(string) != h.cfg.AdminEmail {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	userIDStr := c.Param("id")
	userID, err := gocql.ParseUUID(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get user's images
	iter := h.db.Session().Query(
		"SELECT id FROM images WHERE owner_id = ?",
		userID,
	).Iter()

	var imageID gocql.UUID
	for iter.Scan(&imageID) {
		// Delete the image
		h.db.Session().Query(
			"DELETE FROM images WHERE id = ?",
			imageID,
		).Exec()

		// Delete reports for this image
		reportIter := h.db.Session().Query(
			"SELECT user_id FROM reports WHERE image_id = ?",
			imageID,
		).Iter()

		var reporterID gocql.UUID
		for reportIter.Scan(&reporterID) {
			h.db.Session().Query(
				"DELETE FROM reports WHERE image_id = ? AND user_id = ?",
				imageID, reporterID,
			).Exec()
		}
		reportIter.Close()

		// Delete stars for this image
		starIter := h.db.Session().Query(
			"SELECT user_id FROM stars WHERE image_id = ?",
			imageID,
		).Iter()

		for starIter.Scan(&reporterID) {
			h.db.Session().Query(
				"DELETE FROM stars WHERE image_id = ? AND user_id = ?",
				imageID, reporterID,
			).Exec()
		}
		starIter.Close()
	}
	iter.Close()

	// Delete the user
	err = h.db.Session().Query(
		"DELETE FROM users WHERE id = ?",
		userID,
	).Exec()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to ban user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User banned and all content deleted"})
}

// DismissReports dismisses all reports for an image (admin only)
func (h *ReportHandler) DismissReports(c *gin.Context) {
	userEmail, exists := c.Get("userEmail")
	if !exists || userEmail.(string) != h.cfg.AdminEmail {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	imageIDStr := c.Param("id")
	imageID, err := gocql.ParseUUID(imageIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image ID"})
		return
	}

	// Delete all reports for this image
	iter := h.db.Session().Query(
		"SELECT user_id FROM reports WHERE image_id = ?",
		imageID,
	).Iter()

	var userID gocql.UUID
	for iter.Scan(&userID) {
		h.db.Session().Query(
			"DELETE FROM reports WHERE image_id = ? AND user_id = ?",
			imageID, userID,
		).Exec()
	}
	if err := iter.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to dismiss reports"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reports dismissed successfully"})
}
