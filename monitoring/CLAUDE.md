# Monitoring (`monitoring/`)

Prometheus (`prometheus/`), Grafana (`grafana/`: provisioning and the dashboards as JSON), Loki
(`loki/loki.yml`) and Grafana Alloy (`alloy/config.alloy`), run by both compose files. `deploy.yml`
copies this directory to the host with the compose file.

## Rules

- **No IP address is stored.** Alloy replaces every IPv4/IPv6 address with `<ip>` before shipping logs
  to Loki (Caddy's access log carries visitors' addresses). The Privacy policy depends on this; Loki
  keeps logs for `LOKI_RETENTION_PERIOD` (336h in production).
- **Alloy reaches Docker only through `docker-socket-proxy`** (GETs for containers, networks and logs;
  `POST: 0`) on the internal `docker-api` network. Never mount the Docker socket into Alloy: even `:ro`
  answers every API call, which is root on the host.
- Loki has no shell for a healthcheck: Prometheus scrapes it and Alloy (`up{job=~"loki|alloy"}`). Both
  run with `GOMEMLIMIT`; Go ignores a container's memory limit otherwise.
- Grafana's Postgres datasource and postgres-exporter connect as **`metrics_ro`** (read-only, 15 s
  `statement_timeout`, explicit per-table grants). A new table a dashboard reads is granted in a new
  migration; **never grant it a credential column.**
- Grafana (:3000) and Prometheus (:9090) bind to `127.0.0.1` in production; `/metrics` and
  `/health/detail` are 404 from outside (Caddy).
- **A metric must reflect reality.** A component that is down is never reported as connected; a new
  failure path gets a counter.
- A dashboard query names the metric families that binary actually registers (each binary registers
  only its own), or the panel shows a misleading zero.
