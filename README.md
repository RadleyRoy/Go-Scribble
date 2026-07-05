# Go-Scribble

A real-time multiplayer drawing-and-guessing game in the style of skribbl.io,
built with Go and a vanilla-JavaScript front end. Players take turns drawing a
secret word while everyone else races to guess it in chat for points.

## Gameplay

- **Private rooms.** Create a room to get a short shareable code, or enter a
  code to join friends. Each room is an isolated game.
- Players join with a name and appear in the live scoreboard.
- Each turn one player is the **drawer** and **picks one of three offered words**;
  everyone else sees a masked hint (`_ _ _`) that gradually reveals letters.
- Guessers type in chat. A correct guess is hidden from other players, announced
  as "X guessed the word!", and scored — **faster guesses earn more points**.
- The drawer scores for each player who guesses.
- A turn ends when everyone has guessed or the timer runs out; the word is then
  revealed. After every player has drawn, the round advances.
- After the configured number of rounds the final scoreboard is shown and a new
  game begins automatically.
- Only the current drawer can draw or clear the canvas.
- Players who join mid-turn are caught up with the current drawing and state.
- Empty rooms are torn down automatically when the last player leaves.

## Requirements

- Go 1.25+

## Running

```sh
go run .
```

Open <http://localhost:8080>, enter a name, and **create a room**. Open the URL
again in another tab/device, enter the same **room code**, and play. See
**RUNNING.md** for a detailed guide, LAN play, and troubleshooting.

Words come from a built-in offline list by default. To have Claude generate the
word choices instead, provide an `ANTHROPIC_API_KEY` before starting the server
(uses the official Anthropic Go SDK and the `claude-opus-4-8` model). The key can
come from a real environment variable or from a gitignored **`.env`** file in the
project root:

```sh
echo 'ANTHROPIC_API_KEY=sk-ant-...' > .env
go run .
```

On startup the server verifies Claude access and logs a clear `Claude API check
OK` / `FAILED` result. Because Opus 4.8 has no temperature control, the provider
asks Claude for a larger, deliberately varied pool of words and randomly samples
the three choices from it, so successive turns don't collapse onto the same
predictable picks. If a call fails, it falls back to the offline list so a round
is never blocked. The key is never logged or committed. See **RUNNING.md** for
per-shell instructions.

## Testing

```sh
go test ./...
```

Tests live in the top-level `tests/` package and exercise the code through its
public API, including a real two-player game played over WebSockets.

## Project layout

```
main.go              Composition root: wiring, HTTP + /api/rooms, shutdown.
game/
  message.go         Wire protocol: envelope, state, chat, choices, player views.
  engine.go          The game: single-goroutine state machine (word choice, turns,
                     guessing, scoring, rounds, hints, phases, disconnects).
  room.go            RoomManager: private rooms, code generation, cleanup.
  player.go          Per-player state.
  client.go          WebSocket connection: read/write pumps, ping/pong, limits.
  word.go            WordProvider interface + offline LocalWordProvider.
  claude_word.go     Optional Claude-backed WordProvider (Anthropic Go SDK).
web/
  index.html         Page structure (lobby, canvas, players, chat, word choice).
  style.css          Styling.
  script.js          Client: lobby, state rendering, drawing, guessing, sockets.
tests/
  word_test.go       Local word-provider tests.
  claude_word_test.go Claude provider against a stub Anthropic server.
  room_test.go       Room creation, codes, and empty-room cleanup.
  engine_test.go     End-to-end WebSocket game-flow test (room + word choice).
.github/workflows/
  ci.yml             CI: build, vet, test, and smoke-run (also runnable from the
                     Actions tab via workflow_dispatch).
RUNNING.md           Detailed run/play guide, LAN play, and troubleshooting.
```

## Continuous integration

`.github/workflows/ci.yml` runs on every push/PR to `main` and can be triggered
manually from the repository's **Actions** tab (workflow_dispatch). It verifies
the project **builds**, passes `go vet`, has all **tests** green, and **runs**
(a smoke start of the server). The Claude tests use a stub server, so CI needs no
API key.

## Design notes

- **Single-owner concurrency.** All game and connection state is read and written
  only by the `Engine.Run` goroutine; other goroutines interact through channels,
  so no mutexes are needed.
- **Dependency inversion.** The engine depends on the `WordProvider` interface,
  so the word source (offline list, Claude, a database, …) can be swapped freely.
- **Room isolation.** Each private room runs its own `Engine` on its own
  goroutine; the `RoomManager` owns their lifecycles and reaps empties.
- **Non-blocking word fetch.** The (possibly networked) word provider is called
  in a goroutine so a slow API can never freeze a room's timers.
- **Configurable timing.** `game.Config` makes round/turn/reveal durations
  injectable, which keeps the engine fully testable (a whole game runs in
  milliseconds in the integration test).
- **Fixed drawing resolution.** The canvas draws into a fixed logical space
  (`LogicalWidth`×`LogicalHeight`) that CSS scales to the window. This keeps
  drawings aligned across clients of different sizes and prevents them from being
  lost on window resize.
- **Backpressure.** A client that can't keep up with broadcasts is disconnected
  rather than allowed to block the engine; the drawing so far is replayed to new
  clients as a single message.

## Possible next steps

Avatars, an undo tool, a fill bucket, persisted stats/leaderboards, and a
"kick"/host role are natural extensions of the current design.
