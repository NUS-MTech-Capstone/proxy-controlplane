# Proxy Control Plane

Public control API for starting, rejoining, and observing sessions.

## Overview

The Proxy Control Plane validates incoming requests with the Security Token Service (STS), 
creates sessions in the Session Manager, 
and returns the WebSocket URL for the Proxy Data Plane to connect to.

Session endpoints expect AWS ALB Cognito authentication. The ALB validates the
token and forwards the claims JWT in `x-amzn-oidc-data`. The control plane parses
the `sub` claim as `userId`; clients must not send `userId` in the request body.
`GET /healthz` remains unauthenticated.

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
POST /automation/runs/start
POST /automation/runs/{runId}/token
POST /automation/runs/{runId}/close
```

The automation endpoints are internal workload APIs. Start and token requests
forward a projected Kubernetes service-account bearer token to STS. They do not
derive or accept a human `userId`, and they never carry terminal traffic.

Start request body:

```json
{
  "hostId": "host-123",
  "crId": "cr-123",
  "connectionType": "ssh"
}
```

`connectionType` defaults to `ssh`. `agent` is reserved but rejected until the
streaming DDM Agent transport is implemented.

Rejoin and observe requests use an empty JSON object:

```json
{}
```

Rejoin is limited to the original session owner. Observe returns an observer
token; the signed token, rather than a URL parameter, makes the dataplane
connection read-only.
