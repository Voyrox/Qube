package middleware

import (
	"net/http"
	"strings"

	"github.com/Voyrox/Qube/hub/core/config"
	"github.com/Voyrox/Qube/hub/core/database"
	"github.com/gin-gonic/gin"
	"github.com/gocql/gocql"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return AuthMiddlewareWithDB(cfg, nil)
}

func AuthMiddlewareWithDB(cfg *config.Config, db *database.ScyllaDB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := ""
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString = parts[1]
			}
		}
		if tokenString == "" {
			if cookie, err := c.Cookie("token"); err == nil {
				tokenString = cookie
			}
		}
		if tokenString == "" {
			if qp := c.Query("token"); qp != "" {
				tokenString = qp
			}
		}
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(*Claims); ok {
			userIDParsed, err := gocql.ParseUUID(claims.UserID)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID"})
				c.Abort()
				return
			}
			c.Set("userID", userIDParsed)
			c.Set("user_id", claims.UserID)
			c.Set("username", claims.Username)

			// Fetch user email from database if db is provided
			if db != nil {
				var email string
				err := db.Session().Query(
					"SELECT email FROM users WHERE id = ? LIMIT 1",
					userIDParsed,
				).Scan(&email)
				if err == nil {
					c.Set("userEmail", email)
				}
			}
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func OptionalAuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := ""
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString = parts[1]
			}
		}
		if tokenString == "" {
			if cookie, err := c.Cookie("token"); err == nil {
				tokenString = cookie
			}
		}
		if tokenString == "" {
			if qp := c.Query("token"); qp != "" {
				tokenString = qp
			}
		}
		if tokenString != "" {
			token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
				return []byte(cfg.JWTSecret), nil
			})
			if err == nil && token.Valid {
				if claims, ok := token.Claims.(*Claims); ok {
					c.Set("user_id", claims.UserID)
					c.Set("username", claims.Username)
				}
			}
		}

		c.Next()
	}
}

func GetUserID(c *gin.Context) (gocql.UUID, bool) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		return gocql.UUID{}, false
	}

	userID, err := gocql.ParseUUID(userIDStr.(string))
	if err != nil {
		return gocql.UUID{}, false
	}

	return userID, true
}
