package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	servicespb "notification_service/proto"
	"os"
	"os/signal"

	"github.com/streadway/amqp"
	"google.golang.org/grpc"
)

//NotificationServiceState stores the 10 most recent notifications in an array
type NotificationServiceState struct {
	newestNotifications []*servicespb.Notification
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func startService(s *grpc.Server, lis net.Listener) {
	fmt.Println("Starting Server...")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func (ns *NotificationServiceState) startMonitoring() {
	conn, err := amqp.Dial("amqp://rabbitmq:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"notifications_queue", // name
		false,                 // durable
		false,                 // delete when unused
		false,                 // exclusive
		false,                 // no-wait
		nil,                   // arguments
	)
	failOnError(err, "Failed to declare a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	for d := range msgs {
		msg := servicespb.Notification{}
		err := json.Unmarshal(d.Body, &msg)
		if err != nil {
			log.Println("%s: %s", msg, err)
		} else if len(ns.newestNotifications) > 10 {
			ns.newestNotifications = append(ns.newestNotifications[1:], &msg)
		} else {
			ns.newestNotifications = append(ns.newestNotifications, &msg)
		}
	}
}

//NewestNotifications function implementation of NotificationService
//Returns the latest notifications
func (ns *NotificationServiceState) NewestNotifications(ctx context.Context, request *servicespb.NotificationRequest) (*servicespb.NotificationResponse, error) {
	return &servicespb.NotificationResponse{Notification: ns.newestNotifications}, nil
}

func main() {
	listener, err := net.Listen("tcp", ":4001")
	if err != nil {
		log.Fatalf("Error in listening: %v\n", err)
	}
	fmt.Println("Notification service starting")
	s := grpc.NewServer()
	sv := &NotificationServiceState{}
	servicespb.RegisterNotificationServiceServer(s, sv)
	go sv.startMonitoring()
	go startService(s, listener)
	fmt.Println("Notification service running")
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch

	fmt.Println("Closing the listener")
	if err := listener.Close(); err != nil {
		log.Fatalf("Error on closing the listener : %v", err)
	}
	fmt.Println("Stopping the Notification service")
	s.Stop()

}
