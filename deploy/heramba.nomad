# HashiCorp Nomad job for the Heramba edge (documented example — HCL; the repo
# vendors no HCL parser, so it is marker-checked, not parsed, in tests).
# Run: nomad job run deploy/heramba.nomad
# Observe mode is the safe default; point HERAMBA_UPSTREAM at your service.
job "heramba" {
  datacenters = ["dc1"]
  type        = "service"

  group "edge" {
    count = 2

    network {
      port "http" {
        static = 8080
      }
      port "admin" {
        static = 9090
      }
    }

    task "heramba" {
      driver = "docker"

      config {
        image = "ghcr.io/armur-ai/heramba:latest"
        args  = ["serve"]
        ports = ["http", "admin"]
      }

      env {
        HERAMBA_MODE         = "observe"
        HERAMBA_UPSTREAM     = "http://127.0.0.1:3000"
        HERAMBA_ADMIN_LISTEN = ":9090"
      }

      resources {
        cpu    = 200
        memory = 256
      }
    }
  }
}
