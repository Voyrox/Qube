package main

import (
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	r.GET("/", func(c *gin.Context) {
		c.File("index.html")
	})
	r.StaticFile("/footer.png", "./footer.png")

	_ = r.Run(":32002")
}
