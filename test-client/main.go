// test-client/main.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type OperationRequest struct {
	ID            string    `json:"id"`
	OperationType string    `json:"operation"`
	Numbers       []float64 `json:"numbers"`
}

type OperationResult struct {
	TaskID        string    `json:"task_id"`
	OperationType string    `json:"operation"`
	Result        float64   `json:"result"`
	Timestamp     time.Time `json:"timestamp"`
}

func main() {
	// Connect to RabbitMQ
	conn, err := amqp.Dial("amqp://your_username:your_secure_password@localhost:5672/")
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	// Declare the result queue to receive responses
	resultQueue, err := ch.QueueDeclare(
		"result_queue", // name
		true,          // durable
		false,         // delete when unused
		false,         // exclusive
		false,         // no-wait
		nil,          // arguments
	)
	if err != nil {
		log.Fatalf("Failed to declare result queue: %v", err)
	}

	// Start consuming results
	msgs, err := ch.Consume(
		resultQueue.Name, // queue
		"",              // consumer
		true,            // auto-ack
		false,           // exclusive
		false,           // no-local
		false,           // no-wait
		nil,             // args
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}

	// Create a channel to receive results
	go func() {
		for d := range msgs {
			var result OperationResult
			err := json.Unmarshal(d.Body, &result)
			if err != nil {
				log.Printf("Error parsing result: %v", err)
				continue
			}
			fmt.Printf("Received result for task %s: %s = %v\n", 
				result.TaskID, result.OperationType, result.Result)
		}
	}()

	// Test addition
	addReq := OperationRequest{
		ID:            "add-123",
		OperationType: "add",
		Numbers:       []float64{10, 20, 30},
	}

	addJSON, _ := json.Marshal(addReq)
	err = ch.Publish(
		"number_ops", // exchange
		"add",        // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        addJSON,
		})
	if err != nil {
		log.Printf("Error publishing addition request: %v", err)
	}

	// Test multiplication
	multReq := OperationRequest{
		ID:            "mult-123",
		OperationType: "multiply",
		Numbers:       []float64{2, 3, 4},
	}

	multJSON, _ := json.Marshal(multReq)
	err = ch.Publish(
		"number_ops", // exchange
		"multiply",   // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        multJSON,
		})
	if err != nil {
		log.Printf("Error publishing multiplication request: %v", err)
	}

	fmt.Println("Test requests sent. Waiting for results...")
	// Wait for a bit to receive results
	time.Sleep(5 * time.Second)
}
