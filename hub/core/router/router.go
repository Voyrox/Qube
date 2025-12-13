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

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Middleware to inject admin_email into all templates
	r.Use(func(c *gin.Context) {
		c.Set("admin_email", cfg.AdminEmail)
		c.Next()
	})

	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*")

	authHandler := handlers.NewAuthHandler(db, cfg)
	imageHandler := handlers.NewImageHandler(db, cfg)
	reportHandler := handlers.NewReportHandler(db, cfg)

	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", gin.H{
			"title": "Qube Hub",
		})
	})

	r.GET("/explore", middleware.OptionalAuthMiddleware(cfg), func(c *gin.Context) {
		c.HTML(200, "explore.html", gin.H{"title": "Explore Images"})
	})

	r.GET("/profile", middleware.OptionalAuthMiddleware(cfg), func(c *gin.Context) {
		c.HTML(200, "profile.html", gin.H{"title": "My Profile"})
	})

	r.GET("/settings", middleware.OptionalAuthMiddleware(cfg), func(c *gin.Context) {
		c.HTML(200, "settings.html", gin.H{"title": "Settings"})
	})

	r.GET("/images/:name", middleware.OptionalAuthMiddleware(cfg), imageHandler.DetailLatest)
	r.GET("/images/:name/:tag", middleware.OptionalAuthMiddleware(cfg), imageHandler.Detail)

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

	r.GET("/download/:user/:image", imageHandler.DownloadByUser)

	// Reports page (client-side will check admin access)
	r.GET("/reports", func(c *gin.Context) {
		c.HTML(200, "reports.html", gin.H{
			"title": "Reports",
		})
	})

	api := r.Group("/api")
	{
		api.POST("/auth/register", authHandler.Register)
		api.POST("/auth/login", authHandler.Login)
		api.POST("/auth/update", middleware.AuthMiddleware(cfg), authHandler.UpdateProfile)

		// Image public routes (with optional auth)
		api.GET("/images", imageHandler.List)
		api.GET("/images/:name", imageHandler.GetByName)
		api.GET("/images/:name/:tag/download", imageHandler.Download)
		api.GET("/images/:name/:tag/logo", imageHandler.Logo)
		api.GET("/download/:name", imageHandler.DownloadLatest)
		api.GET("/files/:filename", imageHandler.DownloadFile)

		// Protected routes
		protected := api.Group("")
		protected.Use(middleware.AuthMiddlewareWithDB(cfg, db))
		{
			// Auth
			protected.GET("/auth/profile", authHandler.GetProfile)

			// Images
			protected.POST("/images/upload", imageHandler.Upload)
			protected.GET("/images/my", imageHandler.GetMyImages)
			protected.DELETE("/images/:id", imageHandler.Delete)
			// Use /image-id to avoid conflict with /images/:name
			protected.POST("/image-id/:id/star", imageHandler.Star)
			protected.DELETE("/image-id/:id/star", imageHandler.Unstar)
			protected.GET("/image-id/:id/star", imageHandler.StarStatus)
			protected.POST("/images/by-name/:name/:tag", imageHandler.UpdateImage)

			// Reports
			protected.POST("/reports/:id", reportHandler.SubmitReport)
			protected.GET("/reports", reportHandler.GetReports)
			protected.DELETE("/reports/image/:id", reportHandler.DeleteReportedImage)
			protected.DELETE("/reports/user/:id", reportHandler.BanUser)
			protected.DELETE("/reports/dismiss/:id", reportHandler.DismissReports)
		}
	}

	r.GET("/files/:filename", imageHandler.DownloadFile)

	return r
}
