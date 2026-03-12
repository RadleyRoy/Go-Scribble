package main

import (
	"go-scribble/game" // Replace 'doodle-royale' with your module name in go.mod
	"log"
	"net/http"
)

func main() {
	hub := game.NewHub()
	go hub.Run()

	// Serve static files from the 'web' folder
	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	// WebSocket endpoint
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		game.HandleConnections(hub, w, r)
	})

	log.Println("Server started at http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
