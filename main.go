package main

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"go-scribble/game"
)

const (
	listenAddr = ":8080"
	webDir     = "./web"
)

func main() {
	// Load secrets (e.g. ANTHROPIC_API_KEY) from a gitignored .env file if present.
	loadDotEnv(".env")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Each private room runs its own engine; the manager owns their lifecycles.
	rooms := game.NewRoomManager(newWordProvider(), game.DefaultConfig())

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(webDir)))
	mux.HandleFunc("/api/rooms", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		code := rooms.Create()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"code": code})
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		game.ServeWS(rooms, w, r)
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

// newWordProvider uses Claude when ANTHROPIC_API_KEY is set, otherwise the
// dependency-free local provider.
func newWordProvider() game.WordProvider {
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		log.Println("using Claude word provider")
		return game.NewClaudeWordProvider(key)
	}
	log.Println("using local word provider (set ANTHROPIC_API_KEY to use Claude)")
	return game.NewLocalWordProvider()
}

// loadDotEnv reads simple KEY=VALUE lines from a .env file into the process
// environment without overriding variables that are already set. A missing file
// is ignored, so real environment variables keep working in production. The file
// is gitignored, which keeps secrets like the API key out of version control.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // no .env file — that's fine
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`) // strip optional surrounding quotes
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}
