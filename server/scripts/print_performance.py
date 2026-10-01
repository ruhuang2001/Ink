#!/usr/bin/env python3
"""Measure real Ink API routes against disposable PostgreSQL and a provider stub."""

import argparse
import base64
import hashlib
import hmac
import http.server
import json
import math
import os
import pathlib
import platform
import shutil
import socket
import subprocess
import tempfile
import threading
import time
import urllib.parse
import urllib.request
import uuid


def command(args, *, cwd=None, env=None, data=None):
    result = subprocess.run(args, cwd=cwd, env=env, input=data, text=True,
                            capture_output=True, check=True)
    return result.stdout.strip()


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def percentile(values, fraction):
    return sorted(values)[max(0, math.ceil(len(values) * fraction) - 1)]


def source_fingerprint(server_dir):
    digest = hashlib.sha256()
    for path in sorted(server_dir.rglob("*")):
        if path.is_file() and (path.suffix in (".go", ".sql", ".otf") or path.name in ("go.mod", "go.sum")):
            digest.update(path.relative_to(server_dir).as_posix().encode() + b"\0" + path.read_bytes())
    return digest.hexdigest()


class Provider(http.server.ThreadingHTTPServer):
    def __init__(self, delay):
        super().__init__(("127.0.0.1", 0), ProviderHandler)
        self.delay = delay
        self.printed = False
        self.calls = 0
        self.lock = threading.Lock()
        self.records = []


class ProviderHandler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_args):
        pass

    def do_POST(self):
        self.rfile.read(int(self.headers.get("Content-Length", "0")))
        if self.path != "/home/getprintstatus":
            self.send_error(400, "Only status requests are expected")
            return
        with self.server.lock:
            self.server.calls += 1
            printed = self.server.printed
            self.server.records.append(time.monotonic())
        time.sleep(self.server.delay)
        body = json.dumps({"showapi_res_code": 1, "showapi_res_error": "ok",
                           "printflag": int(printed), "printcontentid": 1}).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def token(secret):
    def encoded(value):
        return base64.urlsafe_b64encode(json.dumps(value).encode()).rstrip(b"=")
    now = int(time.time())
    payload = encoded({"typ": "access", "sid": "perf-session", "sub": "perf-user",
                       "iss": "ink-print-perf", "iat": now, "exp": now + 3600})
    raw = encoded({"alg": "HS256", "typ": "JWT"}) + b"." + payload
    signature = base64.urlsafe_b64encode(hmac.new(secret.encode(), raw,
                                                  hashlib.sha256).digest()).rstrip(b"=")
    return (raw + b"." + signature).decode()


def request(base_url, path, access_token, body=None):
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(base_url + path, data=data,
                                 headers={"Authorization": "Bearer " + access_token,
                                          "Content-Type": "application/json"})
    start = time.perf_counter()
    with urllib.request.urlopen(req, timeout=120) as response:
        raw = response.read()
    return (time.perf_counter() - start) * 1000, raw


class Database:
    def __init__(self):
        self.name = "ink-print-perf-" + uuid.uuid4().hex[:10]
        self.port = free_port()
        self.url = f"postgres://postgres:performance@127.0.0.1:{self.port}/ink?sslmode=disable"

    def __enter__(self):
        command(["docker", "run", "--detach", "--rm", "--name", self.name,
                 "-e", "POSTGRES_PASSWORD=performance", "-e", "POSTGRES_DB=ink",
                 "-p", f"127.0.0.1:{self.port}:5432", "postgres:16"])
        try:
            for _ in range(100):
                ready = subprocess.run(["docker", "exec", self.name, "pg_isready", "-h", "127.0.0.1",
                                        "-U", "postgres", "-d", "ink"],
                                       capture_output=True).returncode == 0
                if ready:
                    return self
                time.sleep(0.1)
            raise RuntimeError("Disposable PostgreSQL failed to become ready")
        except BaseException:
            self.__exit__(None, None, None)
            raise

    def sql(self, sql):
        return command(["docker", "exec", "-i", self.name, "psql", "-U", "postgres",
                        "-d", "ink", "-v", "ON_ERROR_STOP=1", "-qAt"], data=sql)

    def __exit__(self, *_args):
        subprocess.run(["docker", "rm", "--force", self.name], capture_output=True)


