FROM golang:alpine as build-env
ENV GO111MODULE=on

RUN apk update && apk add bash git gcc g++ ca-certificates libc-dev
RUN mkdir product_service 
WORKDIR /product_service
COPY src/product_service ./
COPY .docker/wait /
RUN chmod +x /product_service/startProductService.sh
RUN chmod +x /wait
RUN go mod download
RUN go build -o /product_service_app
RUN chmod +x /product_service_app