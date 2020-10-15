#!/bin/sh

cd /product_service
go mod download
go build -o /product_service_app
/wait && /product_service_app