module github.com/bjoelf/fx-collector

go 1.25

require (
	github.com/bjoelf/saxo-adapter v0.4.1
	github.com/joho/godotenv v1.5.1
	golang.org/x/oauth2 v0.33.0 // indirect
)

require github.com/gorilla/websocket v1.5.0 // indirect

// Use local saxo-adapter for development
// replace github.com/bjoelf/saxo-adapter => ../saxo-adapter
