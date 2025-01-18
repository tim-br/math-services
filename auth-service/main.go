// auth-service/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt"
)

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

func main() {
	log.Printf("Auth service starting up...")

	// Initialize Gin
	r := gin.Default()

	// Connect to Redis
	opt, _ := redis.ParseURL(os.Getenv("REDIS_URL"))

	rdb := redis.NewClient(opt)

	// Test Redis connection
	_, err := rdb.Ping(context.Background()).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Auth endpoints - using root path
	r.POST("/login", func(c *gin.Context) {
		log.Printf("Login request received")
		var user User
		if err := c.ShouldBindJSON(&user); err != nil {
			log.Printf("Error binding JSON: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		log.Printf("Attempting login for user: %s", user.Username)

		// For demo purposes - in production, check against database
		if user.Username != "admin" || user.Password != "admin" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		// Generate token
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"username": user.Username,
			"exp":      time.Now().Add(time.Hour * 24).Unix(),
		})

		tokenString, err := token.SignedString(jwtSecret)
		if err != nil {
			log.Printf("Error generating token: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
			return
		}

		// Store in Redis
		err = rdb.Set(context.Background(), tokenString, user.Username, time.Hour*24).Err()
		if err != nil {
			log.Printf("Error storing in Redis: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not store token"})
			return
		}

		log.Printf("Login successful for user: %s", user.Username)
		c.JSON(http.StatusOK, gin.H{
			"token": tokenString,
			"type":  "Bearer",
		})
	})

	r.POST("/validate", func(c *gin.Context) {
		var req struct {
			Token string `json:"token"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"valid": false, "error": "Invalid request"})
			return
		}

		token, err := jwt.Parse(req.Token, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusOK, gin.H{"valid": false, "error": "Invalid token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"valid": true})
	})

	// Start server on port 8081
	log.Printf("Auth service listening on :8081")
	if err := r.Run(":8081"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
