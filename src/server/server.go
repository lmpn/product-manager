package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	servicespb "server/proto"

	"github.com/golang/protobuf/jsonpb"
	"github.com/gorilla/mux"
	"google.golang.org/grpc"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

type productService struct {
	Client servicespb.ProductServiceClient
	Cc     *grpc.ClientConn
}

type notificationService struct {
	Client servicespb.NotificationServiceClient
	Cc     *grpc.ClientConn
}

type ServerState struct {
	PService productService
	NService notificationService
}

var ss ServerState

func (ss *ServerState) connectProductService() {
	cc, err := grpc.Dial("productservice:4000",
		grpc.WithInsecure())
	failOnError(err, "Could not connect to product service")
	ss.PService = productService{Client: servicespb.NewProductServiceClient(cc), Cc: cc}
}

func (ss *ServerState) connectNotificationService() {
	cc, err := grpc.Dial("notificationservice:4001",
		grpc.WithInsecure())
	failOnError(err, "Could not connect to notification service")
	ss.NService = notificationService{Client: servicespb.NewNotificationServiceClient(cc), Cc: cc}
}

func (ss *ServerState) connect() {
	ss.connectNotificationService()
	ss.connectProductService()
}

func (ss *ServerState) close() {
	ss.NService.Cc.Close()
	ss.PService.Cc.Close()
}

func newestNotifications(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint Hit: notifications")
	request := &servicespb.NotificationRequest{}
	response, err := ss.NService.Client.NewestNotifications(context.Background(), request)
	if err == nil {
		json.NewEncoder(w).Encode(response.GetNotification())
	} else {
		fmt.Println("Error: %v\n", err)
		json.NewEncoder(w).Encode("Error establishing connect to notifications service")
	}
}

func returnProductsPage(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint Hit: returnProducts")
	vars := mux.Vars(r)
	page, pageErr := strconv.Atoi(vars["page"])
	limit, limitErr := strconv.Atoi(vars["limit"])
	if pageErr != nil {
		fmt.Println("Error: %v\n", pageErr)
		json.NewEncoder(w).Encode("Incorrect page data")
		return
	}
	if limitErr != nil {
		fmt.Println("Error: %v\n", limitErr)
		json.NewEncoder(w).Encode("Incorrect limit data")
		return
	}
	var limit32 int32
	var page32 int32
	limit32 = int32(limit)
	page32 = int32(page)
	request := &servicespb.PageRequest{Page: page32, Limit: limit32}
	response, err := ss.PService.Client.ProductsPage(context.Background(), request)

	if err == nil {
		json.NewEncoder(w).Encode(response.GetProduct())
	} else {
		fmt.Println("Error: %v\n", err)
		json.NewEncoder(w).Encode("Error establishing connect to product service")
	}
}

//C - POST
func createProduct(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint Hit: createProduct")
	var request servicespb.CreateProductRequest
	err := jsonpb.Unmarshal(r.Body, &request)
	if err == nil {
		response, rerr := ss.PService.Client.CreateProduct(context.Background(), &request)
		if rerr == nil {
			json.NewEncoder(w).Encode(response.GetResult())
		} else {
			fmt.Println("Error: %v\n", rerr)
			json.NewEncoder(w).Encode("Error in insertion")
		}
	} else {
		fmt.Println("Error: %v\n", err)
		json.NewEncoder(w).Encode("Error in insertion: incorrect data")
	}
}

func changePrice(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint Hit: changePrice")
	vars := mux.Vars(r)
	name := vars["name"]
	reqBody, _ := ioutil.ReadAll(r.Body)
	var jsonBody map[string]float64
	err := json.Unmarshal(reqBody, &jsonBody)
	price, ok := jsonBody["price"]
	if err == nil && ok {
		request := &servicespb.ChangePriceRequest{Name: name, Price: price}
		response, rerr := ss.PService.Client.ChangePrice(context.Background(), request)
		if rerr == nil {
			json.NewEncoder(w).Encode(response.GetResult())
		} else {
			fmt.Println("Error: %v\n", rerr)
			json.NewEncoder(w).Encode("Error in update")
		}
	} else {
		fmt.Println("Error: %v\n", err)
		json.NewEncoder(w).Encode("Error in update: incorrect data")
	}
}

func handleRequests(sv *http.Server, router *mux.Router) {
	router.HandleFunc("/api/v1/createProduct", createProduct).Methods("POST")
	router.HandleFunc("/api/v1/changePrice/{name}", changePrice).Methods("POST")
	router.HandleFunc("/api/v1/notifications", newestNotifications)
	router.HandleFunc("/api/v1/products/page={page}&limit={limit}", returnProductsPage)
	if err := sv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func main() {
	fmt.Println("Starting server at port 10000...")
	ss.connect()
	defer ss.close()
	router := mux.NewRouter().StrictSlash(true)
	server := &http.Server{Addr: ":10000", Handler: router}

	go handleRequests(server, router)

	// Setting up signal capturing
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop
	fmt.Println("Gracefull shutdown...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		panic(err)
	}
}
