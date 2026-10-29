# Memory System

Ink's optional memory integration targets **self-hosted Zep Community Edition v1.0.2**. It recalls facts relevant to a conversation and searches facts across the authenticated user's sessions. This is not a Zep Cloud or legacy v0.x adapter. CE is deprecated upstream; this compatibility target does not imply ongoing upstream support.

## Deployment

Use the tagged upstream [CE deployment](https://github.com/getzep/zep/blob/v1.0.2/docker-compose.ce.yaml) separately from Ink. CE requires Zep, PostgreSQL with pgvector, Graphiti, and Neo4j. Ink's PostgreSQL container alone is insufficient.

1. Clone the source at tag v1.0.2 into a separate directory with `git clone --branch v1.0.2 --depth 1 https://github.com/getzep/zep.git zep-ce-v1.0.2`.
2. Review `docker-compose.ce.yaml` and `zep.yaml`. Set a strong `api_secret` in `zep.yaml` and Graphiti's `OPENAI_API_KEY` in the checkout's private `.env`. Replace example database credentials consistently. Never commit secrets.
3. Pin the Zep implementation: replace `image: zepai/zep:latest` in its compose service with `build: { context: ., dockerfile: Dockerfile.ce }` and a local image name such as `ink-zep-ce:1.0.2`. Build from the tagged source instead of relying on mutable latest. The tagged compose specifies Graphiti 0.3 and Neo4j 5.22.0.
4. Remove host port publishing for upstream `db`, `graphiti`, and `neo4j` unless explicitly required. Upstream port 5432 conflicts with Ink's database. For host-run Ink, bind Zep as `127.0.0.1:8000:8000`; container deployments need a private network or secured HTTPS endpoint reachable from Ink.
5. Start the reviewed stack from that checkout with `docker compose -f docker-compose.ce.yaml up -d --build`. Check all four services and logs. Ink does not manage this external stack.
6. Configure Ink's `AI_CONFIG_ENCRYPTION_KEY` and, when intentionally using private/HTTP Zep, `MEMORY_ALLOW_INSECURE_PRIVATE_URL=true`. Restart Ink after environment changes.
7. Enable memory in Ink's administrator settings. Enter the Zep base URL without `/api/v2` and the same `api_secret` as its API key.

These are source-pinned deployment instructions, not a claim that Ink's HTTP fixture tests validated the upstream container stack.

## Request flow

For AI replies carrying a conversation ID, Ink recalls session facts and up to three search results, adds context, generates the reply, and attempts to save the last user message and assistant reply. Memory operations have bounded deadlines and failures do not fail the AI response. Storage occurs within the reply request; it is not a durable background queue.

Remote session IDs derive from the authenticated user ID and local conversation ID. Ink provisions the user before creating a session and verifies ownership when reusing a session. Shared starter conversation IDs do not become shared remote sessions.

CE returns `relevant_facts`, not prose conversation summaries. The internal `GetSessionSummary` interface joins those facts as context. Zep/Graphiti extracts facts asynchronously, so new messages may not immediately produce recall.

## Supported upstream contract

API requests use `Authorization: Api-Key <api_secret>`.

| Operation | Request | Response used |
| --- | --- | --- |
| User lookup/create | `GET /api/v2/users/{user_id}` / `POST /api/v2/users` with `{user_id}` | `user_id` |
| Session lookup/create | `GET /api/v2/sessions/{session_id}` / `POST /api/v2/sessions` with `{session_id,user_id}` | Session and owner IDs |
| Store message | `POST /api/v2/sessions/{session_id}/memory` | Successful 2xx |
| Recall session facts | `GET /api/v2/sessions/{session_id}/memory` | `relevant_facts[].fact` |
| Search user facts | `POST /api/v2/sessions/search?limit=3` with `{text,user_id}` | `results[].fact.fact` |

Message payloads contain `messages` with `role`, `role_type`, and `content`. Duplicate creates can return 400; Ink re-reads and verifies identity instead of accepting arbitrary errors. Redirects and non-2xx responses are rejected.

## Ink settings API

- `GET /api/v1/memory/settings` requires authentication. Non-administrators receive flags with the endpoint redacted.
- `PUT /api/v1/admin/memory/settings` requires an administrator and accepts `enabled`, `zepBaseUrl`, and `zepApiKey`. An empty key retains the stored key.
- Keys are encrypted using Ink's encryption key. An undecryptable key is unusable.
- `available` is a cached `GET /healthz` liveness result. Zep exposes health outside authentication; this does **not** prove credentials, graph extraction, or search work.

## Troubleshooting and verification

If health succeeds but recall is empty, verify CE version, API secret, user/session creation, Graphiti/Neo4j health, and eventual extraction. Inspect `docker compose -f docker-compose.ce.yaml logs` in the external checkout. Incorrect authentication prefix or secret produces 401.

Run `make smoke-memory` for local E2E checks through the real Ink API. Controlled Zep/provider fixtures verify HTTP wiring and failure handling. They do not deploy Zep, exercise Graphiti extraction, or establish real-provider latency/capacity. Deployment acceptance requires a real CE v1.0.2 write/extract/recall round trip.

Do not reset Ink's PostgreSQL volume to clear memory. Zep has separate PostgreSQL and Neo4j state; use that deployment's supported deletion procedures and backups. Disabling memory does not delete retained data. Deleting an Ink account currently does not automatically delete its Zep data.

## Data handling

Memory records reside in the self-hosted stack, but Graphiti's model/embedding provider receives data for extraction and Ink's AI provider receives recalled context. Self-hosting the database does not mean conversation data never leaves the host.

## Primary references

- [Routes](https://github.com/getzep/zep/blob/v1.0.2/src/api/routes.go)
- [Search handler](https://github.com/getzep/zep/blob/v1.0.2/src/api/apihandlers/session_handlers_common.go)
- [Memory response](https://github.com/getzep/zep/blob/v1.0.2/src/api/apidata/memory_common.go)
- [Fact response](https://github.com/getzep/zep/blob/v1.0.2/src/api/apidata/fact.go)
- [Session creation](https://github.com/getzep/zep/blob/v1.0.2/src/store/sessionstore_common.go)
- [API authentication](https://github.com/getzep/zep/blob/v1.0.2/src/api/middleware/secret_key_auth_ce.go)
- [Deployment](https://github.com/getzep/zep/blob/v1.0.2/docker-compose.ce.yaml)
