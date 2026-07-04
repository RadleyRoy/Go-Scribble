# Running Go-Scribble

A step-by-step guide to running and playing the game locally.

## 1. Prerequisites

- **Go 1.25 or newer** — check with:
  ```sh
  go version
  ```
- A modern web browser (Chrome, Firefox, Edge, Safari).

No database, API key, or internet access is required for the default setup.

## 2. Get the dependencies

From the project root (the folder containing `go.mod`):

```sh
go mod download
```

This fetches `gorilla/websocket`. You only need to do this once.

## 3. Start the server

```sh
go run .
```

You should see:

```
using local word provider (set OPENAI_API_KEY to use OpenAI)
server started at http://localhost:8080
```

The server keeps running until you stop it with **Ctrl+C** (it shuts down
gracefully).

## 4. Play

1. Open <http://localhost:8080> in your browser.
2. Open the same URL in **one or more additional tabs or devices** on the same
   machine to simulate multiple players.
3. Draw on the canvas — every stroke appears live in the other tabs.
4. Use the toolbar to switch between the **pencil** and **eraser**, pick a
   **colour**, or **clear** the canvas for everyone.
5. Type in the chat box and press **Enter** or **Send**.
6. The header shows the round's **word slots** and a **countdown**. When the
   timer hits zero a new round starts automatically: the canvas clears and a new
   word is chosen.

A tab that joins mid-round is automatically caught up with the current word,
timer, and everything drawn so far.

> **Play across devices on your network:** find your machine's LAN IP (e.g.
> `192.168.1.20`) and have others open `http://192.168.1.20:8080`. Make sure your
> firewall allows inbound connections on port 8080.

## 5. Optional: AI-generated words (OpenAI)

By default, words come from a built-in offline list. To have OpenAI pick the
words instead, set an API key **before** starting the server:

```sh
# macOS / Linux
export OPENAI_API_KEY=sk-...
go run .

# Windows (PowerShell)
$env:OPENAI_API_KEY = "sk-..."
go run .

# Windows (cmd)
set OPENAI_API_KEY=sk-...
go run .
```

If the OpenAI call fails for any reason, the game logs a warning and falls back
to a default word, so a round is never blocked.

## 6. Build a standalone binary (optional)

```sh
# Current platform
go build -o go-scribble .
./go-scribble          # ./go-scribble.exe on Windows
```

The `web/` folder is served relative to the working directory, so run the binary
from the project root (or copy `web/` next to it).

## 7. Run the tests

```sh
go test ./...
```

## Troubleshooting

| Symptom | Cause / fix |
| --- | --- |
| `bind: address already in use` (or port 8080 busy) | Another process is on 8080. Stop it, or change `listenAddr` in `main.go`. |
| Page loads but nothing updates between tabs | The WebSocket didn't connect. Check the browser console and that you opened `http://localhost:8080` (not the `file://` path). |
| `go: command not found` | Go isn't installed or not on your `PATH`. Install from <https://go.dev/dl/>. |
| Words look generic even with a key set | The env var wasn't set in the **same** shell before `go run .`, or the OpenAI call failed (see server logs). |

## Changing settings

Common knobs live as constants at the top of `main.go`:

- `listenAddr` — the address/port to listen on (default `:8080`).
- `roundTopic` — the word topic (`animals`, `food`, `objects`, …).
- `roundSeconds` — the length of each round in seconds.
