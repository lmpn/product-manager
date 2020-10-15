package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	servicespb "product_service/proto"

	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
	"github.com/streadway/amqp"
	"google.golang.org/grpc"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

// Product is a struct that models the records to store in the DB
type Product struct {
	ID    int    `sql:"ID,pk"`
	Name  string `pg:",unique"`
	Desc  string
	Price float64
}

// MessageQueue stores all the necessary variables to use RabbitMQ
type MessageQueue struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
	Queue   amqp.Queue
}

// ProductServiceState stores all the necessary variables to use RabbitMQ
type ProductServiceState struct {
	DbConn *pg.DB
	Queue  MessageQueue
}

// New function create a connection to PostgreSQL, creates the necessary schemas, and sets up the messaging queue
func New() *ProductServiceState {
	db := pg.Connect(&pg.Options{
		Addr:     os.Getenv("PGURL"),
		User:     os.Getenv("PGUSER"),
		Password: os.Getenv("PGPASS"),
	})

	ps := &ProductServiceState{DbConn: db}
	ps.CreateSchema()
	ps.SetupMessagingQueue()
	return ps
}

//Delete closes the connection to PostgreSQL and the connection & channel of RabbitMQ
func (ps *ProductServiceState) Delete() {
	ps.DbConn.Close()
	ps.Queue.Conn.Close()
	ps.Queue.Channel.Close()
}

//CreateSchema creates the product schema in PostgreSQL
func (ps *ProductServiceState) CreateSchema() error {
	models := []interface{}{
		(*Product)(nil),
	}

	for _, model := range models {
		err := ps.DbConn.Model(model).CreateTable(&orm.CreateTableOptions{
			Temp:        false,
			IfNotExists: true,
		})
		failOnError(err, "Error creating DB model")
	}
	return nil
}

//StartService is function to execute in a goroutine that uses a GRPC server and a listener to handle requests
func StartService(s *grpc.Server, lis net.Listener) {
	fmt.Println("Starting Server...")
	err := s.Serve(lis)
	failOnError(err, "Error starting server")

}

//CreateProduct is the function implementation of ProductService's interface.
//The insertion fails if exists a record with the same name.
func (ps *ProductServiceState) CreateProduct(ctx context.Context, request *servicespb.CreateProductRequest) (*servicespb.CreateProductResponse, error) {
	product := &Product{Name: request.GetName(),
		Desc:  request.GetDesc(),
		Price: request.GetPrice(),
	}
	result, err := ps.DbConn.Model(product).OnConflict("(name) DO NOTHING").Insert()
	if err != nil {
		fmt.Println("DBError insert product: %v\n", err)
		return nil, err
	}
	if result.RowsAffected() > 0 {
		return &servicespb.CreateProductResponse{Result: "Product inserted"}, nil

	}
	return &servicespb.CreateProductResponse{Result: "Product already exists"}, nil
}

//ListAllProducts is the function implementation of ProductService's interface..
//This function sends to the client all the existing products.
func (ps *ProductServiceState) ListAllProducts(ctx context.Context, request *servicespb.ListAllRequest) (*servicespb.ListAllResponse, error) {
	var prods []Product
	var pbProds []*servicespb.Product
	err := ps.DbConn.Model(&prods).Select()
	if err != nil {
		fmt.Println("DBError on select: %v\n", err)
		return nil, err
	}
	for _, p := range prods {
		pbProduct := &servicespb.Product{Name: p.Name,
			Desc:  p.Desc,
			Price: p.Price}
		pbProds = append(pbProds, pbProduct)
	}
	return &servicespb.ListAllResponse{Product: pbProds}, nil
}

//ChangePrice is the function implementation of ProductService's interface.
//This function updates the price of a product and sends a message to the notification service through RabbitMQ
func (ps *ProductServiceState) ChangePrice(ctx context.Context, request *servicespb.ChangePriceRequest) (*servicespb.ChangePriceResponse, error) {
	product := &Product{Name: request.GetName(), Price: request.GetPrice()}
	result, err := ps.DbConn.Model(product).Where("product.name = ?", product.Name).Column("price").Update()
	if err != nil {
		fmt.Println("DBError update price: %v\n", err)
		return nil, err
	}
	if result.RowsAffected() > 0 {
		ps.sendMessage(product.Name, product.Price)
		return &servicespb.ChangePriceResponse{Result: "Price updated"}, nil
	}
	return &servicespb.ChangePriceResponse{Result: "Inexistent product"}, nil
}

//SetupMessagingQueue functions create the connection, channel, and queue to use when it's necessary to send message to the notification service.
func (ps *ProductServiceState) SetupMessagingQueue() {
	conn, err := amqp.Dial("amqp://rabbitmq:5672/")
	failOnError(err, "Error creating RabbitMQ connection")
	ch, err := conn.Channel()
	failOnError(err, "Error creating RabbitMQ channel")
	q, err := ch.QueueDeclare(
		"notifications_queue", // name
		false,                 // durable
		false,                 // delete when unused
		false,                 // exclusive
		false,                 // no-wait
		nil,                   // arguments
	)
	failOnError(err, "Error creating RabbitMQ queue")

	ps.Queue.Conn = conn
	ps.Queue.Channel = ch
	ps.Queue.Queue = q
}

func (ps *ProductServiceState) sendMessage(name string, price float64) {
	body := servicespb.Notification{Name: name, Price: price}
	msg, _ := json.Marshal(body)
	err := ps.Queue.Channel.Publish(
		"",                  // exchange
		ps.Queue.Queue.Name, // routing key
		false,               // mandatory
		false,               // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(msg),
		})
	if err != nil {
		fmt.Println("Error sending message: %v\n", err)
	}
}

func main() {
	listener, err := net.Listen("tcp", ":4000")
	if err != nil {
		log.Fatalf("Error in listening: %v\n", err)
	}
	fmt.Println("Product service starting")
	s := grpc.NewServer()
	sv := New()

	servicespb.RegisterProductServiceServer(s, sv)

	go StartService(s, listener)
	fmt.Println("Product service running")
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch

	sv.Delete()
	fmt.Println("Closing the listener")
	if err := listener.Close(); err != nil {
		log.Fatalf("Error on closing the listener : %v", err)
	}
	fmt.Println("Stopping the product service")
	s.Stop()

}
