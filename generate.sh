protoc -I=. proto/services.proto --go_out=plugins=grpc:src/server
protoc -I=. proto/services.proto --go_out=plugins=grpc:src/product_service
protoc -I=. proto/services.proto --go_out=plugins=grpc:src/notification_service