def seed(database, server_dir, total, queued, body_bytes):
    manifest = json.loads((server_dir / "testdata/plugins/python-hello-plugin/ink-plugin.json").read_text())
    manifest_sql = json.dumps(manifest).replace("'", "''")
    database.sql(f"""
insert into users (id,email,password_hash,display_name,status,role) values
 ('perf-user','perf@example.invalid','unused','Performance','active','admin');
insert into auth_sessions (id,family_id,user_id,refresh_token_hash,client_type,expires_at,created_at,last_used_at)
 values ('perf-session','perf-family','perf-user','unused','web',now()+interval '1 hour',now(),now());
insert into printer_bindings (id,user_id,name,device_identifier,provider_user_id,status,created_at,updated_at)
 values ('perf-device','perf-user','Stub','stub-device',1,'connected',now(),now());
insert into print_jobs
 (id,user_id,printer_binding_id,title,source,content,status,provider_print_content_id,created_at,updated_at)
 select 'perf-job-'||lpad(i::text,5,'0'),'perf-user','perf-device','Performance '||i,'Baseline',
 repeat('x',{body_bytes}),case when i<={queued} then 'queued' else 'completed' end,
 case when i<={queued} then i else null end,
 now()-i*interval '1 second',now()-i*interval '1 second'
 from generate_series(1,{total}) i;
insert into workspace_snapshots (user_id,state) select 'perf-user',jsonb_build_object(
 'printJobs',jsonb_agg(jsonb_build_object('id',id,'title',title,'source',source,'content',content,
 'deviceId',printer_binding_id,'status',status,'createdAt',created_at,'updatedAt',updated_at)),
 'devices',jsonb_build_array(jsonb_build_object('id','perf-device','name','Stub','status','connected','note','')),
 'preferences',jsonb_build_object('sendConfirmationEnabled',true)) from print_jobs;
insert into plugin_installations
 (id,plugin_key,source_type,display_name,version,runtime_type,manifest_json,current_path,status,created_at,updated_at)
 values ('perf-plugin','performance-source','upload','Performance','1.0.0','python',
 '{manifest_sql}'::jsonb,'unused','ready',now(),now());
insert into plugin_bindings
 (id,plugin_installation_id,user_id,enabled,status,next_fetch_at,created_at,updated_at)
 values ('perf-binding','perf-plugin','perf-user',true,'connected',now()+interval '1 day',now(),now());
insert into plugin_items
 (id,user_id,plugin_installation_id,plugin_binding_id,external_id,title,source_label,blocks_json,status,fetched_at,created_at,updated_at)
 values ('perf-item','perf-user','perf-plugin','perf-binding','unique','Due task','Performance',
 '[{{"type":"paragraph","text":"Due task fixture"}}]'::jsonb,'pending',now(),now(),now());
insert into print_schedules
 (id,user_id,plugin_installation_id,plugin_binding_id,title,frequency_type,timezone,hour,minute,device_id,
 enabled,next_run_at,created_at,updated_at)
 values ('perf-schedule','perf-user','perf-plugin','perf-binding','Due task','daily','UTC',0,0,
 'perf-device',false,now(),now(),now());
analyze print_jobs;
""")
    has_sync = database.sql("select count(*) from information_schema.columns where table_name='print_jobs' and column_name='next_status_check_at';")
    if has_sync == "1":
        database.sql("update print_jobs set next_status_check_at=now() where status='queued';")


