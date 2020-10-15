FROM golang:alpine as build-env
ENV GO111MODULE=on

RUN apk update && apk add bash git gcc g++ ca-certificates libc-dev
RUN mkdir server 
WORKDIR /server
COPY src/server ./
RUN chmod +x /server/startServer.sh
RUN go mod download
RUN go build -o /app