module github.com/bjoelf/fx-collector

go 1.25

replace github.com/bjoelf/saxo-adapter => ../saxo-adapter

require (
	github.com/bjoelf/saxo-adapter v0.0.0-20251122212022-b9d6a0aa59cd
	github.com/joho/godotenv v1.5.1
	golang.org/x/oauth2 v0.33.0 // indirect
)

require github.com/gorilla/websocket v1.5.0 // indirect
