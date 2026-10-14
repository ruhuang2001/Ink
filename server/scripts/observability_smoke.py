#!/usr/bin/env python3
"""Exercise real API metrics/readiness/preview using disposable DB, no printers."""

import json
import os
import pathlib
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

from print_performance import API, Database, command, free_port, request, seed, token


def main():
    server = pathlib.Path(__file__).resolve().parents[1]
    with tempfile.TemporaryDirectory(prefix="ink-observability-") as temp, Database() as db:
        scratch = pathlib.Path(temp)
        binary = scratch / "api"
        migrate = scratch / "migrate"
        command(["go", "build", "-o", str(binary), "./cmd/api"], cwd=server)
        command(["go", "build", "-o", str(migrate), "./cmd/migrate"], cwd=server)
        (scratch / "migrations").symlink_to(server / "migrations", target_is_directory=True)
        secret = "disposable-observability-secret"
        metrics_port = free_port()
        env = dict(os.environ, DATABASE_URL=db.url, JWT_SECRET=secret, APP_NAME="ink-print-perf",
                   METRICS_ADDR=f"127.0.0.1:{metrics_port}", MEMOBIRD_ACCESS_KEY="", AI_CONFIG_ENCRYPTION_KEY="",
                   PRINT_STATUS_SYNC_ENABLED="false", SCHEDULER_POLL_INTERVAL="1s", PLUGIN_ROOT=str(scratch / "plugins"),
                   OTEL_EXPORTER_OTLP_ENDPOINT="", OTEL_EXPORTER_OTLP_TRACES_ENDPOINT="")
        command([str(migrate), "up"], cwd=scratch, env=env)
        seed(db, server, 20, 2, 100)
        access = token(secret)
        metrics_url = f"http://127.0.0.1:{metrics_port}/metrics"
        db.sql("alter system set log_statement = 'all';")
        db.sql("select pg_reload_conf();")
        assert db.sql("show log_statement;") == "all", "SQL logging did not activate"

        def database_log():
            result = subprocess.run(["docker", "logs", db.name], capture_output=True, text=True, check=True)
            return result.stdout + result.stderr

        def backlog_query_count():
            return database_log().count("select 'plugin_fetch', count(*)")

        db.sql("select 1 as ink_observability_sql_log_probe;")
        deadline = time.monotonic() + 3
        while "ink_observability_sql_log_probe" not in database_log() and time.monotonic() < deadline:
            time.sleep(.05)
        assert "ink_observability_sql_log_probe" in database_log(), "Database query logs unavailable"
        initial_backlog_queries = backlog_query_count()

        def read_metrics():
            with urllib.request.urlopen(metrics_url, timeout=3) as response:
                return response.read().decode()

        def status(url):
            try:
                with urllib.request.urlopen(url, timeout=3) as response:
                    return response.status
            except urllib.error.HTTPError as error:
                return error.code

        default_env = dict(env, METRICS_ADDR="")
        with API(binary, scratch, default_env, scratch) as default_api:
            assert status(default_api.url + "/metrics") == 404
            before = db.sql("select count(*) from print_jobs")
            default_synthetic = json.loads(command(["python3", str(server / "scripts/synthetic_preview.py")],
                env=dict(default_env, INK_SYNTHETIC_API_URL=default_api.url, INK_SYNTHETIC_ACCESS_TOKEN=access)))
            assert default_synthetic["success"] and len(default_synthetic["checks"]) == 4
            assert db.sql("select count(*) from print_jobs") == before, "Default synthetic created print jobs"
            with urllib.request.urlopen(default_api.url + "/healthz", timeout=3) as response:
                request_id = response.headers.get("X-Request-ID")
                assert request_id, "Default API omitted request ID"
        assert default_api.process.returncode == 0, "Default API did not shut down cleanly"
        records = [json.loads(line) for line in default_api.log_path.read_text().splitlines() if line.startswith("{")]
        assert any(record.get("msg") == "http request completed" and record.get("request_id") == request_id
                   for record in records), "Default API omitted request completion log"
        assert backlog_query_count() == initial_backlog_queries, "Default API queried database backlog"

        with API(binary, scratch, env, scratch) as api:
            assert status(api.url + "/metrics") == 404
            assert status(api.url + "/readyz") == 200
            before = db.sql("select count(*) from print_jobs")
            synthetic_env = dict(env, INK_SYNTHETIC_API_URL=api.url, INK_SYNTHETIC_ACCESS_TOKEN=access)
            synthetic = json.loads(command(["python3", str(server / "scripts/synthetic_preview.py")], env=synthetic_env))
            assert synthetic["success"] and len(synthetic["checks"]) == 4
            try:
                request(api.url, "/api/v1/print-preview", access, {"title": "Rejected", "content": "x"*8001})
            except urllib.error.HTTPError as error:
                assert error.code == 413
            else:
                raise AssertionError("Oversized preview accepted")
            assert db.sql("select count(*) from print_jobs") == before, "Synthetic created print jobs"
            deadline = time.monotonic() + 3
            body = read_metrics()
            while ('ink_backlog_items{queue="submitted_print"} 2' not in body or
                   backlog_query_count() == initial_backlog_queries) and time.monotonic() < deadline:
                time.sleep(.05)
                body = read_metrics()
            assert backlog_query_count() > initial_backlog_queries, "Enabled API did not query database backlog"
            for expected in ['ink_operations_total{operation="png_render",result="success"} 1',
                             'ink_operations_total{operation="png_render",result="error"} 1',
                             'ink_backlog_items{queue="submitted_print"} 2', 'ink_db_pool_max_connections']:
                assert expected in body, expected
            command(["docker", "pause", db.name])
            try:
                started = time.monotonic()
                assert status(api.url + "/readyz") == 503
                elapsed = time.monotonic()-started
                assert elapsed < 2, "Readiness did not time out"
                assert status(api.url + "/healthz") == 200
                assert "ink_backlog_items" in read_metrics(), "Scrape queried unavailable DB"
            finally:
                command(["docker", "unpause", db.name])
            assert status(api.url + "/readyz") == 200
        assert api.process.returncode == 0, "API did not shut down cleanly"
        print(json.dumps({"schema_version": 1, "success": True, "default_synthetic": default_synthetic,
                          "default_backlog_queries": 0, "default_shutdown_exit": 0, "synthetic": synthetic,
                          "database_outage_ready_seconds": round(elapsed, 3), "shutdown_exit": 0}))


if __name__ == "__main__":
    main()
