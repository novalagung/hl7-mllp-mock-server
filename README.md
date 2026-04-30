# HL7 MLLP Mock Server

A lightweight mock server for HL7 v2 messages over MLLP (Minimal Lower Layer Protocol). Useful for testing HL7 integrations without a real receiver.

It exposes two TCP endpoints with different behaviors:

| Endpoint | Default Port | Behavior |
|---|---|---|
| ACK handler | `2575` | Always replies with `ACK` (application accept) |
| Chaos handler | `2576` | Always replies with `NACK` (application reject) |

## Pull the Image

Available on both Docker Hub and GHCR:

```bash
# Docker Hub
docker pull novalagung/hl7-mll-mock-server:latest

# GHCR
docker pull ghcr.io/novalagung/hl7-mll-mock-server:latest
```

## Run with Docker

```bash
docker run -d \
  -e ACK_PORT=2575 \
  -e CHAOS_PORT=2576 \
  -p 2575:2575 \
  -p 2576:2576 \
  novalagung/hl7-mll-mock-server:latest
```

## Run with Docker Compose

```bash
docker compose up -d
```

The included `docker-compose.yml` uses the Docker Hub image and exposes both ports with the default configuration.

To stop:

```bash
docker compose down
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `HOST` | `0.0.0.0` | Interface to bind |
| `ACK_PORT` | `2575` | Port for the always-ACK handler |
| `CHAOS_PORT` | `2576` | Port for the always-NACK chaos handler |

## Testing the Server

Send any valid HL7 v2 message wrapped in MLLP framing to either port. A quick smoke test with `netcat`:

```bash
# Send to ACK handler — expect MSA|AA back
printf '\x0bMSH|^~\&|sender|sender|receiver|receiver|20240101120000||ADT^A01^ADT_A01|MSG001|P|2.5\rEVN||20240101120000\rPID|||123456||Doe^John\r\x1c\x0d' | nc localhost 2575

# Send to chaos handler — expect MSA|AR back
printf '\x0bMSH|^~\&|sender|sender|receiver|receiver|20240101120000||ADT^A01^ADT_A01|MSG002|P|2.5\rEVN||20240101120000\rPID|||123456||Doe^John\r\x1c\x0d' | nc localhost 2576
```

---

## Local Build

If you want to build and run the image locally instead of pulling from a registry:

**Build the image:**

```bash
docker build -t hl7-mll-mock-server:local .
```

**Run directly:**

```bash
docker run -d \
  -e ACK_PORT=2575 \
  -e CHAOS_PORT=2576 \
  -p 2575:2575 \
  -p 2576:2576 \
  hl7-mll-mock-server:local
```

**Or override the image in Docker Compose:**

```bash
docker compose up -d --build
```

> This requires overriding the `image` field in `docker-compose.yml` with `build: .`, or using a local override file:

```yaml
# docker-compose.override.yml
services:
  hl7-ack:
    build: .
    image: hl7-mll-mock-server:local
```

Then run as usual:

```bash
docker compose up -d
```
