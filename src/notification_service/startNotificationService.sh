#!/bin/sh

cd /notification_service
go mod download
go build -o /notification_service_app
/wait && /notification_service_app