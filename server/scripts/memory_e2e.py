#!/usr/bin/env python3
"""Real API regression checks using disposable PostgreSQL and synthetic HTTP peers.

Failure scenarios recorded before implementation:
- Unauthenticated or invalid-token memory reads/writes bypass authorization.
- A regular user can modify settings or learn the administrator's private URL.
- Admin settings fail to persist or expose the configured API key in JSON.
- The HTTP reply handler drops conversationId before invoking memory.
- Session creation, user association, message append, or retrieval never occurs.
- Identical client conversation IDs merge two authenticated users' memories.
- Repeated replies fail when the synthetic peer returns session conflict (409).
- Session summary/search content fails to reach the real HTTP AI request.
- Missing conversation IDs or disabled memory still call the memory peer.
- Memory health, retrieval, session creation, or append outages break AI replies.
- Restart loses configured memory or key retention silently removes credentials.
- Failed checks leave an API process/container behind or publish credentials.
- Redirects forward credentials or memory requests to an unintended endpoint.
- Slow sequential memory requests exceed the per-phase shared time budget.
- Zep CE requires user creation, role_type, relevant_facts, or fact-search
  shapes that an overly permissive fixture fails to enforce.
- A concurrent session-creation conflict belongs to another user and receives
  messages without an ownership recheck.

The fixture models the audited Zep CE v1.0.2 HTTP contract; it does not
establish compatibility by running an actual Zep deployment.
"""

import argparse
import base64
import datetime
import hashlib
import http.server
import json
import os
from pathlib import Path
import secrets
import signal
import sys
import tempfile
import threading
import time
import urllib.error
import urllib.parse
import urllib.request

# Keep repeated runs from creating untracked bytecode in the checkout.
sys.dont_write_bytecode = True
from print_performance import API, Database, command


class Fixture(http.server.ThreadingHTTPServer):
    """Synthetic contract peer, deliberately strict about ownership and ordering."""

    daemon_threads = True

    def __init__(self):
        super().__init__(("127.0.0.1", 0), FixtureHandler)
        self.url = f"http://127.0.0.1:{self.server_port}"
        self.sessions = {}
        self.users = set()
        self.events = []
        self.completions = []
        self.lock = threading.Lock()
        self.failure = None
        self.delay = 0
        self.redirect_operation = None
        self.session_create_conflict = True
        self.conflict_owner = None
        self.memory_key = secrets.token_urlsafe(24)
        self.ai_key = secrets.token_urlsafe(24)

    def __enter__(self):
        self.thread = threading.Thread(target=self.serve_forever, daemon=True)
        self.thread.start()
        return self

    def __exit__(self, *_args):
        self.shutdown()
        self.server_close()
        self.thread.join(timeout=5)


