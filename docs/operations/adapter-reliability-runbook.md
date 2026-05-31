# Adapter Reliability Runbook

Operator response procedures for the SPEC-EVAL-002 adapter reliability alerts.
Each alert's `runbook_url` annotation links to the matching section anchor
below. Dashboards live in Grafana at `/d/adapter-reliability` (auto-provisioned
via `deploy/grafana/`).

V1 ships **three** alerts. The circuit-open alert and circuit-state dashboard
panel are deferred to post-V1 (amendment A2): no upstream emits
`usearch_adapter_circuit_state` yet, so they would never fire. The metric
family is registered for forward compatibility and the alert/panel are
re-enabled when a future resilience SPEC emits real circuit transitions.

---

## AdapterSuccessRate7dLow

**What you see:** Adapter `<name>` 7-day rolling success rate has stayed below
85% for 30 minutes (`usearch:adapter_success_rate_7d < 0.85`). Severity:
warning.

**Immediately check:**

1. Open the Grafana "Per-adapter 7d success rate (trendline)" panel and select
   the affected adapter from the `adapter` template variable.
2. Open the "Failure cause breakdown by outcome" panel to see whether the
   failures are `timeout`, `rate_limited`, `unavailable`, or `failure`.
3. Drill into structured logs filtered to
   `adapter=<name> AND outcome!=success` and inspect the `failure_class`
   attribute (`5xx` / `4xx` / `dns` / `tls` / `parse` / `transcript` /
   `unknown`) for the finer cause.

**Short-term mitigation:**

- `rate_limited` heavy (Naver / X): rotate the API key or back off request
  volume. Add an Alertmanager silence during the rotation window:
  `amtool silence add alertname=AdapterSuccessRate7dLow adapter=<name> --duration=2h --comment="key rotation"`.
- `4xx` heavy (Reddit / GitHub OAuth): the token likely expired or lost
  scope. Refresh the OAuth token / re-grant the required scope.
- `parse` heavy (KoreaNewsCrawler): upstream HTML structure changed — update
  the crawler/parser library.

**Long-term:** if the adapter is structurally broken (upstream gone), disable
it via `POST /api/admin/adapters/<id>/toggle` (loopback only) so users stop
seeing partial-result gaps, and track a follow-up fix.

---

## AdapterSuccessRate1hCritical

**What you see:** Adapter `<name>` 1-hour success rate dropped below 50% for 5
minutes (`usearch:adapter_success_rate_1h < 0.50`) — an acute outage.
Severity: critical.

**Immediately check:**

1. Confirm it is real and not a low-traffic false positive — a single failed
   call on an adapter with very low volume can cross 50%. Check the absolute
   call rate on the failure-cause panel.
2. Check whether a deploy or config change landed in the last hour
   (`git log`, deploy history).
3. Probe the upstream directly (curl / `GET /api/admin/adapters` →
   `success_rate`, or `GET /api/admin/adapters/health`).

**Short-term mitigation:**

- Upstream down (`unavailable` / `dns` / `timeout`): nothing to fix locally;
  wait for upstream recovery and silence to avoid alert fatigue.
- SearXNG upstream search engine blocked: enable a different upstream engine
  in the SearXNG config.
- YouTube transcript extraction broken (`transcript`): verify the `yt-dlp`
  version and bump if a breaking upstream change shipped.

**Long-term:** if a specific upstream is chronically flaky, consider raising the
per-adapter timeout or adding a retry budget (future resilience SPEC).

---

## FanoutPartialRatioHigh

**What you see:** Adapter `<name>` contributed an error to more than 30% of
fanout dispatches over 24h (`usearch:adapter_fanout_partial_ratio_24h > 0.30`)
for 15 minutes. Users may be silently receiving partial results without this
source. Severity: warning.

**Immediately check:**

1. Cross-reference the adapter's `usearch:adapter_success_rate_24h` — a high
   partial ratio usually tracks a low success rate.
2. Determine whether the adapter is failing on every dispatch or intermittently
   (failure-cause breakdown panel).

**Short-term mitigation:** same per-cause playbook as `AdapterSuccessRate7dLow`
(rotate keys for `rate_limited`, refresh OAuth for `4xx`, update parser for
`parse`). If the source is degraded but non-critical, an operator may disable
it so the partial-ratio noise stops.

**Long-term:** evaluate whether the adapter belongs in the default fanout set
for its category if it is unreliable enough to routinely degrade results.

---

## Threshold tuning

The default thresholds (85% / 50% / 30%) and `for:` durations (30m / 5m / 15m)
are recommendations. For low-traffic adapters, raise the `for:` duration or
lower the success-rate threshold in `deploy/prometheus/alerts.yml` to reduce
false positives — a handful of failed calls per hour on a rarely-used adapter
can cross 50% without indicating a real outage.

## Prometheus storage sizing

Approximate footprint: 12 adapters × 6 outcomes ≈ 72 active series for the
calls counter, plus the recording-rule output series. At ~1 req/sec sustained
over 30-day retention this is on the order of 50–100 MB of compressed TSDB on
disk. Operators with larger adapter fleets or higher traffic should size
`prometheus_data` and `--storage.tsdb.retention.time` accordingly.

## Silencing during planned maintenance

During a planned maintenance window (e.g. an API key rotation), silence the
relevant alert to avoid paging:

```
amtool silence add alertname=AdapterSuccessRate7dLow adapter=naver \
  --duration=2h --comment="planned Naver key rotation"
```

List and expire silences with `amtool silence query` / `amtool silence expire`.