class API:
    def __init__(self, binary, server_dir, env, temp_dir):
        self.port = free_port()
        self.url = f"http://127.0.0.1:{self.port}"
        self.env = dict(env, PORT=str(self.port))
        self.binary = binary
        self.server_dir = server_dir
        self.log_path = temp_dir / f"api-{self.port}.log"

    def __enter__(self):
        self.log = self.log_path.open("w")
        self.process = subprocess.Popen([str(self.binary)], cwd=self.server_dir, env=self.env,
                                        stdout=self.log, stderr=subprocess.STDOUT)
        try:
            for _ in range(100):
                if self.process.poll() is not None:
                    raise RuntimeError(self.log_path.read_text())
                try:
                    with urllib.request.urlopen(self.url + "/healthz", timeout=1):
                        return self
                except OSError:
                    time.sleep(0.05)
            raise RuntimeError("API did not start")
        except BaseException:
            self.__exit__(None, None, None)
            raise

    def __exit__(self, *_args):
        self.process.terminate()
        try:
            self.process.wait(timeout=15)
        except subprocess.TimeoutExpired:
            self.process.kill()
            self.process.wait()
        self.log.close()


def sample_endpoint(api, provider, access_token, path, count, warmup, validate=None):
    for _ in range(warmup):
        request(api.url, path, access_token)
    before = provider.calls
    samples = []
    sizes = []
    rows = []
    for _ in range(count):
        elapsed, raw = request(api.url, path, access_token)
        payload = json.loads(raw)
        if validate:
            validate(payload)
        samples.append(elapsed)
        sizes.append(len(raw))
        rows.append(len(payload.get("printJobs", [])))
    return {"path": path, "samples": count, "warmup": warmup,
            "p50_ms": percentile(samples, .50), "p95_ms": percentile(samples, .95),
            "response_bytes": sizes, "row_counts": rows,
            "provider_calls": provider.calls - before, "latencies_ms": samples}


def rss_bytes(pid):
    return int(command(["ps", "-o", "rss=", "-p", str(pid)])) * 1024