class FixtureHandler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_args):
        pass

    def respond(self, status, body):
        encoded = json.dumps(body).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        try:
            self.wfile.write(encoded)
        except (BrokenPipeError, ConnectionResetError):
            pass  # Expected when the API cancels an intentionally slow peer.

    def do_GET(self):
        self.handle_request()

    def do_POST(self):
        self.handle_request()

    def handle_request(self):
        raw = self.rfile.read(int(self.headers.get("Content-Length", "0")))
        body = json.loads(raw) if raw else {}
        parts = urllib.parse.urlsplit(self.path).path.strip("/").split("/")
        fixture = self.server
        if fixture.delay and not self.path.startswith("/v1/"):
            with fixture.lock:
                fixture.events.append({"operation": "delayed", "method": self.command,
                                       "path": self.path, "delay_seconds": fixture.delay})
            time.sleep(fixture.delay)
        with fixture.lock:
            if self.path == "/v1/chat/completions" and self.command == "POST":
                if self.headers.get("Authorization") != "Bearer " + fixture.ai_key:
                    return self.respond(401, {"error": "invalid synthetic AI credential"})
                fixture.completions.append(body)
                return self.respond(200, {"model": "ink-memory-e2e", "choices": [
                    {"message": {"role": "assistant", "content": "Synthetic AI reply"}}]})

            operation = "unknown"
            if self.path == "/healthz":
                operation = "health"
            elif self.path == "/api/v2/users" and self.command == "POST":
                operation = "user_create"
            elif len(parts) == 4 and parts[:3] == ["api", "v2", "users"] and self.command == "GET":
                operation = "user_get"
            elif self.path == "/api/v2/sessions" and self.command == "POST":
                operation = "session"
            elif parts == ["api", "v2", "sessions", "search"] and self.command == "POST":
                operation = "search"
            elif len(parts) == 4 and parts[:3] == ["api", "v2", "sessions"] and self.command == "GET":
                operation = "session_get"
            elif len(parts) == 5 and parts[:3] == ["api", "v2", "sessions"] and parts[4] == "memory":
                operation = "summary" if self.command == "GET" else "append"
            event = {"operation": operation, "method": self.command, "path": self.path}
            fixture.events.append(event)
            if fixture.redirect_operation == operation:
                event["status"] = 307
                self.send_response(307)
                self.send_header("Location", fixture.url + "/redirect-target")
                self.send_header("Content-Length", "0")
                self.end_headers()
                return
            if operation != "health" and self.headers.get("Authorization") != "Api-Key " + fixture.memory_key:
                event["status"] = 401
                return self.respond(401, {"error": "invalid synthetic memory credential"})
            if fixture.failure == operation:
                event["status"] = 503
                return self.respond(503, {"error": "synthetic outage"})
            event["status"] = 200
            if operation == "health":
                return self.respond(200, {"status": "ok"})
            if operation == "user_get":
                user_id = urllib.parse.unquote(parts[3])
                event["user_id"] = user_id
                if user_id not in fixture.users:
                    event["status"] = 404
                    return self.respond(404, {"error": "user missing"})
                return self.respond(200, {"user_id": user_id})
            if operation == "user_create":
                user_id = body.get("user_id")
                event["user_id"] = user_id
                if not user_id:
                    event["status"] = 400
                    return self.respond(400, {"error": "missing user identity"})
                if user_id in fixture.users:
                    event["status"] = 409
                    return self.respond(409, {"error": "user already exists"})
                fixture.users.add(user_id)
                event["status"] = 201
                return self.respond(201, {"user_id": user_id})
            if operation == "session_get":
                session_id = urllib.parse.unquote(parts[3])
                event["session_id"] = session_id
                session = fixture.sessions.get(session_id)
                if session is None:
                    event["status"] = 404
                    return self.respond(404, {"error": "session missing"})
                return self.respond(200, {"session_id": session_id, "user_id": session["user_id"]})
            if operation == "session":
                session_id, user_id = body.get("session_id"), body.get("user_id")
                event.update(session_id=session_id, user_id=user_id)
                if not session_id or not user_id or user_id not in fixture.users:
                    event["status"] = 400
                    return self.respond(400, {"error": "missing identity"})
                if session_id in fixture.sessions:
                    event["status"] = 409
                    return self.respond(409, {"error": "already exists"})
                fixture.sessions[session_id] = {"user_id": fixture.conflict_owner or user_id, "messages": []}
                if fixture.session_create_conflict:
                    # Simulate another creator winning after GET returned 404.
                    event["status"] = 409
                    return self.respond(409, {"error": "concurrent session creation"})
                event["status"] = 201
                return self.respond(201, {"session_id": session_id})
            if operation in ("summary", "append"):
                session_id = urllib.parse.unquote(parts[3])
                event["session_id"] = session_id
                session = fixture.sessions.get(session_id)
                if session is None:
                    event["status"] = 404
                    return self.respond(404, {"error": "session missing"})
                if operation == "summary":
                    return self.respond(200, {"relevant_facts": [{"fact": "summary-for-" + session["user_id"],
                                                                  "created_at": "2026-01-01T00:00:00Z"}]})
                messages = body.get("messages", [])
                if not messages or any(m.get("role_type") not in ("user", "assistant", "system") for m in messages):
                    event["status"] = 400
                    return self.respond(400, {"error": "valid role_type required"})
                session["messages"].extend({"role": m["role_type"], "content": m["content"]} for m in messages)
                event["roles"] = [message["role_type"] for message in messages]
                event["status"] = 201
                return self.respond(201, {})
            if operation == "search":
                user_id = body.get("user_id")
                query = urllib.parse.parse_qs(urllib.parse.urlsplit(self.path).query)
                event.update(user_id=user_id, query=body.get("text"), limit=int(query.get("limit", ["0"])[0]))
                if not user_id or not body.get("text") or "limit" in body:
                    event["status"] = 400
                    return self.respond(400, {"error": "flat text/user_id and query limit required"})
                has_memory = any(s["user_id"] == user_id and s["messages"] for s in fixture.sessions.values())
                results = [{"fact": {"fact": "memory-for-" + user_id, "created_at": "2026-01-01T00:00:00Z"}}] if has_memory else []
                return self.respond(200, {"results": results})
            event["status"] = 404
            return self.respond(404, {"error": "unexpected fixture route"})


