# Proxy Control Plane

Public control API for starting, rejoining, and observing sessions.

## Overview

The Proxy Control Plane validates incoming requests with the Security Token Service (STS), 
creates sessions in the Session Manager, 
and returns the WebSocket URL for the Proxy Data Plane to connect to.

Session endpoints expect AWS ALB Cognito authentication. The control plane
cryptographically verifies the ES256 `x-amzn-oidc-data` JWT against the AWS
regional ALB public key endpoint. It rejects an unexpected signer ALB ARN,
issuer, client ID, algorithm, key ID, signature, expiry, or a mismatch between
the signed `sub` and `x-amzn-oidc-identity`.

The verified `sub` becomes `userId`; clients must not send `userId` in the
request body. `GET /healthz` remains unauthenticated.

## Run

```bash
PORT=8080 \
STS_URL=http://localhost:8082 \
SESSION_MANAGER_URL=http://localhost:8081 \
AWS_REGION=ap-southeast-1 \
ALB_ARN=arn:aws:elasticloadbalancing:ap-southeast-1:123456789012:loadbalancer/app/example/id \
COGNITO_USER_POOL_ID=ap-southeast-1_XXXXXXXXX \
COGNITO_CLIENT_ID=xxxxxxxxxxxxxxxxxxxxxxxxxx \
go run ./cmd/server
```

`COGNITO_ISSUER` may be set directly. Otherwise it is derived from
`AWS_REGION` and `COGNITO_USER_POOL_ID`. Production startup fails when verifier
configuration is incomplete.

## Endpoints

```http
GET  /healthz
POST /sessions/start
GET  /sessions?status=ACTIVE&mine=false
GET  /sessions/rejoin-candidates?hostId={hostId}&crId={crId}
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

Rejoin is limited to the original session owner. Observe is available to any
authenticated user and returns an observer token; the signed token, rather than
a URL parameter, makes the dataplane connection read-only.

To discover sessions eligible for owner rejoin, call
`GET /sessions/rejoin-candidates?hostId={hostId}&crId={crId}`. The control plane
derives `userId` from verified Cognito identity, validates rejoin access with
STS, and returns active sessions owned by that user for the requested server and
change request. The response intentionally excludes session tokens, WebSocket
URLs, and dataplane ownership fields. Use the selected `sessionId` with
`POST /sessions/{sessionId}/rejoin` to receive a fresh scoped session token.

For webapp session tables, call `GET /sessions`. Optional filters are `hostId`,
`crId`, `status`, and `mine=true`. The response contains safe session metadata
only and never includes session tokens, WebSocket URLs, or dataplane ownership
fields. Token issuance still happens only through start, rejoin, and observe.

## API Specification

The OpenAPI contract is checked in at `api/openapi.yaml` and is served
unauthenticated at:

```http
GET /openapi.yaml
```
