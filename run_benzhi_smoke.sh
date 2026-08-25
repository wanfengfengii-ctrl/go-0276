#!/usr/bin/env bash
# Deterministic local smoke test for the benzhi server. It builds the binary,
# starts the service on a loopback port with a temporary data file, probes the
# health and cycle API over HTTP, then tears everything down. No external
# network access is required and no process or temp file is left behind.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

PORT="${SMOKE_PORT:-18080}"
BASE="http://127.0.0.1:${PORT}"
DATA_FILE="$(mktemp -t benzhi-smoke.XXXXXX.json)"
BIN="$(mktemp -t benzhi-server.XXXXXX)"
SERVER_PID=""

cleanup() {
  if [[ -n "${SERVER_PID}" ]] && kill -0 "${SERVER_PID}" 2>/dev/null; then
    kill "${SERVER_PID}" 2>/dev/null || true
    wait "${SERVER_PID}" 2>/dev/null || true
  fi
  rm -f "${DATA_FILE}" "${BIN}"
}
trap cleanup EXIT

echo "building server binary"
go build -o "${BIN}" ./cmd/server

echo "starting server on ${PORT}"
ADDR="127.0.0.1:${PORT}" DATA_PATH="${DATA_FILE}" "${BIN}" &
SERVER_PID=$!

# Wait for the health endpoint to become ready (deterministic bounded retry).
ready=""
for _ in $(seq 1 50); do
  if resp="$(curl -sS --max-time 2 "${BASE}/healthz" 2>/dev/null || true)"; then
    if [[ "${resp}" == *'"status":"ok"'* ]]; then
      ready="1"
      break
    fi
  fi
  sleep 0.1
done
if [[ -z "${ready}" ]]; then
  echo "server did not become healthy" >&2
  exit 1
fi

# Assert the health payload captured in a variable (never pipe curl into grep,
# which can close the pipe early and fail curl with SIGPIPE).
health="$(curl -sS --max-time 5 "${BASE}/healthz")"
if [[ "${health}" != *'"status":"ok"'* ]]; then
  echo "unexpected health payload: ${health}" >&2
  exit 1
fi

# Exercise the cycle API: create+lock a cycle and read it back.
lock_body='{
  "cycle_id": "smoke-1",
  "greenhouse_id": "g1",
  "rule_version": 1,
  "colony_ids": ["c1", "c2"],
  "batch_ids": ["b1", "b2"],
  "reviewer_ids": ["r1", "r2"],
  "observation_points": [1, 2],
  "budget": 10,
  "windows": [
    {"colony_id": "c1", "zone_id": "z1", "start": 0, "end": 100},
    {"colony_id": "c2", "zone_id": "z2", "start": 0, "end": 100}
  ],
  "env_thresholds": {"temp_low": 0, "temp_high": 1000, "hum_low": 0, "hum_high": 1000, "co2_low": 0, "co2_high": 5000},
  "retry_spec": {"max_attempts": 3, "base_delay": 1, "step_delay": 2},
  "drift_layers": 1,
  "reentry_hours": 24,
  "operation_id": "smoke-op-1",
  "logical_time": 1000
}'

create_resp="$(curl -sS --max-time 5 -X POST "${BASE}/v1/cycles" \
  -H 'Content-Type: application/json' -d "${lock_body}")"
if [[ "${create_resp}" != *'"state":"pending_admit"'* ]]; then
  echo "unexpected create response: ${create_resp}" >&2
  exit 1
fi

get_resp="$(curl -sS --max-time 5 "${BASE}/v1/cycles/smoke-1")"
if [[ "${get_resp}" != *'"cycle_id":"smoke-1"'* || "${get_resp}" != *'"coverage_cells":4'* ]]; then
  echo "unexpected get response: ${get_resp}" >&2
  exit 1
fi

echo "smoke test passed"
