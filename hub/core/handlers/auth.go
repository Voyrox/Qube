package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Voyrox/Qube/hub/core/cache"
	"github.com/Voyrox/Qube/hub/core/config"
	"github.com/Voyrox/Qube/hub/core/database"
	"github.com/Voyrox/Qube/hub/core/models"
	"github.com/gin-gonic/gin"
	"github.com/gocql/gocql"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	db        *database.ScyllaDB
	cfg       *config.Config
	userCache *cache.UserCache
}

func NewAuthHandler(db *database.ScyllaDB, cfg *config.Config, userCache *cache.UserCache) *AuthHandler {
	return &AuthHandler{db: db, cfg: cfg, userCache: userCache}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var existingUser string
	if err := h.db.Session().Query(
		"SELECT username FROM users WHERE username = ? LIMIT 1",
		req.Username,
	).Scan(&existingUser); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}

	if err := h.db.Session().Query(
		"SELECT email FROM users WHERE email = ? LIMIT 1 ALLOW FILTERING",
		req.Email,
	).Scan(&existingUser); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := models.User{
		ID:           gocql.TimeUUID(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := h.db.Session().Query(
		`INSERT INTO users (id, username, email, password_hash, created_at, updated_at) 
		 VALUES (?, ?, ?, ?, ?, ?)`,
		user.ID, user.Username, user.Email, user.PasswordHash, user.CreatedAt, user.UpdatedAt,
	).Exec(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	h.userCache.SetUser(user.ID, &user)

	token, err := h.generateToken(user.ID.String(), user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, models.AuthResponse{
		Token: token,
		User:  user,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User

	if cachedUser, found := h.userCache.GetUserByUsername(req.Identifier); found {
		user = *cachedUser
	} else if cachedUser, found := h.userCache.GetUserByEmail(req.Identifier); found {
		user = *cachedUser
	} else {
		err := h.db.Session().Query(
			`SELECT id, username, email, password_hash, created_at, updated_at 
			 FROM users WHERE username = ? LIMIT 1 ALLOW FILTERING`,
			req.Identifier,
		).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)

		if err != nil {
			err = h.db.Session().Query(
				`SELECT id, username, email, password_hash, created_at, updated_at 
				 FROM users WHERE email = ? LIMIT 1 ALLOW FILTERING`,
				req.Identifier,
			).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
		}

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		h.userCache.SetUser(user.ID, &user)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, err := h.generateToken(user.ID.String(), user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, models.AuthResponse{
		Token: token,
		User:  user,
	})
}

func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uuid, _ := gocql.ParseUUID(userID.(string))

	if user, found := h.userCache.GetUser(uuid); found {
		c.JSON(http.StatusOK, user)
		return
	}

	var user models.User
	if err := h.db.Session().Query(
		`SELECT id, username, email, created_at, updated_at 
		 FROM users WHERE id = ?`,
		uuid,
	).Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt, &user.UpdatedAt); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	h.userCache.SetUser(user.ID, &user)

	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userIDStr, _ := c.Get("user_id")
	uuid, err := gocql.ParseUUID(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user"})
		return
	}

	var user models.User
	if err := h.db.Session().Query(
		`SELECT id, username, email, password_hash, created_at, updated_at FROM users WHERE id = ?`,
		uuid,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Parse request
	var req struct {
		Username        string `form:"username" json:"username"`
		Email           string `form:"email" json:"email"`
		CurrentPassword string `form:"current_password" json:"current_password"`
		NewPassword     string `form:"new_password" json:"new_password"`
	}
	if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/") {
		_ = c.ShouldBind(&req)
	} else {
		_ = c.ShouldBindJSON(&req)
	}

	// Username change
	if req.Username != "" && req.Username != user.Username {
		var existing string
		if err := h.db.Session().Query(
			"SELECT username FROM users WHERE username = ? LIMIT 1",
			req.Username,
		).Scan(&existing); err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
			return
		}
		user.Username = req.Username
	}

	// Email change
	if req.Email != "" && req.Email != user.Email {
		var existing string
		if err := h.db.Session().Query(
			"SELECT email FROM users WHERE email = ? LIMIT 1 ALLOW FILTERING",
			req.Email,
		).Scan(&existing); err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
		user.Email = req.Email
	}

	if strings.TrimSpace(req.NewPassword) != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password incorrect"})
			return
		}
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
		user.PasswordHash = string(hashedPassword)
	}

	if file, err := c.FormFile("avatar"); err == nil && file != nil {
		avatarDir := "static/avatars"
		_ = os.MkdirAll(avatarDir, 0755)
		avatarPath := filepath.Join(avatarDir, uuid.String()+".png")
		_ = c.SaveUploadedFile(file, avatarPath)
	}

	user.UpdatedAt = time.Now()
	if err := h.db.Session().Query(
		`UPDATE users SET username = ?, email = ?, password_hash = ?, updated_at = ? WHERE id = ?`,
		user.Username, user.Email, user.PasswordHash, user.UpdatedAt, user.ID,
	).Exec(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	// Invalidate user cache on update
	h.userCache.InvalidateUser(user.ID, user.Username, user.Email)

	token, err := h.generateToken(user.ID.String(), user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh token"})
		return
	}

	c.JSON(http.StatusOK, models.AuthResponse{Token: token, User: user})
}

func (h *AuthHandler) generateToken(userID, username string) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"exp":      time.Now().Add(time.Hour * 24 * 7).Unix(), // 7 days
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.JWTSecret))
}
