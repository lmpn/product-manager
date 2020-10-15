#!/bin/sh

cd /server
go mod download
go build -o /app
cd /
/app