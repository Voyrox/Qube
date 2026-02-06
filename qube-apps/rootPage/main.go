package main

import (
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.Static("/static", "./static")

	r.GET("/", func(c *gin.Context) {
		c.File("index.html")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "2343"
	}

	_ = r.Run(":" + port)
}
