# iTAFM Gateway Stack

This repository runs a message pipeline:

1. `java_broker`: ActiveMQ destinations -> Kafka topics
2. `gateway`: Kafka topics -> PostgreSQL updates
3. `kafka`: Redpanda Kafka broker

## Compose services

- `kafka` in `/Users/krittuch/Projects/itafm_go/development.yml`
- `java_broker` in `/Users/krittuch/Projects/itafm_go/development.yml`
- `gateway` in `/Users/krittuch/Projects/itafm_go/development.yml`

## Monitor endpoints

Both runtime services expose HTTP monitor endpoints so you can debug flow without reading logs.

### Java broker monitor

- Default URL: `http://<host>:18080`
- Endpoints:
  - `GET /health`
  - `GET /routes`

Example:

```bash
curl -s http://127.0.0.1:18080/health | jq
curl -s http://127.0.0.1:18080/routes | jq
```

Key route fields:

- `online`: route is connected/subscribed
- `received_count`: messages read from ActiveMQ
- `forwarded_count`: messages written to Kafka
- `last_error`: most recent route error

### Gateway monitor

- Default URL: `http://<host>:18081`
- Endpoints:
  - `GET /health`
  - `GET /routes`

Example:

```bash
curl -s http://127.0.0.1:18081/health | jq
curl -s http://127.0.0.1:18081/routes | jq
```

Key route fields:

- `received_count`: messages consumed from Kafka
- `processed_count`: handler path processed the message
- `skipped_count`: handler received message but skipped processing
- `decode_error_count`: payload/command decoding errors
- `consumer_error_count`: Kafka consumer loop errors
- `last_error`: most recent consumer/decode error

## Monitor configuration

### Java broker env

- `MONITOR_ENABLED` (default: `true`)
- `MONITOR_HOST` (default: `0.0.0.0`)
- `MONITOR_PORT` (default: `18080`)

### Gateway env

- `GATEWAY_MONITOR_ENABLED` (default: `true`)
- `GATEWAY_MONITOR_HOST` (default: `0.0.0.0`)
- `GATEWAY_MONITOR_PORT` (default: `18081`)

## Quick debug workflow

1. Confirm Java broker route has `received_count > 0` and `forwarded_count > 0`.
2. Confirm gateway matching route has `received_count > 0`.
3. If gateway `received_count > 0` but `processed_count = 0`, inspect skip conditions (for example callsign conversion and route filters).
4. Use `last_error` from both monitors before checking container logs.
