# Go-Scribble

A small real-time multiplayer drawing game (a Scribble/Skribbl clone) built with
Go and a vanilla-JavaScript front end. Players share a canvas over WebSockets:
strokes, chat, the round word and the countdown are all broadcast live, and a
player who joins mid-round is caught up with the current state.

## Features

- Real-time collaborative canvas over WebSockets (`gorilla/websocket`).
- Live chat.
- Automatic rounds: a new word is chosen and the canvas cleared every round.
- Countdown timer broadcast every second.
- Mid-round joiners receive a snapshot (current word, timer and drawing so far).
- Pluggable word source: an offline word list by default, or OpenAI when
  configured.

## Requirements

- Go 1.25+

## Running

```sh
go run .
```

Then open <http://localhost:8080> in one or more browser tabs.

By default words come from a built-in offline list (`LocalWordProvider`), so no
network access or API key is needed. To generate words with OpenAI instead, set
an API key before starting:

```sh
# macOS/Linux
export OPENAI_API_KEY=sk-...
go run .

# Windows (PowerShell)
$env:OPENAI_API_KEY = "sk-..."
go run .
```

## Testing

```sh
go test ./...
```

## Project layout

```
main.go              Composition root: wiring, HTTP server, graceful shutdown.
game/
  message.go         The Message envelope and its type constants (wire protocol).
  hub.go             Client registry + broadcast loop; single-owner, no locks.
  client.go          WebSocket connection: read/write pumps, ping/pong, limits.
  game.go            Round orchestration (pick word -> clear -> announce -> countdown).
  timer.go           Cancellable per-second countdown.
  word.go            WordProvider interface + offline LocalWordProvider.
  openai_word.go     Optional OpenAI-backed WordProvider.
web/
  index.html         Page structure.
  style.css          Styling.
  script.js          Client: rendering, drawing, chat, socket handling.
```

## Design notes

- **Single-owner concurrency.** All shared hub state is read and written only by
  the `Hub.Run` goroutine; other goroutines talk to it through channels, so no
  mutexes are needed.
- **Dependency inversion.** The game depends on the `WordProvider` interface, so
  the word source (offline list, OpenAI, a database, …) can be swapped without
  changing game logic.
- **Backpressure.** A client that cannot keep up with the broadcast rate is
  disconnected rather than allowed to block the hub. The drawing history is
  replayed to new clients as a single message instead of streamed segment by
  segment.
