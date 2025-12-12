package models

import (
	"time"

	"github.com/gocql/gocql"
)

type User struct {
	ID           gocql.UUID `json:"id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type Image struct {
	ID          gocql.UUID `json:"id"`
	Name        string     `json:"name"`
	Tag         string     `json:"tag"`
	OwnerID     gocql.UUID `json:"owner_id"`
	Description string     `json:"description"`
	Digest      string     `json:"digest"`
	Size        int64      `json:"size"`
	Downloads   int64      `json:"downloads"`
	Pulls       int64      `json:"pulls"`
	Stars       int64      `json:"stars"`
	Category    string     `json:"category"`
	IsPublic    bool       `json:"is_public"`
	FilePath    string     `json:"-"`
	LogoPath    string     `json:"logo_path,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastUpdated time.Time  `json:"last_updated"`
}

type ImageTag struct {
	ImageID   gocql.UUID `json:"image_id"`
	Tag       string     `json:"tag"`
	CreatedAt time.Time  `json:"created_at"`
}

type Star struct {
	UserID    gocql.UUID `json:"user_id"`
	ImageID   gocql.UUID `json:"image_id"`
	CreatedAt time.Time  `json:"created_at"`
}

type Comment struct {
	ID        gocql.UUID `json:"id"`
	ImageID   gocql.UUID `json:"image_id"`
	UserID    gocql.UUID `json:"user_id"`
	Content   string     `json:"content"`
	CreatedAt time.Time  `json:"created_at"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginRequest struct {
	Identifier string `json:"identifier" binding:"required"`
	Password   string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type ImageUploadRequest struct {
	Name        string `form:"name" binding:"required"`
	Tag         string `form:"tag" binding:"required"`
	Description string `form:"description"`
	Category    string `form:"category"`
	IsPublic    bool   `form:"is_public"`
}

type SearchImagesRequest struct {
	Query  string `form:"query"`
	Limit  int    `form:"limit"`
	Offset int    `form:"offset"`
}

type CommentRequest struct {
	Content string `json:"content" binding:"required,min=1,max=500"`
}
