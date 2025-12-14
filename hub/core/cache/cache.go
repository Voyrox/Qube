package cache

import (
	"sync"
	"time"

	"github.com/Voyrox/Qube/hub/core/models"
	"github.com/gocql/gocql"
)

type CacheEntry struct {
	Value      interface{}
	ExpiresAt  time.Time
	CreatedAt  time.Time
	LastAccess time.Time
}

type Cache struct {
	mu              sync.RWMutex
	data            map[string]*CacheEntry
	defaultTTL      time.Duration
	cleanupInterval time.Duration
	lastCleanupTime time.Time
	maxSize         int
	currentSize     int
}

func NewCache(defaultTTL time.Duration) *Cache {
	c := &Cache{
		data:            make(map[string]*CacheEntry),
		defaultTTL:      defaultTTL,
		cleanupInterval: time.Minute,
		lastCleanupTime: time.Now(),
		maxSize:         10000,
		currentSize:     0,
	}

	go c.cleanupExpired()

	return c
}

func (c *Cache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, c.defaultTTL)
}

func (c *Cache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	c.data[key] = &CacheEntry{
		Value:      value,
		ExpiresAt:  now.Add(ttl),
		CreatedAt:  now,
		LastAccess: now,
	}
	c.currentSize++

	if now.Sub(c.lastCleanupTime) > c.cleanupInterval {
		c.lastCleanupTime = now
		go c.cleanupExpiredUnlocked()
	}
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	entry, exists := c.data[key]
	c.mu.RUnlock()

	if !exists {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		c.Delete(key)
		return nil, false
	}

	c.mu.Lock()
	entry.LastAccess = time.Now()
	c.mu.Unlock()

	return entry.Value, true
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.data[key]; exists {
		delete(c.data, key)
		c.currentSize--
	}
}

func (c *Cache) DeletePattern(pattern string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	deleted := 0
	for key := range c.data {
		if len(key) >= len(pattern) && key[:len(pattern)] == pattern {
			delete(c.data, key)
			deleted++
			c.currentSize--
		}
	}
	return deleted
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[string]*CacheEntry)
	c.currentSize = 0
}

func (c *Cache) GetSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.currentSize
}

func (c *Cache) cleanupExpired() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		c.cleanupExpiredUnlocked()
		c.mu.Unlock()
	}
}

func (c *Cache) cleanupExpiredUnlocked() {
	now := time.Now()
	for key, entry := range c.data {
		if now.After(entry.ExpiresAt) {
			delete(c.data, key)
			c.currentSize--
		}
	}
}

type ImageCache struct {
	*Cache
}

func NewImageCache(defaultTTL time.Duration) *ImageCache {
	return &ImageCache{Cache: NewCache(defaultTTL)}
}

func (ic *ImageCache) SetImage(id gocql.UUID, image *models.Image) {
	ic.Set("image:"+id.String(), image)
	ic.Set("image:name:"+image.Name, image)
}

func (ic *ImageCache) GetImage(id gocql.UUID) (*models.Image, bool) {
	val, exists := ic.Get("image:" + id.String())
	if !exists {
		return nil, false
	}
	return val.(*models.Image), true
}

func (ic *ImageCache) GetImageByName(name string) (*models.Image, bool) {
	val, exists := ic.Get("image:name:" + name)
	if !exists {
		return nil, false
	}
	return val.(*models.Image), true
}

func (ic *ImageCache) InvalidateImage(id gocql.UUID, name string) {
	ic.Delete("image:" + id.String())
	ic.Delete("image:name:" + name)
	ic.DeletePattern("images:list:")
	ic.DeletePattern("images:search:")
}

func (ic *ImageCache) SetImages(key string, images []models.Image) {
	ic.Set("images:list:"+key, images)
}

func (ic *ImageCache) GetImages(key string) ([]models.Image, bool) {
	val, exists := ic.Get("images:list:" + key)
	if !exists {
		return nil, false
	}
	return val.([]models.Image), true
}

func (ic *ImageCache) InvalidateImagesList() {
	ic.DeletePattern("images:list:")
}

type UserCache struct {
	*Cache
}

func NewUserCache(defaultTTL time.Duration) *UserCache {
	return &UserCache{Cache: NewCache(defaultTTL)}
}

func (uc *UserCache) SetUser(id gocql.UUID, user *models.User) {
	uc.Set("user:"+id.String(), user)
	uc.Set("user:username:"+user.Username, user)
	uc.Set("user:email:"+user.Email, user)
}

func (uc *UserCache) GetUser(id gocql.UUID) (*models.User, bool) {
	val, exists := uc.Get("user:" + id.String())
	if !exists {
		return nil, false
	}
	return val.(*models.User), true
}

func (uc *UserCache) GetUserByUsername(username string) (*models.User, bool) {
	val, exists := uc.Get("user:username:" + username)
	if !exists {
		return nil, false
	}
	return val.(*models.User), true
}

func (uc *UserCache) GetUserByEmail(email string) (*models.User, bool) {
	val, exists := uc.Get("user:email:" + email)
	if !exists {
		return nil, false
	}
	return val.(*models.User), true
}

func (uc *UserCache) InvalidateUser(id gocql.UUID, username string, email string) {
	uc.Delete("user:" + id.String())
	uc.Delete("user:username:" + username)
	uc.Delete("user:email:" + email)
}

type CommentCache struct {
	*Cache
}

func NewCommentCache(defaultTTL time.Duration) *CommentCache {
	return &CommentCache{Cache: NewCache(defaultTTL)}
}

func (cc *CommentCache) SetComments(imageID gocql.UUID, comments []models.Comment) {
	cc.Set("comments:"+imageID.String(), comments)
}

func (cc *CommentCache) GetComments(imageID gocql.UUID) ([]models.Comment, bool) {
	val, exists := cc.Get("comments:" + imageID.String())
	if !exists {
		return nil, false
	}
	return val.([]models.Comment), true
}

func (cc *CommentCache) InvalidateComments(imageID gocql.UUID) {
	cc.Delete("comments:" + imageID.String())
}

type CacheManager struct {
	Images   *ImageCache
	Users    *UserCache
	Comments *CommentCache
	General  *Cache
}

func NewCacheManager(ttl time.Duration) *CacheManager {
	return &CacheManager{
		Images:   NewImageCache(ttl),
		Users:    NewUserCache(ttl),
		Comments: NewCommentCache(ttl),
		General:  NewCache(ttl),
	}
}

func (cm *CacheManager) InvalidateAll() {
	cm.Images.Clear()
	cm.Users.Clear()
	cm.Comments.Clear()
	cm.General.Clear()
}

func (cm *CacheManager) GetStats() map[string]int {
	return map[string]int{
		"images":   cm.Images.GetSize(),
		"users":    cm.Users.GetSize(),
		"comments": cm.Comments.GetSize(),
		"general":  cm.General.GetSize(),
	}
}
