package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"mountain-race/api"
)

func main() {
	// Load .env from project root (one level up from binary in container)
	if err := godotenv.Load("../.env"); err != nil {
		// Try current directory as fallback (dev)
		_ = godotenv.Load(".env")
	}

	r := gin.Default()

	// API routes
	api.Register(r)

	// Serve Next.js static export
	static := "./static"
	if _, err := os.Stat(static); err == nil {
		r.NoRoute(func(c *gin.Context) {
			// Try to serve the file; fall back to index.html for SPA routing
			path := static + c.Request.URL.Path
			if _, err := os.Stat(path); err == nil && path != static+"/" {
				c.File(path)
				return
			}
			c.File(static + "/index.html")
		})
	} else {
		r.NoRoute(func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "backend running, no frontend built yet"})
		})
	}

	log.Println("Starting mountain-race server on :8003")
	if err := r.Run(":8003"); err != nil {
		log.Fatal(err)
	}
}
