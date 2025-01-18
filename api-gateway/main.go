// api-gateway/main.go
package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type CalculationResult struct {
	TaskID        string  `json:"task_id"`
	OperationType string  `json:"operation"` // Added this field
	Result        float64 `json:"result"`
	Error         string  `json:"error,omitempty"`
}

type CalculationRequest struct {
	Numbers       []float64 `json:"numbers"`
	OperationType string    `json:"operation"`
}

type AuthResponse struct {
	Valid bool   `json:"valid"`
	Error string `json:"error,omitempty"`
}

// Add pending results map
var (
	pendingResults = make(map[string]chan CalculationResult)
	resultsMutex   sync.RWMutex
)

func main() {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	rabbitmqURI := os.Getenv("RABBITMQ_URI")
	var conn *amqp.Connection
	var ch *amqp.Channel
	var err error

	for i := 0; i < 30; i++ {
		conn, err = amqp.Dial(rabbitmqURI)
		if err == nil {
			ch, err = conn.Channel()
			if err == nil {
				break
			}
		}
		log.Printf("Failed to connect to RabbitMQ, retrying in 2 seconds...")
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ after retries: %v", err)
	}

	defer conn.Close()
	defer ch.Close()

	// Declare result queue
	resultQueue, err := ch.QueueDeclare(
		"result_queue", // name
		true,           // durable
		false,          // delete when unused
		false,          // exclusive
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		log.Fatalf("Failed to declare result queue: %v", err)
	}

	// Start consuming results
	results, err := ch.Consume(
		resultQueue.Name, // queue
		"",               // consumer
		true,             // auto-ack
		false,            // exclusive
		false,            // no-local
		false,            // no-wait
		nil,              // args
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}

	// Handle results in a goroutine
	log.Printf("Starting result handler...")
	go handleResults(results)

	// Routes
	r.POST("/auth/login", handleLogin)

	api := r.Group("/api")
	api.Use(authMiddleware())
	{
		api.POST("/calculate", func(c *gin.Context) {
			handleCalculation(c, ch)
		})
		// Add endpoint to get calculation result
		api.GET("/result/:taskId", handleGetResult)
	}

	r.Run(":8080")
}

func handleResults(results <-chan amqp.Delivery) {
	log.Printf("Starting result handler...")

	for d := range results {
		log.Printf("Received raw message: %s", string(d.Body))

		var result CalculationResult
		if err := json.Unmarshal(d.Body, &result); err != nil {
			log.Printf("Failed to unmarshal result: %v\nRaw message: %s", err, string(d.Body))
			continue
		}

		log.Printf("Received calculation result - Task ID: %s, Type: %s, Result: %f, Error: %s",
			result.TaskID,
			result.OperationType,
			result.Result,
			result.Error)

		resultsMutex.RLock()
		resultChan, exists := pendingResults[result.TaskID]
		resultsMutex.RUnlock()

		if exists {
			log.Printf("Found pending request for task ID: %s", result.TaskID)
			resultChan <- result
			resultsMutex.Lock()
			delete(pendingResults, result.TaskID)
			resultsMutex.Unlock()
		} else {
			log.Printf("No pending request found for task ID: %s", result.TaskID)
		}
	}

	log.Printf("Result handler loop ended - this shouldn't happen!")
}

func handleGetResult(c *gin.Context) {
	taskID := c.Param("taskId")

	resultsMutex.RLock()
	resultChan, exists := pendingResults[taskID]
	resultsMutex.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	select {
	case result := <-resultChan:
		if result.Error != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": result.Error})
			return
		}
		c.JSON(http.StatusOK, result)
	case <-time.After(30 * time.Second):
		c.JSON(http.StatusRequestTimeout, gin.H{"error": "Calculation timed out"})
		resultsMutex.Lock()
		delete(pendingResults, taskID)
		resultsMutex.Unlock()
	}
}

func handleCalculation(c *gin.Context, ch *amqp.Channel) {
	var req CalculationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	taskID := uuid.New().String()

	// Create result channel for this task
	resultChan := make(chan CalculationResult)
	resultsMutex.Lock()
	pendingResults[taskID] = resultChan
	resultsMutex.Unlock()

	msg := struct {
		ID            string    `json:"id"`
		OperationType string    `json:"operation"`
		Numbers       []float64 `json:"numbers"`
	}{
		ID:            taskID,
		OperationType: req.OperationType,
		Numbers:       req.Numbers,
	}

	body, err := json.Marshal(msg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process request"})
		return
	}

	err = ch.Publish(
		"number_ops",
		req.OperationType,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to queue calculation"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"task_id": taskID,
		"message": "Calculation queued",
	})
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No token provided"})
			c.Abort()
			return
		}

		// Remove "Bearer " prefix if present
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		// Create request body
		validationReq := map[string]string{"token": token}
		jsonData, err := json.Marshal(validationReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create validation request"})
			c.Abort()
			return
		}

		// Call auth service to validate token
		authServiceURL := os.Getenv("AUTH_SERVICE_URL")
		log.Printf("Forwarding to auth service at: %s", authServiceURL)
		resp, err := http.Post(
			authServiceURL+"/validate",
			"application/json",
			bytes.NewBuffer(jsonData),
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication service unavailable"})
			c.Abort()
			return
		}
		defer resp.Body.Close()

		var authResp AuthResponse
		if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid auth response"})
			c.Abort()
			return
		}

		if !authResp.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func handleLogin(c *gin.Context) {
	log.Printf("Login request received")
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&credentials); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Marshal credentials to JSON
	jsonData, err := json.Marshal(credentials)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process credentials"})
		return
	}

	// Forward to auth service
	authServiceURL := os.Getenv("AUTH_SERVICE_URL")
	resp, err := http.Post(
		authServiceURL+"/login",
		"application/json",
		bytes.NewBuffer(jsonData),
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication service unavailable"})
		return
	}
	defer resp.Body.Close()

	// Forward auth service response
	var authResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&authResp)
	c.JSON(resp.StatusCode, authResp)
}
