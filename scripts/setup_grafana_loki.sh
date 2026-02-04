#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(pwd)"
OUT_DIR="${ROOT_DIR}/grafana-loki"

mkdir -p "${OUT_DIR}/provisioning/datasources" "${OUT_DIR}/provisioning/dashboards" "${OUT_DIR}/dashboards"

cat > "${OUT_DIR}/docker-compose.yml" <<'YAML'
version: "3.8"
services:
  loki:
    image: grafana/loki:2.9.6
    command: -config.file=/etc/loki/local-config.yaml
    volumes:
      - ./loki-config.yaml:/etc/loki/local-config.yaml:ro
      - ./data/loki:/loki
    ports:
      - "3100:3100"

  promtail:
    image: grafana/promtail:2.9.6
    command: -config.file=/etc/promtail/config.yml
    volumes:
      - ./promtail-config.yaml:/etc/promtail/config.yml:ro
      - /var/log/edgeprobe:/var/log/edgeprobe:ro
      - ./data/promtail:/promtail

  grafana:
    image: grafana/grafana-oss:10.4.3
    environment:
      - GF_SECURITY_ADMIN_USER=admin
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_AUTH_ANONYMOUS_ENABLED=false
    ports:
      - "3000:3000"
    volumes:
      - ./provisioning:/etc/grafana/provisioning
      - ./dashboards:/var/lib/grafana/dashboards
      - ./data/grafana:/var/lib/grafana
YAML

cat > "${OUT_DIR}/loki-config.yaml" <<'YAML'
auth_enabled: false

server:
  http_listen_port: 3100

common:
  path_prefix: /loki
  storage:
    filesystem:
      chunks_directory: /loki/chunks
      rules_directory: /loki/rules
  replication_factor: 1
  ring:
    kvstore:
      store: inmemory

schema_config:
  configs:
    - from: 2023-01-01
      store: boltdb-shipper
      object_store: filesystem
      schema: v12
      index:
        prefix: index_
        period: 24h

limits_config:
  retention_period: 168h
YAML

cat > "${OUT_DIR}/promtail-config.yaml" <<'YAML'
server:
  http_listen_port: 9080
  grpc_listen_port: 0

positions:
  filename: /promtail/positions.yaml

clients:
  - url: http://loki:3100/loki/api/v1/push

scrape_configs:
  - job_name: edgeprobe
    static_configs:
      - targets:
          - localhost
        labels:
          job: edgeprobe
          __path__: /var/log/edgeprobe/*.jsonl
    pipeline_stages:
      - json:
          expressions:
            type: type
            target: target
            outage_id: outage_id
            duration_ms: duration_ms
      - labels:
          type: type
          target: target
          outage_id: outage_id
YAML

cat > "${OUT_DIR}/provisioning/datasources/loki.yml" <<'YAML'
apiVersion: 1

datasources:
  - name: Loki
    type: loki
    access: proxy
    url: http://loki:3100
    isDefault: true
YAML

cat > "${OUT_DIR}/provisioning/dashboards/dashboards.yml" <<'YAML'
apiVersion: 1

providers:
  - name: Edgeprobe
    orgId: 1
    folder: ""
    type: file
    disableDeletion: false
    editable: true
    options:
      path: /var/lib/grafana/dashboards
YAML

cat > "${OUT_DIR}/dashboards/edgeprobe-outages.json" <<'JSON'
{
  "annotations": {"list": []},
  "editable": true,
  "fiscalYearStartMonth": 0,
  "graphTooltip": 0,
  "panels": [
    {
      "datasource": {"type": "loki", "uid": "Loki"},
      "fieldConfig": {"defaults": {"unit": "short"}, "overrides": []},
      "gridPos": {"h": 8, "w": 12, "x": 0, "y": 0},
      "id": 1,
      "options": {"reduceOptions": {"values": false, "calcs": ["lastNotNull"]}},
      "targets": [
        {
          "expr": "count_over_time({job=\"edgeprobe\"} | json | type=\"outage_summary\" [1h])",
          "refId": "A"
        }
      ],
      "title": "Outages per Hour",
      "type": "stat"
    },
    {
      "datasource": {"type": "loki", "uid": "Loki"},
      "fieldConfig": {"defaults": {"unit": "ms"}, "overrides": []},
      "gridPos": {"h": 8, "w": 12, "x": 12, "y": 0},
      "id": 2,
      "options": {"legend": {"displayMode": "list"}},
      "targets": [
        {
          "expr": "sum_over_time(({job=\"edgeprobe\"} | json | type=\"outage_summary\" | unwrap duration_ms) [1h])",
          "refId": "A"
        }
      ],
      "title": "Total Outage Duration (1h)",
      "type": "timeseries"
    }
  ],
  "schemaVersion": 38,
  "style": "dark",
  "tags": ["edgeprobe"],
  "templating": {"list": []},
  "time": {"from": "now-24h", "to": "now"},
  "timepicker": {},
  "timezone": "browser",
  "title": "Edgeprobe Outages",
  "uid": "edgeprobe-outages",
  "version": 1
}
JSON

echo "Grafana + Loki config generated at ${OUT_DIR}"
