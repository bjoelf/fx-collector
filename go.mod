module github.com/bjoelf/fx-collector

go 1.25

replace github.com/bjoelf/saxo-adapter => ../saxo-adapter

require (
	github.com/bjoelf/saxo-adapter v0.0.0-20251122212022-b9d6a0aa59cd
	github.com/joho/godotenv v1.5.1
	golang.org/x/oauth2 v0.15.0
)

require (
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	golang.org/x/net v0.19.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
)
