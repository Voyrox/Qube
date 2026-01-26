package handlers

import (
	"net/http"
	"time"

	"github.com/Voyrox/Qube/hub/core/cache"
	"github.com/Voyrox/Qube/hub/core/config"
	"github.com/Voyrox/Qube/hub/core/database"
	"github.com/gin-gonic/gin"
)

type HubStats struct {
	Images      int64     `json:"images"`
	Pulls       int64     `json:"pulls"`
	Users       int64     `json:"users"`
	GeneratedAt time.Time `json:"generated_at"`
}

type StatsHandler struct {
	db    *database.ScyllaDB
	cfg   *config.Config
	cache *cache.Cache
}

func NewStatsHandler(db *database.ScyllaDB, cfg *config.Config, generalCache *cache.Cache) *StatsHandler {
	return &StatsHandler{db: db, cfg: cfg, cache: generalCache}
}

func (h *StatsHandler) GetStats(c *gin.Context) {
	const cacheKey = "hub:stats"
	if h.cache != nil {
		if val, ok := h.cache.Get(cacheKey); ok {
			if stats, ok2 := val.(HubStats); ok2 {
				c.JSON(http.StatusOK, stats)
				return
			}
		}
	}

	var imagesCount int64
	var usersCount int64
	var totalPulls int64

	if err := h.db.Session().Query(
		"SELECT COUNT(*) FROM images WHERE is_public = true ALLOW FILTERING",
	).Scan(&imagesCount); err != nil {
		imagesCount = 0
	}

	iter := h.db.Session().Query(
		"SELECT pulls FROM images WHERE is_public = true ALLOW FILTERING",
	).Iter()
	var pulls int64
	for iter.Scan(&pulls) {
		totalPulls += pulls
	}
	_ = iter.Close()

	if err := h.db.Session().Query(
		"SELECT COUNT(*) FROM users",
	).Scan(&usersCount); err != nil {
		usersCount = 0
	}

	stats := HubStats{
		Images:      imagesCount,
		Pulls:       totalPulls,
		Users:       usersCount,
		GeneratedAt: time.Now(),
	}

	if h.cache != nil {
		h.cache.SetWithTTL(cacheKey, stats, 24*time.Hour)
	}

	c.JSON(http.StatusOK, stats)
}
