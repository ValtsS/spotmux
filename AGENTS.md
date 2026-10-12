# AGENTS.md — SpotMux

## Project overview

SpotMux is a lightweight DX (Distance X) Spot Aggregator + Telnet server for amateur radio.
It connects to multiple upstream DX clusters, aggregates/dedups/filters spots, and serves a
unified stream to downstream telnet clients (loggers). Downstream `DX` spots are rebroadcast upstream.

- Language: Go `1.25.6` (`go.mod`), **stdlib only — no external dependencies**.
- Module: `github.com/valtss/spotmux`
- Entry: `cmd/spotmux/main.go` (`-config config.json` flag, default `config.json`).
- Runtime: single binary, designed to run in Docker as non-root user `spotmux` (see `Dockerfile`).
- Protocol: plain TCP telnet, DXSpider-compatible prompts (`login:`, `de <CALL> >`, `DX de ...` lines).

## Repository layout

```
cmd/spotmux/main.go       # wiring: config → cache/dedup/filter → cluster manager + telnet server → glue loops + signals
internal/config/config.go # Config struct + LoadConfig() with defaults
internal/cluster/manager.go # UpstreamManager: dial upstream clusters, handshake, read loop, BroadcastSpot()
internal/spot/spot.go     # Spot struct, String(), ParseSpot(), ShouldIgnore()
internal/spot/cache.go    # Cache: thread-safe last-N ring (slice shift)
internal/spot/deduplication.go # Deduplicator: freq+call key, time window, Cleanup()
internal/spot/filter.go   # PrefixFilter: length-bucketed case-insensitive prefix match
internal/spot/spot_test.go # only tests in repo (filter behavior)
internal/telnet/server.go # Server: accept loop, per-client login/cache replay, DX/HELP/#SPOT/BYE, broadcastLoop()
internal/telnet/rate_limiter.go # Token-bucket per-minute limiter
internal/metrics/metrics.go # atomic counters + StartPrinter() log ticker
config.json               # example runtime config (service callsign, upstreams, ignored prefixes)
Dockerfile                # multi-stage: golang:alpine build → alpine runtime + ca-certificates/tzdata
```

## Data flow (must preserve)

1. `UpstreamManager.Start()` → one `maintainConnection(addr)` goroutine per `upstream_clusters` → `spots chan (cap 100)`.
2. `main` glue #1: `upstreamSpots` → `ShouldIgnore(filter)` → `metrics.SpotsReceived++` → `server.HandleInboundSpot(s)` (pushes to `spotChan cap 100`).
3. `telnet.Server.broadcastLoop()`: `IsDuplicate()` drop → `cache.Add()` → `fmt.Fprintln(client.conn, sp.String())` to logged-in clients with `spotsOn`.
4. `main` glue #2: downstream `DX freq call [remarks]` → `outboundSpots chan (cap 10)` → `metrics.SpotsSent++` → `HandleInboundSpot` (local echo) + `upstream.BroadcastSpot(s)` (`dx <freq> <call> <remarks>` to all live upstreams).
5. `Deduplicator.Cleanup()` runs on ticker `duplicate_window_seconds * 2`.

## Build / run / test / lint

```bash
go build -o spotmux ./cmd/spotmux/main.go
./spotmux -config config.json

go test ./...
go vet ./...
gofmt -l .
docker build -t spotmux .
docker run -d -p 7300:7300 -v $(pwd)/config.json:/home/spotmux/config.json --name spotmux spotmux
```

- No Makefile, no CI, no `go.sum` (stdlib only).
- Only test file is `internal/spot/spot_test.go`; run `go test ./internal/spot/ -v` for focused work.
- `go` is not on PATH in some dev containers — use Docker build or install Go 1.25+.

## Config (`internal/config`, `config.json`)

| Key | Default | Notes |
|---|---|---|
| `service_callsign` | — (required) | Sent as upstream login, advertised in telnet banner. Uppercased in telnet server. |
| `operator_name/qth/location` | `""` → skipped | Sent as `set/name`, `set/qth`, `set/qra\|location` after login. 4–6 char `AA00[aa]` → `set/qra`, else `set/location`. |
| `listen_port` | — | Telnet `net.Listen("tcp", :port)`. Example `7300`. |
| `max_downstream_connections` | `10` | Checked in `acceptLoop`; excess gets `Server full` + close. |
| `max_spots_per_minute` | `5` | Token bucket capacity + refill rate. |
| `spot_cache_size` | `200` | `Cache` capacity; replayed to client right after login. |
| `duplicate_window_seconds` | `5` | Dedup window; cleanup ticker = `2×` window. |
| `upstream_clusters` | — | `host:port` strings, e.g. `dxspider.co.uk:7300`. |
| `ignored_prefixes` | `R,UA,UB,UC,UD,UE,UF,UG,UH,UI,EU,EW,UA2,RA2` | Case-insensitive prefix match on `DXCall`. |

## Conventions and constraints

