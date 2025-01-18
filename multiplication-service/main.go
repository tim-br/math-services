package main

import (
	"encoding/json"
	"log"
	"os"
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
	OperationType string    `json:"operation_type"`
	Result        float64   `json:"result"`
	Timestamp     time.Time `json:"timestamp"`
}

func main() {
	amqpURI := os.Getenv("AMQP_URI")
	if amqpURI == "" {
		amqpURI = "amqp://guest:guest@rabbitmq:5672/"
	}

	conn, err := amqp.Dial(amqpURI)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	err = ch.ExchangeDeclare(
		"number_ops", // name
		"topic",      // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		log.Fatalf("Failed to declare exchange: %v", err)
	}

	multQueue, err := ch.QueueDeclare(
		"",    // name (let RabbitMQ generate a name)
		false, // durable
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		log.Fatalf("Failed to declare queue: %v", err)
	}

	err = ch.QueueBind(
		multQueue.Name, // queue name
		"multiply",     // routing key
		"number_ops",   // exchange
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to bind queue: %v", err)
	}

	resultQueue, err := ch.QueueDeclare(
		"result_queue", // name
		true,           // durable
		false,          // delete when unused
		false,          // exclusive
		false,          // no-wait
		nil,            // arguments
	)

	msgs, err := ch.Consume(
		multQueue.Name, // queue
		"",             // consumer
		true,           // auto-ack
		false,          // exclusive
		false,          // no-local
		false,          // no-wait
		nil,            // args
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}

	forever := make(chan bool)
	go func() {
		for d := range msgs {
			var req OperationRequest
			err = json.Unmarshal(d.Body, &req)
			if err != nil {
				log.Printf("Error unmarshaling message: %v", err)
				continue
			}

			// Simulate computation time
			time.Sleep(3 * time.Second)

			// Perform multiplication
			product := 1.0
			for _, num := range req.Numbers {
				product *= num
			}

			// Create result with minimal info
			result := OperationResult{
				TaskID:        req.ID,
				OperationType: "multiply",
				Result:        product,
				Timestamp:     time.Now(),
			}

			// Publish result
			resultJSON, err := json.Marshal(result)
			if err != nil {
				log.Printf("Error marshaling result: %v", err)
				continue
			}

			err = ch.Publish(
				"",               // exchange
				resultQueue.Name, // routing key
				false,            // mandatory
				false,            // immediate
				amqp.Publishing{
					ContentType: "application/json",
					Body:        resultJSON,
				})
			if err != nil {
				log.Printf("Error publishing result: %v", err)
			}

			log.Printf("Task %s: Multiplication result = %v", req.ID, product)
		}
	}()

	log.Printf(" [*] Multiplication service waiting for messages")
	<-forever
}
