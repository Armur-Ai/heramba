# Deployment

Heramba is a single static Go binary. Start in **observe** mode in front of your
app, watch the detections, then opt into **protect** when you trust them.

## 1. Build / install

```bash
make build            # -> ./bin/heramba
# or
go install github.com/Armur-Ai/heramba/cmd/heramba@latest
```

## 2. Configure

```bash
heramba config init          # writes a commented .heramba.yml (observe mode)
heramba doctor               # advisory readiness check
```

A minimal reverse-proxy config:

```yaml
mode: observe
listen: ":8080"
upstream: "http://127.0.0.1:3000"   # your real app
alert:
  format: jsonl
  file: "/var/log/heramba.jsonl"
```

## 3. Run

```bash
heramba serve                # reads .heramba.yml
# or override:
heramba serve --mode observe --listen :8080 --upstream http://127.0.0.1:3000
```

Point your ingress/load balancer at Heramba's `listen` address instead of the
app directly. Heramba forwards legitimate traffic to `upstream` untouched (in
observe mode) and emits detections to the alert sink.

## Deployment shapes

- **Reverse proxy** (recommended): Heramba in front, `upstream` is the app.
- **Honeypot-only**: omit `upstream`. Everything non-decoy returns 404; the box
  exists purely to catch probes/enumeration. Good for exposed decoy subdomains.
- **Sidecar**: run Heramba next to the app in the same pod/host, app listens on
  loopback, Heramba binds the public port.

## Containers

```dockerfile
FROM golang:1.27 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /heramba ./cmd/heramba

FROM gcr.io/distroless/static
COPY --from=build /heramba /heramba
COPY .heramba.yml /.heramba.yml
ENTRYPOINT ["/heramba","serve"]
```

## Going to protect mode

1. Run observe for long enough to confirm detections are what you expect.
2. Set `mode: protect`. This normalizes response headers and tarpits flagged
   sources; it requires an `upstream`.
3. (Optional) enable the egress firewall and list the hosts your app may call:

```yaml
mode: protect
upstream: "http://127.0.0.1:3000"
egress:
  enabled: true
  allowed_hosts: ["api.stripe.com", "db.internal"]
```

> Protect mode acts inline. Validate the allowlist first — an enabled egress
> firewall with an empty allowlist blocks **all** outbound calls (`heramba
> doctor` warns about this).