def render_memory(api, access_token, size, runs):
    body = {"title": "Performance preview", "content": "Render memory measurement.\n" * (size // 27)}
    request(api.url, "/api/v1/print-preview", access_token, body)
    time.sleep(.2)
    before = rss_bytes(api.process.pid)
    samples = [before]
    stop = threading.Event()

    def sample():
        while not stop.is_set():
            started = time.monotonic()
            samples.append(rss_bytes(api.process.pid))
            stop.wait(max(0, .02 - (time.monotonic() - started)))

    thread = threading.Thread(target=sample)
    thread.start()
    latencies = []
    hashes = []
    try:
        for _ in range(runs):
            elapsed, raw = request(api.url, "/api/v1/print-preview", access_token, body)
            payload = json.loads(raw)
            png = base64.b64decode(payload["image"], validate=True)
            assert png.startswith(b"\x89PNG\r\n\x1a\n"), "Preview is not a PNG"
            hashes.append(hashlib.sha256(png).hexdigest())
            latencies.append(elapsed)
    finally:
        stop.set()
        thread.join()
    return {"input_bytes": len(body["content"].encode()), "runs": runs, "warmup": 1,
            "png_sha256": hashes,
            "rss_before_bytes": before, "sampled_peak_rss_bytes": max(samples),
            "sampled_peak_delta_bytes": max(samples) - before, "rss_sampling_interval_ms": 20,
            "rss_samples": len(samples), "p95_ms": percentile(latencies, .95)}


def measure(source, label, candidate, args, provider, temp_dir):
    server_dir = temp_dir / label / "server"
    shutil.copytree(source / "server", server_dir,
                    ignore=shutil.ignore_patterns(".env", ".dev-admin-password", ".plugins"))
    binary = temp_dir / f"ink-{label}"
    command(["go", "build", "-o", str(binary), "./cmd/api"], cwd=server_dir)
    with Database() as database:
        secret = "disposable-performance-secret"
        env = dict(os.environ, DATABASE_URL=database.url, JWT_SECRET=secret, APP_NAME="ink-print-perf",
                   MEMOBIRD_ACCESS_KEY="stub-key", MEMOBIRD_BASE_URL=f"http://127.0.0.1:{provider.server_port}",
                   MEMOBIRD_TIMEOUT="5s", PRINT_STATUS_TIMEOUT="5s", PRINT_STATUS_SYNC_ENABLED="false",
                   PRINT_STATUS_POLL_INTERVAL="2s", PRINT_STATUS_BATCH_SIZE="20", SCHEDULER_POLL_INTERVAL="2s",
                   INBOX_JANITOR_INTERVAL="24h", PLUGIN_ROOT=str(temp_dir / "unused-plugins"))
        command(["go", "run", "./cmd/migrate", "up"], cwd=server_dir, env=env)
        seed(database, server_dir, args.jobs, args.queued, args.body_bytes)
        access_token = token(secret)
        provider.printed = False
        result = {"label": label, "server_source_sha256": source_fingerprint(server_dir),
                  "compiled_go": command(["go", "version", str(binary)]).rsplit(": ", 1)[1], "routes": {}}

        def validate_page(payload):
            assert len(payload["printJobs"]) <= 20, "Page exceeded requested limit"
            assert all("content" not in job for job in payload["printJobs"]), "Page returned full content"
            assert bool(payload.get("nextCursor")) == (args.jobs > 20)

        with API(binary, server_dir, env, temp_dir) as api:
            result["routes"]["list"] = sample_endpoint(api, provider, access_token,
                "/api/v1/print-jobs?status=all&limit=20", args.samples, args.warmup,
                validate_page if candidate else None)
            result["routes"]["workspace"] = sample_endpoint(api, provider, access_token,
                "/api/v1/workspace", args.samples, args.warmup)
            if candidate:
                ids = ",".join(f"perf-job-{i:05}" for i in range(1, args.queued + 1))
                result["routes"]["status"] = sample_endpoint(api, provider, access_token,
                    "/api/v1/print-jobs/status?ids=" + ids, args.samples, args.warmup)
                assert result["routes"]["list"]["provider_calls"] == 0
                assert result["routes"]["status"]["provider_calls"] == 0
                _, raw = request(api.url, "/api/v1/print-jobs/perf-job-00001", access_token)
                assert len(json.loads(raw)["printJob"]["content"]) == args.body_bytes
                _, workspace_raw = request(api.url, "/api/v1/workspace", access_token)
                workspace = json.loads(workspace_raw)
                assert not workspace.get("printJobs"), "Workspace duplicated print history"
                assert any(device["id"] == "perf-device" for device in workspace["devices"])
                seen_ids = set()
                cursor = None
                page_count = 0
                pagination_start = time.perf_counter()
                pagination_calls = provider.calls
                while True:
                    path = "/api/v1/print-jobs?status=all&limit=100"
                    if cursor:
                        path += "&cursor=" + urllib.parse.quote(cursor, safe="")
                    _, raw = request(api.url, path, access_token)
                    page = json.loads(raw)
                    page_count += 1
                    assert len(page["printJobs"]) <= 100
                    for job in page["printJobs"]:
                        assert "content" not in job and job["id"] not in seen_ids
                        seen_ids.add(job["id"])
                    cursor = page["nextCursor"]
                    if not cursor:
                        break
                    assert page_count <= math.ceil(args.jobs / 100), "Cursor did not progress"
                assert len(seen_ids) == args.jobs, "Pagination omitted jobs"
                result["pagination_validation"] = {"rows": len(seen_ids), "pages": page_count,
                                                    "limit": 100, "content_in_summary": False,
                                                    "elapsed_ms": (time.perf_counter() - pagination_start) * 1000,
                                                    "provider_calls": provider.calls - pagination_calls}
            else:
                assert result["routes"]["list"]["provider_calls"] == args.samples * args.queued

        if candidate and args.verify_store_tests:
            test_env = dict(env, INK_TEST_DATABASE_URL=database.url)
            result["store_tests"] = command(["go", "test", "-race", "./internal/platform/store/postgres",
                                               "-count=1"], cwd=server_dir, env=test_env)
        metrics = result["routes"]["list"]
        print(f"{label}: list p95={metrics['p95_ms']:.2f}ms bytes={metrics['response_bytes'][0]} "
              f"foreground provider calls={metrics['provider_calls']}", flush=True)
        if args.core_only:
            return result

        # Fresh API process for each preview size isolates retained list-response heaps.
        result["render"] = []
        for size in (512, 4096):
            with API(binary, server_dir, env, temp_dir) as api:
                result["render"].append(render_memory(api, access_token, size, 3))

        with API(binary, server_dir, env, temp_dir) as api:
            due_epoch = database.sql("update print_schedules set enabled=true,next_run_at=now()+interval '0.5 second' returning extract(epoch from next_run_at);")
            deadline = time.monotonic() + 10
            while time.monotonic() < deadline:
                exists = database.sql("select count(*) from print_schedule_deliveries where print_schedule_id='perf-schedule' and status='printed';")
                if exists == "1":
                    break
                time.sleep(.05)
            else:
                raise RuntimeError("Due schedule did not create a local print job")
            wait_ms = database.sql("select extract(epoch from (j.created_at-(select last_run_at from print_schedules where id='perf-schedule')))*1000 from print_jobs j join print_schedule_deliveries d on d.print_job_id=j.id where d.print_schedule_id='perf-schedule';")
            due_wait_ms = database.sql(f"select (extract(epoch from j.created_at)-{due_epoch})*1000 from print_jobs j join print_schedule_deliveries d on d.print_job_id=j.id where d.print_schedule_id='perf-schedule';")
            result["due_schedule"] = {"scheduler_poll_interval_seconds": 2,
                                      "due_to_job_ms": float(due_wait_ms), "samples": 1,
                                      "claimed_to_job_ms": float(wait_ms), "confirmation_enabled": True,
                                      "polling_sleep_ms": 50}

        # Once the provider has answered 'not printed', completion must be found without a read request.
        provider.printed = False
        before = provider.calls
        sync_env = dict(env, PRINT_STATUS_SYNC_ENABLED="true")
        with API(binary, server_dir, sync_env, temp_dir) as sync_api:
            deadline = time.monotonic() + 5
            while candidate and provider.calls < before + args.queued and time.monotonic() < deadline:
                time.sleep(.02)
            if candidate:
                assert provider.calls >= before + args.queued, "Startup worker did not query all queued fixtures"
                time.sleep(.1)
            provider.printed = True
            start = time.monotonic()
            deadline = start + (15 if candidate else 2.5)
            synced = False
            while time.monotonic() < deadline:
                if database.sql("select count(*) from print_jobs where status='queued';") == "0":
                    synced = True
                    break
                time.sleep(.05)
            result["background_completion"] = {"completed_without_read": synced,
                "observed_wait_ms": (time.monotonic() - start) * 1000,
                "provider_calls": provider.calls - before, "healthy_recheck_seconds": 10,
                "worker_poll_interval_seconds": 2, "polling_sleep_ms": 50,
                "old_observation_timeout_seconds": 2.5 if not candidate else None}
            if synced != candidate:
                diagnostic = database.sql("select id,status,next_status_check_at,status_check_attempts from print_jobs where status='queued';")
                recent_logs = "\n".join(sync_api.log_path.read_text().splitlines()[-2:])
                raise AssertionError(f"Unexpected background sync behavior; provider_calls={provider.calls-before}; queued={diagnostic}; logs={recent_logs}")

        return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--baseline-ref", default="4399d13")
    parser.add_argument("--baseline-dir", type=pathlib.Path)
    parser.add_argument("--candidate-dir", type=pathlib.Path,
                        default=pathlib.Path(__file__).resolve().parents[2])
    parser.add_argument("--only", choices=("baseline", "candidate"))
    parser.add_argument("--jobs", type=int, default=1000)
    parser.add_argument("--queued", type=int, default=20)
    parser.add_argument("--body-bytes", type=int, default=4096)
    parser.add_argument("--samples", type=int, default=30)
    parser.add_argument("--warmup", type=int, default=3)
    parser.add_argument("--provider-delay-ms", type=float, default=20)
    parser.add_argument("--verify-store-tests", action="store_true")
    parser.add_argument("--core-only", action="store_true", help="Only measure foreground API routes")
    parser.add_argument("--assert-improvement", action="store_true",
                        help="Require at least 50%% less list p95 and 90%% fewer list/workspace bytes")
    parser.add_argument("--output", type=pathlib.Path, required=True)
    args = parser.parse_args()
    if not (1 <= args.queued <= 20 <= args.jobs <= 99999 and args.samples >= 1 and args.warmup >= 0
            and args.body_bytes >= 1 and args.provider_delay_ms >= 0):
        parser.error("Require 1 <= queued <= 20 <= jobs <= 99999, samples >= 1, nonnegative warmup/delay, positive bytes")
    if args.assert_improvement and args.only:
        parser.error("--assert-improvement requires both baseline and candidate")
    args.candidate_dir = args.candidate_dir.resolve()
    metadata = {"platform": platform.platform(), "machine": platform.machine(),
                "host_go": command(["go", "version"]), "postgres_image": "postgres:16",
                "postgres_image_id": command(["docker", "image", "inspect", "postgres:16", "--format", "{{.Id}}"]),
                "baseline_ref": args.baseline_ref,
                "candidate_head": command(["git", "rev-parse", "HEAD"], cwd=args.candidate_dir),
                "candidate_dirty": bool(command(["git", "status", "--porcelain"], cwd=args.candidate_dir)),
                "fixture": {"jobs": args.jobs, "queued": args.queued, "body_bytes_per_job": args.body_bytes,
                            "provider_delay_ms": args.provider_delay_ms,
                            "samples": args.samples, "warmup": args.warmup},
                "timestamp_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}
    provider = Provider(args.provider_delay_ms / 1000)
    provider_thread = threading.Thread(target=provider.serve_forever, daemon=True)
    provider_thread.start()
    try:
        with tempfile.TemporaryDirectory(prefix="ink-print-perf-") as temp:
            temp_dir = pathlib.Path(temp)
            baseline = args.baseline_dir
            if baseline is None and args.only != "candidate":
                baseline = temp_dir / "baseline-source"
                baseline.mkdir()
                archive = subprocess.Popen(["git", "archive", args.baseline_ref], cwd=args.candidate_dir,
                                           stdout=subprocess.PIPE)
                extraction = subprocess.run(["tar", "-x", "-C", str(baseline)], stdin=archive.stdout, check=True)
                archive.stdout.close()
                if archive.wait() != 0 or extraction.returncode != 0:
                    raise RuntimeError("Unable to extract baseline Git ref")
            report = {"metadata": metadata, "measurements": []}
            sources = [("baseline", baseline, False), ("candidate", args.candidate_dir, True)]
            for label, source, candidate in sources:
                if args.only and label != args.only:
                    continue
                print(f"Measuring {label} with disposable database and provider stub", flush=True)
                result = measure(source, label, candidate, args, provider, temp_dir)
                report["measurements"].append(result)
                args.output.parent.mkdir(parents=True, exist_ok=True)
                args.output.write_text(json.dumps(report, indent=2) + "\n")
            if args.assert_improvement:
                old, new = report["measurements"]
                old_list, new_list = old["routes"]["list"], new["routes"]["list"]
                gates = {"list_p95_ratio": new_list["p95_ms"] / old_list["p95_ms"],
                         "list_bytes_ratio": max(new_list["response_bytes"]) / min(old_list["response_bytes"]),
                         "workspace_bytes_ratio": max(new["routes"]["workspace"]["response_bytes"]) /
                                                  min(old["routes"]["workspace"]["response_bytes"])}
                report["improvement_gates"] = gates
                args.output.write_text(json.dumps(report, indent=2) + "\n")
                assert gates["list_p95_ratio"] < .5, "List p95 reduction was under 50%"
                assert gates["list_bytes_ratio"] < .1, "List bytes reduction was under 90%"
                assert gates["workspace_bytes_ratio"] < .1, "Workspace bytes reduction was under 90%"
                if not args.core_only:
                    assert [r["png_sha256"] for r in old["render"]] == [r["png_sha256"] for r in new["render"]]
            print(f"Raw measurements saved to {args.output}", flush=True)
    finally:
        provider.shutdown()
        provider.server_close()
        provider_thread.join()


if __name__ == "__main__":
    main()
