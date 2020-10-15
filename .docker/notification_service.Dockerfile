FROM golang:alpine as build-env
ENV GO111MODULE=on

RUN apk update && apk add bash git gcc g++ ca-certificates libc-dev
RUN mkdir notification_service
WORKDIR /notification_service
COPY src/notification_service ./
COPY .docker/wait /
RUN chmod +x /notification_service/startNotificationService.sh
RUN chmod +x /wait
RUN go mod download
RUN go build -o /notification_service_app
RUN chmod +x /notification_service_app