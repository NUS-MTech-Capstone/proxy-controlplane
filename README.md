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

## Design Patterns

The service layer acts as a Facade over STS and Session Manager. Public handlers
call one control-plane operation, while the service coordinates authorization,
session assignment or lookup, and scoped token issuance without exposing those
internal calls to clients.

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

Optional lifecycle event publication:

```bash
EVENT_PUBLISHER_ENABLED=true
EVENTS_TOPIC_ARN=arn:aws:sns:ap-southeast-1:123456789012:dendenmushi-events-dev
AWS_REGION=ap-southeast-1
```

When enabled, the control plane publishes sanitized `SessionStarted`,
`UserRejoined`, `ObserverJoined`, and user-requested `SessionClosed` metadata
events to SNS after the synchronous control operation succeeds. Publish
failures are logged and do not fail the session path. Event payloads never
include session tokens, WebSocket URLs, terminal data, command bodies, or
secrets.

## Endpoints

```http
GET  /healthz
POST /sessions/start
GET  /sessions?status=ACTIVE&mine=false
GET  /sessions/rejoin-candidates?hostId={hostId}&crId={crId}
POST /sessions/{sessionId}/rejoin
POST /sessions/{sessionId}/observe
POST /sessions/{sessionId}/close
```

Bounded Agentic AI and automation run endpoints are owned by
`agent-controlplane`. Proxy Control Plane remains focused on authenticated
session start, rejoin, observe, list, and close flows. It never carries
terminal traffic.

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

Close requests also use an empty JSON object. The session owner may close their
own session. Users in the Cognito `SecurityAnalyst` or `Admin` groups may close
another user's active session.

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
