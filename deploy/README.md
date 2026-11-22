# Deploying Heramba

Heramba is a single static binary / container you put **in front of** your app.
It starts in **observe mode** — safe in production, it forwards every request
untouched and only logs detections — so you can deploy it the same afternoon and
flip to **protect** (inline) once you trust it.

All three shapes are configured entirely with `HERAMBA_*` env vars, so no config
file is required.

| Shape | File | Use it when |
|---|---|---|
| **One-liner** | — | quickest smoke test / single host |
| **Docker Compose** | [`docker-compose.yml`](docker-compose.yml) | a host or VM running your app in Docker |
| **Kubernetes sidecar** | [`k8s/sidecar.yaml`](k8s/sidecar.yaml) | your app runs in k8s; Heramba shares the pod |
| **Helm chart** | [`helm/heramba/`](helm/heramba/) | templated k8s install you tune via `values.yaml` |
| **Fly.io** | [`fly.toml`](fly.toml) | `fly deploy` from this repo or the published image |
| **Railway** | [`railway.json`](railway.json) | Railway build/deploy config (Dockerfile) |
| **Render** | [`render.yaml`](render.yaml) | Render Blueprint running the published image |
| **AWS ECS** | [`ecs-taskdef.json`](ecs-taskdef.json) | Fargate task definition |
| **GCP Cloud Run** | [`cloudrun-service.yaml`](cloudrun-service.yaml) | Knative service spec |
| **DigitalOcean** | [`.do/app.yaml`](.do/app.yaml) | App Platform spec |
| **Docker Swarm** | [`docker-stack.yml`](docker-stack.yml) | `docker stack deploy` |
| **Ansible** | [`ansible-deploy.yml`](ansible-deploy.yml) | `ansible-playbook` (community.docker) |
| **systemd** | [`heramba.service`](heramba.service) | bare-metal/VM service unit |
| **Nomad** | [`heramba.nomad`](heramba.nomad) | `nomad job run` |

### One-liner

```bash
docker run -p 8080:8080 \
  -e HERAMBA_UPSTREAM=http://host.docker.internal:3000 \
  -e HERAMBA_MODE=observe \
  ghcr.io/armur-ai/heramba:latest
```

### Docker Compose

Edit `HERAMBA_UPSTREAM` to point at your service, then:

```bash
docker compose -f deploy/docker-compose.yml up
curl -s -o /dev/null http://localhost:8080/.env   # trip a decoy
```

### Kubernetes sidecar

Set your app image + port in `k8s/sidecar.yaml`, then `kubectl apply -f` it.
Ingress hits the Service → Heramba (`:8080`) → your app on `localhost`.

### Helm chart

```
helm install heramba ./deploy/helm/heramba \
  --set config.upstream=http://myapp:3000 \
  --set config.mode=observe
```

Edit [`helm/heramba/values.yaml`](helm/heramba/values.yaml) for image, replicas,
mode/upstream/preset, resources and Service port. Observe mode is the default.

### Managed platforms (Fly.io / Railway / Render)

All three are env-configured — set `HERAMBA_UPSTREAM` to your app and keep
`HERAMBA_MODE=observe` until trusted:

- **Fly.io** — [`fly.toml`](fly.toml): `fly launch --no-deploy --copy-config` then `fly deploy`.
- **Railway** — [`railway.json`](railway.json): build from the `Dockerfile`, start command `serve`.
- **Render** — [`render.yaml`](render.yaml): a Blueprint running `ghcr.io/armur-ai/heramba`; set `HERAMBA_LISTEN` to Render's port.

### Configuration

Precedence is **flags > env > file (`.heramba.yml`) > defaults**. The common knobs:

| Env var | Meaning |
|---|---|
| `HERAMBA_UPSTREAM` | your app's address (empty = honeypot-only) |
| `HERAMBA_MODE` | `observe` (safe, default) or `protect` (inline) |
| `HERAMBA_PRESET` | framework lures: `node`/`next`/`rails`/`django`/`laravel`/`wordpress`/`spring` |
| `HERAMBA_LISTEN` | bind address (default `:8080`) |
| `HERAMBA_ADMIN_LISTEN` | health+metrics address, e.g. `:9090` (empty=off) |
| `HERAMBA_ALERT_FORMAT` | `text` or `jsonl` |
| `HERAMBA_ALERT_FILE` | sink path (empty = stdout) |

`MODE` and `UPSTREAM` are also read without the `HERAMBA_` prefix for the
one-liner. Run `heramba doctor` to see exactly what will run.
