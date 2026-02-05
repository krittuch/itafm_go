#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/compose/integration.flow.yml"

cleanup() {
  if [[ "${KEEP_FLOW_STACK:-0}" == "1" ]]; then
    return
  fi
  docker compose -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

wait_for_postgres() {
  local retries=40
  local cmd=(docker compose -f "$COMPOSE_FILE" exec -T postgres psql -U test -d itafm_test -tAc 'select 1')
  until "${cmd[@]}" >/dev/null 2>&1; do
    retries=$((retries - 1))
    if [[ $retries -le 0 ]]; then
      echo "Postgres did not become ready in time"
      return 1
    fi
    sleep 2
  done
}

wait_for_gateway() {
  local retries=60
  until docker compose -f "$COMPOSE_FILE" logs --no-color gateway 2>/dev/null | grep -q "Connected to iTAFM"; do
    retries=$((retries - 1))
    if [[ $retries -le 0 ]]; then
      echo "Gateway did not report iTAFM connection in time"
      return 1
    fi
    sleep 2
  done
}

wait_for_java_broker() {
  local retries=60
  until docker compose -f "$COMPOSE_FILE" logs --no-color java_broker 2>/dev/null | grep -q "Route IDEP online"; do
    retries=$((retries - 1))
    if [[ $retries -le 0 ]]; then
      echo "Java broker did not report IDEP route readiness in time"
      return 1
    fi
    sleep 2
  done
}

publish_idep() {
  local payload
  payload='{"AircraftID":"THA0123","Departure":"VTBS","Destination":"VTSP","EOBT":"2026-02-05 10:00:00","DepartureRunway":"01R","ArrivalRunway":"","DepartureParkingStand":"B12","ArrivalParkingStand":"","EXOT":0,"TOBT":"2026-02-05 10:10:00","TSAT":"","AIBT":"","AOBT":"","URNO":"FLOWTEST-1"}'
  python3 - "$payload" <<'PY'
import socket
import sys

payload = sys.argv[1]
host = "127.0.0.1"
port = 16113

def send_frame(sock, frame: str):
    sock.sendall(frame.encode("utf-8") + b"\x00")

with socket.create_connection((host, port), timeout=10) as sock:
    send_frame(
        sock,
        "CONNECT\naccept-version:1.2\nhost:/\nlogin:admin\npasscode:admin\n\n",
    )
    response = sock.recv(4096).decode("utf-8", errors="ignore")
    if "CONNECTED" not in response:
        raise RuntimeError(f"ActiveMQ STOMP connect failed: {response!r}")

    send_frame(
        sock,
        "SEND\ndestination:/queue/IDEP_ITFM_Queue\ncontent-type:application/json\ncontent-length:"
        + str(len(payload.encode('utf-8')))
        + "\n\n"
        + payload,
    )
    send_frame(sock, "DISCONNECT\n\n")
PY
}

wait_for_db_update() {
  local retries=40
  while [[ $retries -gt 0 ]]; do
    local row
    row="$(docker compose -f "$COMPOSE_FILE" exec -T postgres psql -U test -d itafm_test -tAc "select coalesce(bay,''), coalesce(tobt,'') from flight_flight where flight_number='TG 123' limit 1" | tr -d '[:space:]')"
    if [[ "$row" == B12* && "$row" == *2026-02-0510:10:00+00* ]]; then
      return 0
    fi
    retries=$((retries - 1))
    sleep 2
  done

  echo "Timed out waiting for DB update"
  docker compose -f "$COMPOSE_FILE" logs --no-color java_broker gateway | tail -n 200
  return 1
}

echo "[flow-test] Starting integration stack..."
docker compose -f "$COMPOSE_FILE" up -d --build

echo "[flow-test] Waiting for postgres..."
wait_for_postgres

echo "[flow-test] Waiting for gateway startup..."
wait_for_gateway

echo "[flow-test] Waiting for java broker route startup..."
wait_for_java_broker

echo "[flow-test] Publishing IDEP message to ActiveMQ..."
publish_idep

echo "[flow-test] Verifying gateway wrote to database..."
wait_for_db_update

echo "[flow-test] PASS: ActiveMQ -> Java broker -> Kafka -> Go -> DB"
