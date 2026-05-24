# proxy-controlplane
# Proxy Control Plane

Public control API for starting, rejoining, and observing MVP sessions.

## Run

```bash
PORT=8080 \
STS_URL=http://localhost:8082 \
SESSION_MANAGER_URL=http://localhost:8081 \
go run ./cmd/server
```

## Endpoints

```http
GET  /healthz
POST /sessions/start
POST /sessions/{sessionId}/rejoin
POST /sessions/{sessionId}/observe
```

`/sessions/start` validates with STS, creates a session in Session Manager, asks
STS for a session token, then returns the direct Proxy Dataplane WebSocket URL.
