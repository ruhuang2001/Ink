#!/usr/bin/env bash
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
INK_GRAFANA_ADMIN_PASSWORD=configuration-validation-only docker compose -f "$root/docker-compose.observability.yml" --profile tracing --profile logging config --quiet
docker run --rm -v "$root/monitoring/prometheus:/etc/prometheus:ro" --entrypoint /bin/promtool prom/prometheus:v3.5.0 check config /etc/prometheus/prometheus.yml
docker run --rm -v "$root/monitoring/prometheus:/etc/prometheus:ro" --entrypoint /bin/promtool prom/prometheus:v3.5.0 test rules /etc/prometheus/rules.test.yml
docker run --rm -v "$root/monitoring/alertmanager.yml:/etc/alertmanager.yml:ro" --entrypoint /bin/amtool prom/alertmanager:v0.28.1 check-config /etc/alertmanager.yml
docker run --rm -v "$root/monitoring/blackbox.yml:/etc/blackbox.yml:ro" prom/blackbox-exporter:v0.27.0 --config.file=/etc/blackbox.yml --config.check
docker run --rm -v "$root/monitoring/otel-collector.yml:/etc/otel.yml:ro" otel/opentelemetry-collector-contrib:0.135.0 validate --config=/etc/otel.yml
docker run --rm -v "$root/monitoring/loki.yml:/etc/loki.yml:ro" grafana/loki:3.5.5 -config.file=/etc/loki.yml -verify-config=true
docker run --rm -v "$root/monitoring/alloy.alloy:/etc/config.alloy:ro" grafana/alloy:v1.10.2 validate /etc/config.alloy
python3 - "$root/monitoring/grafana/dashboards/ink.json" <<'PY_DASHBOARD'
import json
import sys

with open(sys.argv[1]) as source:
    dashboard = json.load(source)
panels = {panel["id"]: panel for panel in dashboard["panels"]}
for panel_id in (2, 3, 4):
    for target in panels[panel_id]["targets"]:
        assert target.get("instant") is True and target.get("range") is False,             "Current-state panels must not reduce historical samples"
for panel_id in (2, 3):
    assert panels[panel_id]["fieldConfig"]["defaults"]["noValue"] == "No data",         "Missing health data must remain unknown"
PY_DASHBOARD
python3 "$root/server/scripts/observability_smoke.py"
