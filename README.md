# product-manager
The application is containerized with Docker and the services are managed with docker-compose.

## Functionalities:
- API to create products
- API to list product
- API to manage price
- API to list notifications

## Technologies:
- Golang
- PostgreSQL
- RabbitMQ
- Docker
- GRPC
- Docker

## Architecture
![product-manager](https://imgur.com/R5GzOVW)
## Requirements
- protoc-gen-go
- Docker

## Usage
To start run: `startApp.sh`
If the RabbitMQ container exists when you run the start script run: `chmod 600 .docker/rabbitmq/data/.erlang.cookie`

