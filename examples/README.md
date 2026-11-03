# Example: Heramba in front of a sample app

```bash
# 1. start the sample upstream
go run ./examples/sample-app &            # listens on 127.0.0.1:3000

# 2. run heramba in front of it (observe mode)
go run ./cmd/heramba serve --config examples/.heramba.yml

# 3. normal traffic is proxied through…
curl -s http://127.0.0.1:8080/api/health   # {"status":"ok"}

# 4. …but a probe of a decoy fires a zero-FP detection in heramba's log
curl -s -o /dev/null http://127.0.0.1:8080/.env
#   decoy_hit src=127.0.0.1 target="/.env" conf=1.00

# 5. and using the planted canary token is caught too
curl -s -o /dev/null -H "Authorization: Bearer hrmb_3a0ea0333739e3ff693757cd84aae412" \
     http://127.0.0.1:8080/api/health
#   canary_triggered conf=1.00
```

Observe mode never alters the sample app's responses — it only watches. Flip
`mode: protect` to see inline normalization + tarpitting.
