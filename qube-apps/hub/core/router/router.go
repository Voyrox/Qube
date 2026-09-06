package router

import (
	"os"
	"path/filepath"

	"github.com/Voyrox/Qube/hub/core/cache"
	"github.com/Voyrox/Qube/hub/core/config"
	"github.com/Voyrox/Qube/hub/core/database"
	"github.com/Voyrox/Qube/hub/core/handlers"
	"github.com/Voyrox/Qube/hub/core/middleware"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func Setup(db *database.ScyllaDB, cfg *config.Config, cacheManager *cache.CacheManager) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	r.Use(func(c *gin.Context) {
		c.Set("admin_email", cfg.AdminEmail)
		c.Next()
	})

	websiteDir := resolveWebsiteDir()
	r.Static("/static", "./static")
	r.StaticFile("/logo.png", filepath.Join(websiteDir, "logo.png"))
	r.StaticFile("/styles.css", filepath.Join(websiteDir, "styles.css"))
	r.StaticFile("/script.js", filepath.Join(websiteDir, "script.js"))
	r.LoadHTMLGlob("templates/*")

	authHandler := handlers.NewAuthHandler(db, cfg, cacheManager.Users)
	imageHandler := handlers.NewImageHandler(db, cfg, cacheManager.Images)
	reportHandler := handlers.NewReportHandler(db, cfg)
	statsHandler := handlers.NewStatsHandler(db, cfg, cacheManager.General)

	r.GET("/", func(c *gin.Context) {
		c.File(filepath.Join(websiteDir, "index.html"))
	})

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	r.GET("/hub", func(c *gin.Context) {
		c.Redirect(301, "/hub/")
	})

	hub := r.Group("/hub")
	{
		hub.GET("/", func(c *gin.Context) {
			c.HTML(200, "index.html", gin.H{
				"title": "Qube Hub",
			})
		})

		hub.GET("/explore", middleware.OptionalAuthMiddleware(cfg), func(c *gin.Context) {
			c.HTML(200, "explore.html", gin.H{"title": "Explore Images"})
		})

		hub.GET("/profile", middleware.OptionalAuthMiddleware(cfg), func(c *gin.Context) {
			c.HTML(200, "profile.html", gin.H{"title": "My Profile"})
		})

		hub.GET("/settings", middleware.OptionalAuthMiddleware(cfg), func(c *gin.Context) {
			c.HTML(200, "settings.html", gin.H{"title": "Settings"})
		})

		hub.GET("/images/:name", middleware.OptionalAuthMiddleware(cfg), imageHandler.DetailLatest)
		hub.GET("/images/:name/:tag", middleware.OptionalAuthMiddleware(cfg), imageHandler.Detail)

		hub.GET("/auth", func(c *gin.Context) {
			c.HTML(200, "auth.html", gin.H{
				"title": "Sign In",
			})
		})

		hub.GET("/login", func(c *gin.Context) {
			c.HTML(200, "auth.html", gin.H{
				"title": "Sign In",
			})
		})

		hub.GET("/signup", func(c *gin.Context) {
			c.HTML(200, "auth.html", gin.H{
				"title": "Sign Up",
			})
		})

		hub.GET("/download/:user/:image", imageHandler.DownloadByUser)

		hub.GET("/reports", func(c *gin.Context) {
			c.HTML(200, "reports.html", gin.H{
				"title": "Reports",
			})
		})
	}

	api := r.Group("/api")
	{
		api.GET("/stats", statsHandler.GetStats)

		api.POST("/auth/register", authHandler.Register)
		api.POST("/auth/login", authHandler.Login)
		api.POST("/auth/update", middleware.AuthMiddleware(cfg), authHandler.UpdateProfile)

		api.GET("/images", imageHandler.List)
		api.GET("/images/:name", imageHandler.GetByName)
		api.GET("/images/:name/:tag/download", imageHandler.Download)
		api.GET("/images/:name/:tag/logo", imageHandler.Logo)
		api.GET("/download/:name", imageHandler.DownloadLatest)
		api.GET("/files/:filename", imageHandler.DownloadFile)

		protected := api.Group("")
		protected.Use(middleware.AuthMiddlewareWithDB(cfg, db))
		{
			protected.GET("/auth/profile", authHandler.GetProfile)

			protected.POST("/images/upload", imageHandler.Upload)
			protected.GET("/images/my", imageHandler.GetMyImages)
			protected.DELETE("/images/:id", imageHandler.Delete)

			protected.POST("/image-id/:id/star", imageHandler.Star)
			protected.DELETE("/image-id/:id/star", imageHandler.Unstar)
			protected.GET("/image-id/:id/star", imageHandler.StarStatus)
			protected.POST("/images/by-name/:name/:tag", imageHandler.UpdateImage)

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

func resolveWebsiteDir() string {
	candidates := []string{"Website", filepath.Join("..", "Website"), filepath.Join("..", "qube-apps", "Website")}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return "Website"
}
