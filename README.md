# Microservices Math Calculator

A distributed calculator system built with Go microservices, RabbitMQ for message queuing, and JWT authentication. For local testing only.

## Project Structure

```
.
├── add.sh                     # Script to test addition service
├── addition-service/          # Microservice for addition operations
├── api-gateway/              # API Gateway service
├── auth-service/             # Authentication service
├── docker-compose.yml        # Docker compose configuration
├── multiplication-service/    # Microservice for multiplication operations
├── multiply.sh               # Script to test multiplication service
└── test-client/             # Test client application
```

## Services

### API Gateway
- Main entry point for all requests
- Handles authentication via JWT
- Routes calculation requests to appropriate services
- Manages asynchronous responses via result queue
- Endpoints:
  - POST `/auth/login`: Authentication endpoint
  - POST `/api/calculate`: Submit calculation requests
  - GET `/api/result/:taskId`: Get calculation results

### Auth Service
- Manages user authentication
- Issues JWT tokens
- Validates tokens for protected routes
- Default credentials: admin/admin (for development only)

### Addition Service
- Handles addition operations
- Consumes messages from RabbitMQ queue
- Returns results to result queue

### Multiplication Service
- Handles multiplication operations
- Consumes messages from RabbitMQ queue
- Returns results to result queue

## Message Queue Structure

- Exchange: "number_ops" (topic)
- Routing Keys:
  - "add" for addition operations
  - "multiply" for multiplication operations
- Result Queue: "result_queue" for operation results

## Setup & Running

1. Start all services:
```bash
docker-compose up
```

2. Get an authentication token:
```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin"}'
```

3. Make a calculation request:
```bash
curl -X POST http://localhost:8080/api/calculate \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"numbers": [2,3,4], "operation": "multiply"}'
```

4. Get the result using the returned task ID:
```bash
curl -X GET http://localhost:8080/api/result/TASK_ID \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## Technical Details

- Built with Go 1.21+
- Uses RabbitMQ for message queuing
- JWT for authentication
- Docker for containerization
- Gin web framework for API Gateway
- Implements circuit breaker pattern for service resilience
- Uses environment variables for configuration

## Environment Variables

```
JWT_SECRET=your-super-secret-key-
RABBITMQ_USER=your_username
RABBITMQ_PASSWORD=your_secure_password
REDIS_PASSWORD=super_secure
```

## Development Notes

- Services use graceful shutdown
- Implemented retry mechanisms for RabbitMQ connections
- Result queue is durable to prevent message loss
- Timeouts implemented for result retrieval (30s)
- Concurrent request handling with goroutines
- Thread-safe result tracking with mutex

## Error Handling

- Circuit breaker for external service calls
- Timeout handling for long-running calculations
- Proper error propagation through the system
- Detailed logging for debugging

## Security Considerations

- JWT-based authentication
- Protected routes in API Gateway
- Environment variables for sensitive data
- No hardcoded credentials in production

## Future Improvements

- Implement user management
- Add request validation
- Add metrics and monitoring
- Implement rate limiting
- Add OpenAPI documentation
- Add unit and integration tests
- Implement proper user management
- Add more mathematical operations
