# Proxy Control Plane

Public control API for starting, rejoining, and observing sessions.

## Overview

The Proxy Control Plane validates incoming requests with the Security Token Service (STS), 
creates sessions in the Session Manager, 
and returns the WebSocket URL for the Proxy Data Plane to connect to.

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