- **KISS + stdlib only.** Do not add dependencies without explicit approval. Prefer `net`, `bufio`, `sync`, `strings`, `strconv`, `time`.
- **Package boundaries:** `spot` = parsing/value types (`Cache`, `Deduplicator`, `PrefixFilter`); `cluster` = upstream I/O; `telnet` = downstream I/O; `config` = loading/defaults; `metrics` = counters. Keep parsing out of I/O loops.
- **Concurrency:** channel owners are `UpstreamManager.spots`, `Server.spotChan/outboundSpots`. Shared maps (`conns`, `clients`, `lastSeen`, `spots` slice) are mutex-guarded — keep that pattern; return copies from `Cache.GetAll()`.
- **Style:** `gofmt` clean, `log.Printf` for ops messages, `fmt.Fprintln(conn, ...)` for client output. Table-driven tests like `spot_test.go`.
- **Docker:** keep final image non-root (`USER spotmux`), `CGO_ENABLED=0`, `-ldflags="-s -w"`, `EXPOSE 7300`, `ENTRYPOINT ["./spotmux"]`.

## Spot format details

- Wire format: `DX de SPOTTER: <freq> <dxcall> <remarks> <HHMM>Z`, e.g. `DX de K1ABC:  14025.0  KH6J  CW 599  1234Z`.
- `ParseSpot(line, source)`: requires `DX de` prefix and `≥6` whitespace fields; `spotter=parts[2]` trimmed of `:`, `freq=parts[3]`, `dxcall=parts[4]`, `time=last field`, `remarks=` substring between `dxcall` and last `time` occurrence. Returns error `not a spot line` / `invalid spot line format` otherwise — callers treat non-spot lines as control/info messages.
- `Spot.String()`: `fmt.Sprintf("DX de %-10s %8s  %-12s %-25s %s", spotter+":", freq, dxcall, remarks, time)`.
- **Dedup key:** `normalizedFreq|DXCall` where freq is `strconv.ParseFloat` → `%.1f` (so `14000` == `14000.0`); remarks/time/spotter/source are ignored. `DXCall` case is significant — preserve upstream case.
- **Filter:** `PrefixFilter` buckets prefixes by length, uppercases both sides. `Matches("")` = false; nil/empty filter = allow-all. Check `ShouldIgnore` only on the upstream path (main glue #1), not on downstream echo.

## Telnet server behavior (do not break clients)

- Banner: `Welcome to SpotMux DX Aggregator` + `DXSpider V1.55 ... <CALL>` (loggers sniff this), then `login: ` (no newline). After login, server sends `Hello <CALL>...` plus a post-login `Running DXSpider V1.55 build 0.000 (SpotMux) <CALL>` line — required because QLog only sniffs `dxspider` while in its `LOGIN_SENT` state to enable spotting.
- First line after connect = callsign (uppercased); reply `Hello <CALL>...`, replay full `cache.GetAll()`, then prompt `de <SERVICE> > `.
- Commands (case-insensitive first token): `DX <freq> <call> [remarks]` (validates `len≥3`, rate-limits, replies `Spot broadcast requested.`), `HELP`, `#SPOT ON|OFF` (per-client `spotsOn`, default ON), `BYE`/`QUIT`. Unknown → `Unknown command.`
- Only logged-in (`callsign != ""`) + `spotsOn` clients receive broadcasts. Each broadcast is prefixed with `\r\n` so the spot starts on a fresh line even though the `de <SERVICE> > ` prompt has no trailing newline (line-oriented clients like QLog require this).
- Broadcast path does dedup + cache insert centrally, so both upstream and downstream-echoed spots flow through the same `broadcastLoop`.

## Upstream manager behavior

- Handshake reads **byte-by-byte** until `login:`/`callsign:` (some clusters omit newline), then sends `<callsign>\n` + optional `set/name|qth|qra|location`. Buffer is trimmed if `>1024` bytes.
- `connectAndRead` logs non-spot lines as `[addr] <line>`; `already connected|already logged in` → returned as error → `maintainConnection` uses **1-hour backoff**, otherwise exponential `1s→5m` cap. Successful close resets to `1s`. `metrics.UpstreamErrors++` on every failed cycle.
- Live conns tracked in `conns[addr]` under mutex; `BroadcastSpot` formats `dx <freq> <dxcall> <remarks>\n` and best-effort writes to all (logs on failure; dead conns reaped on next read error).

## Known pitfalls — check before changing

- `RateLimiter` is **global to the Server, not per-client** (`telnet/server.go:44`). Do not assume per-user fairness; converting to per-client is a behavior change.
- `metrics.DownstreamConnections` is declared but **never incremented/decremented** — `StartPrinter` always shows `0`. Wire it up if you touch connection counting.
- Channels can block: `m.spots <- s` (unbuffered-overflow beyond 100 blocks the upstream read loop), `spotChan <- sp` / `outboundSpots <- sp` (cap 100/10). Keep handlers non-blocking or document backpressure.
- `Cache.Add` shifts with `append(spots[1:], s)` — fine for N≈200, do not use for large N without a ring buffer.
- `Deduplicator` uses `time.Now()` at insert and `time.Since(last)` at check; wall-clock jumps affect it. `Cleanup` deletes entries older than `2×window`.
- No TLS/auth, no graceful drain (SIGINT/SIGTERM just logs and exits; sockets close on process exit). Keep it that way unless asked.
- `processCommand` indexes `parts[0]` after `strings.Fields` without empty check — empty lines are skipped earlier, but refactor carefully to avoid a panic.
- `BYE` calls `c.conn.Close()` inside the scanner loop without returning — relies on deferred cleanup; keep deferred `delete(clients)+Close()`.
- `config.json` in repo contains a real callsign/QTH (`YL3OI`); do not commit personal credentials. `.gitignore` only covers `*.exe`.
