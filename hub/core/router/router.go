package router

import (
	"github.com/Voyrox/Qube/hub/core/config"
	"github.com/Voyrox/Qube/hub/core/database"
	"github.com/Voyrox/Qube/hub/core/handlers"
	"github.com/Voyrox/Qube/hub/core/middleware"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func Setup(db *database.ScyllaDB, cfg *config.Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Serve static files
	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*")

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(db, cfg)
	imageHandler := handlers.NewImageHandler(db, cfg)

	// Home page
	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", gin.H{
			"title": "Qube Hub",
		})
	})

	// Auth page
	r.GET("/auth", func(c *gin.Context) {
		c.HTML(200, "auth.html", gin.H{
			"title": "Sign In",
		})
	})

	r.GET("/login", func(c *gin.Context) {
		c.HTML(200, "auth.html", gin.H{
			"title": "Sign In",
		})
	})

	r.GET("/signup", func(c *gin.Context) {
		c.HTML(200, "auth.html", gin.H{
			"title": "Sign Up",
		})
	})

	// API routes
	api := r.Group("/api")
	{
		// Public routes
		api.POST("/auth/register", authHandler.Register)
		api.POST("/auth/login", authHandler.Login)

		// Image public routes (with optional auth)
		api.GET("/images", imageHandler.List)
		api.GET("/images/:name", imageHandler.GetByName)
		api.GET("/images/:name/:tag/download", imageHandler.Download)
		api.GET("/download/:name", imageHandler.DownloadLatest)
		api.GET("/files/:filename", imageHandler.DownloadFile)

		// Protected routes
		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware(cfg))
		{
			// Auth
			protected.GET("/auth/profile", authHandler.GetProfile)

			// Images
			protected.POST("/images/upload", imageHandler.Upload)
			protected.GET("/images/my", imageHandler.GetMyImages)
			protected.DELETE("/images/:id", imageHandler.Delete)
			protected.POST("/images/:id/star", imageHandler.Star)
			protected.DELETE("/images/:id/star", imageHandler.Unstar)
		}
	}

	// Legacy compatibility route (for existing Qube client)
	r.GET("/files/:filename", imageHandler.DownloadFile)

	return r
}
