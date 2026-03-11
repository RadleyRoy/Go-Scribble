package main

import (
	"go-scribble/game"
	"log"
	"net/http"
)

func main() {

	hub := game.NewHub()
	go hub.Run()

	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		game.HandleWebSocket(hub, w, r)
	})

	log.Println("Server running on :8080")
	http.ListenAndServe(":8080", nil)
}
