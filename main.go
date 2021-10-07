package main

import (
	"log"

	rabbitmq "bitbucket.org/latonaio/rabbitmq-golang-client"
)

func main() {
	log.Printf("started")

	mq, err := rabbitmq.NewRabbitmqClient(
		"amqp://guest:guest@192.168.xxx.xx:xxxxxxx",
		[]string{"xxxxxxx"},
		[]string{},
	)
	if err != nil {
		log.Printf("ERROR %v", err)
		return
	}

	log.Printf("connected!")

	iter, err := mq.Iterator()
	if err != nil {
		log.Printf("ERROR %v", err)
	}

	for msg := range iter {
		log.Println("Message received")
		log.Printf("data: %v", msg.Data())

		if err := msg.Success(); err != nil {
			log.Printf("aaaaaaa %v\n", err)
		}
	}
	mq.Stop()
	mq.Close()
}
