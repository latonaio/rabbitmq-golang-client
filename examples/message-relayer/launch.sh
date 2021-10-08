go mod download
go mod tidy

RABBITMQ_URL=amqp://guest:guest@rabbitmq:5672/%2F \
QUEUE_FROM=test_a \
QUEUE_TO=test_b \
go run main.go
