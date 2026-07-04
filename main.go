package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"go-scribble/game"
)

const (
	listenAddr   = ":8080"
	roundTopic   = "animals"
	roundSeconds = 80
	webDir       = "./web"
)

func main() {
	// The hub owns all shared state on its own goroutine.
	hub := game.NewHub()
	go hub.Run()

	// Rounds stop cleanly when an interrupt arrives.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	gameLoop := game.NewGame(hub, newWordProvider(), roundTopic, roundSeconds)
	go gameLoop.Run(ctx)

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(webDir)))
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		game.ServeWS(hub, w, r)
	})

	server := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("server started at http://localhost%s", listenAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

// newWordProvider selects the AI-backed provider when an OpenAI key is present
// and otherwise falls back to the dependency-free local provider.
func newWordProvider() game.WordProvider {
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		log.Println("using OpenAI word provider")
		return game.NewOpenAIWordProvider(key)
	}
	log.Println("using local word provider (set OPENAI_API_KEY to use OpenAI)")
	return game.NewLocalWordProvider()
}
