package main

import (
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.Static("/", "./rootPage")

	port := os.Getenv("PORT")
	if port == "" {
		port = "2343"
	}

	_ = r.Run(":" + port)
}
