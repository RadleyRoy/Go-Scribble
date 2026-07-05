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
ANTHROPIC_API_KEY not set — using local word list (set it in .env to use Claude).
server started at http://localhost:8080
```

(With a key configured, you'll instead see `ANTHROPIC_API_KEY found — checking
Claude access...` followed by `Claude API check OK` — see step 5.)

The server keeps running until you stop it with **Ctrl+C** (it shuts down
gracefully).

## 4. Play

The game needs **at least two players**, so open more than one tab/device.

1. Open <http://localhost:8080>, enter a **name**, and click **Create private
   room**. A short **room code** appears in the header — share it.
2. On another tab/device, open the same URL, enter a name, type the **room code**
   into the join box, and click **Join**. The game starts automatically once two
   players are in the room.
3. Each turn, one player is the **drawer** and **chooses one of three words**;
   everyone else sees a masked hint like `_ _ _ _` that reveals letters over time.
   - **If you are the drawer:** pick a word, then the toolbar unlocks — use the
     **pencil**, **eraser**, **colour**, **brush size**, and **clear** to draw it.
   - **If you are guessing:** type your guess in the chat and press **Enter**. A
     correct guess is hidden from others and scores points — the faster, the
     more. The toolbar stays locked.
4. A turn ends when everyone guesses or the timer hits zero, then the word is
   revealed. After every player has drawn, the round advances. When all rounds
   finish, the final scoreboard shows and a new game begins.

A tab that joins a room mid-turn is automatically caught up with the current
drawing, word hint, timer, and scores. When the last player leaves a room, it is
cleaned up automatically.

> **Play across devices on your network:** find your machine's LAN IP (e.g.
> `192.168.1.20`) and have others open `http://192.168.1.20:8080`. Make sure your
> firewall allows inbound connections on port 8080.

## 5. Optional: AI-generated words (Claude)

By default, the three word choices come from a built-in offline list. To have
**Claude** generate them instead, provide your Anthropic API key (it uses the
official Anthropic Go SDK and the `claude-opus-4-8` model).

**Recommended — a `.env` file.** Create a file named `.env` in the project root:

```
ANTHROPIC_API_KEY=sk-ant-...
```

`.env` is gitignored, so the key stays out of version control, and you don't have
to re-set it every shell. The server loads it automatically on startup (it never
overrides a variable already set in the environment).

**Or a shell environment variable** set **before** `go run .`:

```sh
# macOS / Linux
export ANTHROPIC_API_KEY=sk-ant-...
go run .

# Windows (PowerShell)
$env:ANTHROPIC_API_KEY = "sk-ant-..."
go run .

# Windows (cmd)
set ANTHROPIC_API_KEY=sk-ant-...
go run .
```

On startup the server makes one tiny verification request and logs `Claude API
check OK` (or a clear `FAILED` reason, e.g. a bad key or no credit). Because Opus
4.8 has no temperature knob, the provider asks Claude for a larger, varied pool
and randomly samples the three choices, so the words don't repeat the same
predictable picks each turn. The word fetch runs in the background, so a slow API
never freezes a room, and if the call fails the game falls back to built-in
words — a round is never blocked. The key is read only from the environment/`.env`
and is never logged or committed.

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
| Nothing happens / "Waiting for more players…" | The game needs **two** players in the **same room**. Share the room code and join it in a second tab. |
| "Room not found" | The code was mistyped or the room was empty and got cleaned up. Create a fresh room and share the new code. |
| Page loads but nothing updates between tabs | The WebSocket didn't connect. Check the browser console and that you opened `http://localhost:8080` (not the `file://` path). |
| The toolbar is greyed out | You are guessing, not drawing (or you haven't picked a word yet). Only the current drawer can draw. |
| `go: command not found` | Go isn't installed or not on your `PATH`. Install from <https://go.dev/dl/>. |
| Words look generic even with a key set | The key isn't in `.env` (project root) or wasn't set in the **same** shell before `go run .`, or the Claude call failed — check the startup log for `Claude API check OK` vs `FAILED`. |

## Changing settings

- The listen address is the `listenAddr` constant in `main.go` (default `:8080`).
- The game rules — **topic**, **rounds**, **turn length**, **word-choice time**,
  **reveal time**, and **minimum players** — live in `game.DefaultConfig()` in
  `game/engine.go`:

  ```go
  func DefaultConfig() Config {
      return Config{
          Topic:         "animals", // animals | food | objects | ...
          MaxRounds:     3,
          TurnSeconds:   80,
          ChooseSeconds: 15, // time for the drawer to pick a word
          RevealSeconds: 6,
          MinPlayers:    2,
      }
  }
  ```