def request(api, method, path, token=None, body=None, expected=200):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    req = urllib.request.Request(api.url + path, method=method, headers=headers,
                                 data=None if body is None else json.dumps(body).encode())
    try:
        with urllib.request.urlopen(req, timeout=30) as response:
            status, raw = response.status, response.read()
    except urllib.error.HTTPError as error:
        status, raw = error.code, error.read()
    if status != expected:
        # Do not include payloads, headers, or credentials in errors/evidence.
        raise AssertionError(f"{method} {path}: expected HTTP {expected}, received {status}")
    return json.loads(raw) if raw else None


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def remote_session_id(user_id, conversation_id):
    pair = json.dumps([user_id, conversation_id], separators=(",", ":"), ensure_ascii=False)
    return "ink-" + hashlib.sha256(pair.encode()).hexdigest()


def verify(api, fixture, credentials, evidence):
    checks = evidence["checks"]
    get_path = "/api/v1/memory/settings"
    put_path = "/api/v1/admin/memory/settings"
    settings = {"enabled": True, "zepBaseUrl": fixture.url, "zepApiKey": fixture.memory_key}
    for token in (None, "invalid-token"):
        request(api, "GET", get_path, token, expected=401)
        request(api, "PUT", put_path, token, settings, expected=401)
    checks.append("unauthenticated_and_invalid_token_memory_endpoints_401")
    login = request(api, "POST", "/api/v1/auth/login", body={
        "email": credentials["login"], "password": credentials["password"]})
    admin = login["accessToken"]
    require(login["user"]["role"] == "admin", "Seeded account must be admin")
    initial = request(api, "GET", get_path, admin)
    require(not initial["enabled"], "Disposable database must start with memory disabled")
    saved = request(api, "PUT", put_path, admin, settings)
    require(saved["enabled"] and saved["available"] and saved["keyConfigured"], "Memory settings not active")
    require(saved["zepBaseUrl"] == fixture.url, "Admin settings omit configured URL")
    require(fixture.memory_key not in json.dumps(saved), "Memory response exposes configured key")
    persisted = request(api, "GET", get_path, admin)
    require(all(saved[key] == persisted[key] for key in ("enabled", "available", "keyConfigured", "zepBaseUrl")),
            "Memory settings did not persist")
    checks.append("admin_configure_and_read_persisted_memory_settings_without_key_disclosure")
    users = []
    for number in (1, 2):
        account = {"email": f"memory-{number}@example.invalid", "name": f"Memory user {number}",
                   "password": secrets.token_urlsafe(24)}
        created = request(api, "POST", "/api/v1/admin/users", admin, account, expected=201)
        logged = request(api, "POST", "/api/v1/auth/login", body={
            "email": account["email"], "password": account["password"]})
        require(created["user"]["id"] == logged["user"]["id"], "Created and authenticated user differ")
        users.append((logged["user"]["id"], logged["accessToken"]))
        member = request(api, "GET", get_path, logged["accessToken"])
        require(member["enabled"] and member["zepBaseUrl"] == "", "Regular user learns private URL or loses enabled flag")
        request(api, "PUT", put_path, logged["accessToken"], settings, expected=403)
    checks.append("regular_user_read_redacts_private_url_and_write_returns_403")
    request(api, "PUT", "/api/v1/admin/ai/config", admin, {
        "providerName": "Synthetic HTTP fixture", "providerType": "openai-compatible",
        "baseUrl": fixture.url + "/v1", "model": "ink-memory-e2e", "apiKey": fixture.ai_key})

    def reply(token, conversation_id, content):
        payload = {"messages": [{"role": "user", "content": content}]}
        if conversation_id is not None:
            payload["conversationId"] = conversation_id
        before = len(fixture.completions)
        result = request(api, "POST", "/api/v1/ai/reply", token, payload)
        require(result["content"] == "Synthetic AI reply", "AI reply changed or disappeared")
        require(len(fixture.completions) == before + 1, "Reply did not reach HTTP AI peer exactly once")
        return fixture.completions[-1]["messages"]

    conversation = "shared-client-conversation"
    for user_id, token in users:
        marker = "private-message-for-" + user_id
        first = reply(token, conversation, marker)
        require(first == [{"role": "user", "content": marker}], "New user received another user's memory")
        session_id = remote_session_id(user_id, conversation)
        session = fixture.sessions.get(session_id)
        require(session is not None and session["user_id"] == user_id, "HTTP conversationId was dropped or session ownership differs")
        require(session["messages"] == [{"role": "user", "content": marker},
                                         {"role": "assistant", "content": "Synthetic AI reply"}],
                "Memory did not append both user message and assistant reply")
        second = reply(token, conversation, "recall-my-memory")
        require(second[0]["role"] == "system", "Retrieved memory is not a system message")
        require("summary-for-" + user_id in second[0]["content"], "Session summary not forwarded to HTTP AI")
        require("memory-for-" + user_id in second[0]["content"], "User memory search not forwarded to HTTP AI")
        for other_id, _ in users:
            if other_id != user_id:
                require(other_id not in json.dumps(second), "Cross-user memory content leaked to AI")
        require(len(session["messages"]) == 4, "Session 409 prevented subsequent message appends")
    require(len(fixture.sessions) == 2, "Identical client conversation IDs must produce two remote sessions")
    require(fixture.users == {uid for uid, _ in users}, "Session storage did not create its Zep user first")
    require(sum(e["operation"] == "session" and e.get("status") == 409 for e in fixture.events) == 2,
            "Concurrent session creation conflict was not exercised for both users")
    for index, event in enumerate(fixture.events):
        if event["operation"] == "session" and event.get("status") == 409:
            following = [e for e in fixture.events[index + 1:] if e.get("session_id") == event["session_id"]]
            require(following and following[0]["operation"] == "session_get",
                    "Session conflict was accepted without checking remote ownership")
    require(all(event.get("limit") == 3 for event in fixture.events if event["operation"] == "search"),
            "Memory search limit differs from expected contract")
    checks.extend(["http_conversation_id_reaches_user_scoped_session_creation",
                   "user_and_assistant_messages_append_after_session_creation",
                   "session_summary_and_user_search_reach_http_ai_request",
                   "same_client_conversation_id_isolated_between_authenticated_users",
                   "user_creation_and_session_409_identity_recheck_preserve_appends"])
    fixture.conflict_owner = users[1][0]
    reply(users[0][1], "conflict-owned-by-other-user", "must-not-store")
    foreign_session = fixture.sessions[remote_session_id(users[0][0], "conflict-owned-by-other-user")]
    require(not foreign_session["messages"], "Session creation conflict appended to another user's session")
    fixture.conflict_owner = None
    checks.append("session_creation_conflict_rejects_other_user_ownership")

    before = len(fixture.events)
    reply(users[0][1], None, "no-conversation")
    require(len(fixture.events) == before, "Reply without conversationId contacted memory")
    checks.append("missing_conversation_id_skips_memory")
    outages = []
    for failure in ("health", "summary", "search", "session_get", "append"):
        fixture.failure = failure
        before = len(fixture.events)
        started = time.monotonic()
        reply(users[0][1], conversation, "outage-" + failure)
        elapsed = round(time.monotonic() - started, 3)
        require(any(e["operation"] == failure and e["status"] == 503 for e in fixture.events[before:]),
                "Requested memory outage was not exercised: " + failure)
        outages.append({"operation": failure, "reply_status": 200, "elapsed_seconds": elapsed})
    fixture.failure = None
    evidence["outages"] = outages
    checks.append("health_retrieval_session_and_append_outages_preserve_ai_reply")
    retained = dict(settings, zepApiKey="")
    request(api, "PUT", put_path, admin, retained)
    reply(users[0][1], conversation, "retained-key")
    require(not any(e["status"] == 401 for e in fixture.events), "Empty key update lost configured credential")
    checks.append("empty_key_update_retains_working_memory_credential")
    for operation in ("health", "summary", "search", "append"):
        fixture.redirect_operation = operation
        before = len(fixture.events)
        reply(users[0][1], conversation, "redirect-" + operation)
        require(any(e.get("status") == 307 and e["operation"] == operation for e in fixture.events[before:]),
                "Redirect scenario did not exercise expected operation: " + operation)
        require(not any(e["path"] == "/redirect-target" for e in fixture.events[before:]),
                "Memory client followed redirect to an unintended endpoint")
    fixture.redirect_operation = None
    checks.append("memory_health_and_authenticated_requests_reject_redirects")
    fixture.delay = 1.2
    before = len(fixture.events)
    started = time.monotonic()
    reply(users[0][1], conversation, "shared-budget")
    elapsed = time.monotonic() - started
    fixture.delay = 0
    require(3.5 <= elapsed < 5.5, "Slow memory phases did not respect two shared two-second budgets")
    require(sum(e["operation"] == "delayed" for e in fixture.events[before:]) >= 4,
            "Shared-budget scenario did not delay multiple operations in both phases")
    evidence["timeout_budget"] = {"phase_budget_seconds": 2, "per_http_delay_seconds": 1.2,
                                  "reply_elapsed_seconds": round(elapsed, 3), "reply_status": 200}
    # Let canceled fixture handlers finish before subsequent event-count checks.
    time.sleep(1.3)
    checks.append("slow_memory_requests_share_two_second_enhance_and_store_budgets")
    request(api, "PUT", put_path, admin, dict(retained, enabled=False))
    before = len(fixture.events)
    reply(users[0][1], conversation, "disabled-memory")
    require(len(fixture.events) == before, "Disabled memory contacted remote peer")
    checks.append("disabled_memory_skips_remote_requests")
    request(api, "PUT", put_path, admin, retained)
    evidence["isolation"] = [{"user_id": uid, "remote_session_id": remote_session_id(uid, conversation)} for uid, _ in users]
    return admin, users[0], conversation


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    root = Path(__file__).resolve().parents[2]
    parser.add_argument("--output", type=Path, default=root / "output/pr131/memory-e2e.json")
    args = parser.parse_args()
    server = root / "server"
    evidence = {"schema_version": 1, "success": False, "checks": [],
                "started_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                "scope": "Real Ink API and disposable PostgreSQL; synthetic local HTTP Zep CE v1.0.2 and AI contracts only. No actual Zep deployment compatibility claim.",
                "repeat": "sh server/scripts/smoke_memory.sh"}
    # SIGTERM must unwind context managers just as Ctrl-C does.
    def terminate(_signum, _frame):
        raise KeyboardInterrupt("Memory E2E interrupted")
    signal.signal(signal.SIGTERM, terminate)
    try:
        command(["docker", "info"], data=None)
        with tempfile.TemporaryDirectory(prefix="ink-memory-e2e-") as temp:
            scratch = Path(temp)
            for entry in ("api", "migrate", "seed"):
                command(["go", "build", "-o", str(scratch / entry), "./cmd/" + entry], cwd=server)
            (scratch / "migrations").symlink_to(server / "migrations", target_is_directory=True)
            with Database() as db, Fixture() as fixture:
                credentials_path = scratch / "admin-credentials"
                # Run binaries outside the checkout so local .env files cannot
                # activate real providers, metrics listeners, or background jobs.
                env = {key: os.environ[key] for key in ("PATH", "HOME", "TMPDIR") if key in os.environ}
                env.update(DATABASE_URL=db.url, JWT_SECRET=secrets.token_hex(32),
                           AI_CONFIG_ENCRYPTION_KEY=base64.b64encode(secrets.token_bytes(32)).decode(),
                           INK_DEV_ADMIN_CREDENTIALS_PATH=str(credentials_path),
                           WORKERS_ENABLED="false", PRINT_STATUS_SYNC_ENABLED="false",
                           AI_ALLOW_INSECURE_PRIVATE_URL="true", MEMORY_ALLOW_INSECURE_PRIVATE_URL="true")
                command([str(scratch / "migrate"), "up"], cwd=scratch, env=env)
                command([str(scratch / "seed"), "dev"], cwd=scratch, env=env)
                credentials = dict(line.split("=", 1) for line in credentials_path.read_text().splitlines() if "=" in line)
                with API(scratch / "api", scratch, env, scratch) as api:
                    admin, (user_id, token), conversation = verify(api, fixture, credentials, evidence)
                require(api.process.returncode == 0, "First API process did not shut down cleanly")
                with API(scratch / "api", scratch, env, scratch) as api:
                    settings = request(api, "GET", "/api/v1/memory/settings", admin)
                    require(settings["enabled"] and settings["keyConfigured"] and settings["available"],
                            "Restart did not restore persisted memory configuration")
                    before = len(fixture.completions)
                    result = request(api, "POST", "/api/v1/ai/reply", token, {"conversationId": conversation,
                        "messages": [{"role": "user", "content": "after-restart"}]})
                    require(result["content"] == "Synthetic AI reply" and len(fixture.completions) == before + 1,
                            "Restarted API failed to use persisted AI config")
                    require("memory-for-" + user_id in fixture.completions[-1]["messages"][0]["content"],
                            "Restarted API did not restore active memory client")
                require(api.process.returncode == 0, "Restarted API did not shut down cleanly")
                evidence["checks"].append("restart_restores_persisted_memory_and_ai_configuration")
                evidence["memory_http_events"] = fixture.events
                evidence["ai_http_requests"] = len(fixture.completions)
            require(not command(["docker", "ps", "--all", "--quiet", "--filter", "name=^/" + db.name + "$"]),
                    "Disposable PostgreSQL container was not removed")
            evidence["checks"].append("api_and_disposable_database_cleaned_up")
        evidence["success"] = True
    except (Exception, KeyboardInterrupt) as error:
        # Runtime logs may contain environmental values. Keep evidence to
        # assertion descriptions; command failures expose only their type.
        evidence["error"] = str(error) if isinstance(error, AssertionError) else type(error).__name__
    finally:
        evidence["finished_at"] = datetime.datetime.now(datetime.timezone.utc).isoformat()
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(json.dumps(evidence, indent=2) + "\n")
    print(json.dumps({"success": evidence["success"], "checks": len(evidence["checks"]),
                      "evidence": str(args.output), "error": evidence.get("error")}))
    return 0 if evidence["success"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
