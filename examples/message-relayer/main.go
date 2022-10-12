package main

import (
	"log"
	"os"

	rabbitmq "github.com/latonaio/rabbitmq-golang-client"
)

func main() {
	log.Printf("started")

	url := os.Getenv("RABBITMQ_URL")
	queueOrigin := os.Getenv("QUEUE_ORIGIN")
	queueTo := os.Getenv("QUEUE_TO")

	mq, err := rabbitmq.NewRabbitmqClient(
		url,
		[]string{queueOrigin},
		[]string{queueTo},
	)
	if err != nil {
		log.Printf("failed to create RabbitmqClient: %v", err)
		return
	}
	log.Printf("connected!")
	defer mq.Close()

	iter, err := mq.Iterator()
	if err != nil {
		log.Printf("failed to create iterator: %v", err)
		return
	}
	defer mq.Stop()

	for msg := range iter {
		log.Printf("received from: %v", msg.QueueName())
		log.Printf("data: %v", msg.Data())

		if err := mq.Send(queueTo, msg.Data()); err != nil {
			log.Printf("failed to send message: %v", err)
		}

		if err := msg.Success(); err != nil {
			log.Printf("failed to send success response: %v", err)
		}
	}
}
