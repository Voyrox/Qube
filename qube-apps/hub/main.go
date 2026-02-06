package main

import (
	"log"
	"os"
	"time"

	"github.com/Voyrox/Qube/hub/core/cache"
	"github.com/Voyrox/Qube/hub/core/config"
	"github.com/Voyrox/Qube/hub/core/database"
	"github.com/Voyrox/Qube/hub/core/router"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg := config.Load()
	db, err := database.NewScyllaDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to ScyllaDB: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}

	if err := os.MkdirAll(cfg.StoragePath, 0755); err != nil {
		log.Fatalf("Failed to create storage directory: %v", err)
	}

	if cfg.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize cache manager with 15 minute TTL
	cacheManager := cache.NewCacheManager(15 * time.Minute)

	r := router.Setup(db, cfg, cacheManager)

	log.Printf("Starting QubeCloud on %s", cfg.Addr)
	if err := r.Run(cfg.Addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
