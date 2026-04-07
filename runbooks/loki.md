# Loki Alerts Runbook

## Table of Contents

- [Loki Alerts Runbook](#loki-alerts-runbook)
  - [Table of Contents](#table-of-contents)
  - [Environment \& Access Information](#environment--access-information)
    - [Multi-Cluster Environment](#multi-cluster-environment)
    - [Prometheus Query Access](#prometheus-query-access)
    - [Deployment Overview](#deployment-overview)
  - [LokiRequestErrors](#lokirequesterrors)
  - [LokiRequestPanics](#lokirequestpanics)
  - [LokiRequestLatency](#lokirequestlatency)
  - [LokiTenantRateLimit](#lokitenantratelimit)
  - [LokiWritePathHighLoad](#lokiwritepathhighload)
  - [LokiReadPathHighLoad](#lokireadpathhighload)
  - [LokiDiscardedSamplesWarning](#lokidiscardedsampleswarning)
  - [LokiIngesterFlushFailureRateCritical](#lokiingesterflushfailureratecritical)
  - [LokistackComponentsNotReadyWarning](#lokistackcomponentsnotreadywarning)

---

## Environment & Access Information

### Multi-Cluster Environment

This operator runs across multiple private Kubernetes clusters. To troubleshoot alerts:

- **Identify the cluster** from the alert labels (typically `cluster` or `prometheus` label)
- **Access Grafana for metrics:**
   - Navigate to Loki dashboards (linked in alert) or use Explore view for LogQL/PromQL queries
   - URL: `https://grafana.app-sre.devshift.net/?orgId=1`
   - Select datasource: `<cluster>-prometheus` (e.g., `rhobsp02ue1-prometheus`)
- **Access the cluster (for kubectl commands):**
   - Visit the cluster page: `https://visual-app-interface.devshift.net/clusters/<cluster-name>`
   - Follow the sshuttle access instructions provided on the cluster page
- **View Configuration:**
   - Configuration repository: `https://gitlab.cee.redhat.com/rhobs/configuration/-/tree/main/resources/clusters/production/<cluster>/logs/bundle`
   - Contains LokiOperator and the `LokiStack` CR (`observatorium-lokistack`)

### Prometheus Query Access

For all PromQL metric queries in this runbook:
- **Preferred Method:** Use **Grafana** at `https://grafana.app-sre.devshift.net/?orgId=1`
  - Select the `<cluster>-prometheus` datasource (e.g., `rhobsp01ue1-prometheus`)
  - Use the Loki dashboards for pre-built visualizations
  - Copy-paste PromQL queries from investigative steps for ad-hoc exploration in the Explore view
- **Alternative:** Access Prometheus UI directly via sshuttle tunnel after cluster access
- **Query Placeholders:**
  - Replace `<cluster>` with actual cluster name from alert labels
  - Replace `<namespace>` with `rhobs-production` (the namespace used across all clusters)
  - Replace `<pod>` with the specific pod name from alert labels or `kubectl get pods` output

### Deployment Overview

The LokiStack is deployed as a single CR (`observatorium-lokistack`) in `rhobs-production` across all production clusters. The Loki Operator manages all component lifecycles. Key deployment parameters per cluster:

| Parameter | Value |
|---|---|
| LokiStack CR | `observatorium-lokistack` |
| Namespace | `rhobs-production` |
| Operator namespace | `rhobs-production` |
| Storage backend | S3 (secret: `loki-default-bucket`) |
| Storage schema | v13 (effective 2025-06-06) |
| Global ingestion rate | 20 MB/s (burst: 256 MB) |
| Per-stream rate limit | 15 MB/s (burst: 30 MB) |
| Max line size | 2 MB |
| Query timeout | 5 minutes |
| Distributor replicas | 3 |
| Ingester replicas | 3 |
| Querier replicas | 2 |
| Query-frontend replicas | 2 |

---

## LokiRequestErrors

**Severity:** `warning` | **For:** 15m | **Component:** All Loki services

**Summary:**
At least 10% of requests to a Loki service are returning 5xx server errors. A sustained error rate of this magnitude indicates a systemic issue — not just transient failures — and risks data loss if the write path is affected.

**Impact:**
Log ingestion or querying may be degraded or completely broken for affected routes. Depending on the failing component, upstream log shippers may start dropping logs if retries are exhausted.

**Alert Expression:**
```promql
sum by (job, namespace, route) (
  job_namespace_route_statuscode:loki_request_duration_seconds_count:irate1m{status_code=~"5.."}
)
/
sum by (job, namespace, route) (
  job_namespace_route_statuscode:loki_request_duration_seconds_count:irate1m
)
* 100 > 10
```

**Steps:**

- **Identify the failing component and route** — the alert labels include `job` and `route`. Common values:
  - `job`: `loki-distributor`, `loki-ingester`, `loki-querier`, `loki-query-frontend`, `loki-index-gateway`, `loki-compactor`
  - `route`: `/loki/api/v1/push`, `/loki/api/v1/query_range`, `/loki/api/v1/series`, etc.

- **Check the error rate per component** in Grafana (datasource: `<cluster>-prometheus`):
  ```promql
  sum by (job, route, status_code) (
    job_namespace_route_statuscode:loki_request_duration_seconds_count:irate1m{
      namespace="rhobs-production",
      status_code=~"5.."
    }
  )
  ```

- **Inspect logs of the failing component** for error details:
  ```bash
  kubectl logs -n rhobs-production -l app.kubernetes.io/component=<component> --tail=100 | grep -E '"status":"5[0-9]{2}|level=error'
  ```

- **Check WAL health** — WAL issues are a common root cause for ingester 5xx errors:
  ```promql
  sum by (pod, namespace) (rate(loki_ingester_wal_disk_full_failures_total[5m]))
  ```
  ```promql
  sum by (pod, namespace) (rate(loki_ingester_wal_corruptions_total[5m]))
  ```

- **Verify all components can reach S3 storage** — storage connectivity failures propagate as 5xx errors:
  ```bash
  kubectl logs -n rhobs-production -l app.kubernetes.io/component=ingester --tail=200 | grep -i "storage\|s3\|bucket\|connection"
  ```

- **Check pod resource pressure** — OOM kills or CPU throttling can cause 5xx spikes:
  ```bash
  kubectl top pods -n rhobs-production -l app.kubernetes.io/part-of=lokistack
  kubectl get events -n rhobs-production --sort-by='.lastTimestamp' | grep -i "oom\|kill\|evict"
  ```

- **Check the LokiStack CR status** for any reported conditions:
  ```bash
  kubectl describe lokistack observatorium-lokistack -n rhobs-production
  ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs/top/describe` on pods in `rhobs-production`
- Grafana access to query `<cluster>-prometheus` datasource
- Access to `loki-default-bucket` S3 secret (read-only) to validate credentials

---

## LokiRequestPanics

**Severity:** `warning` | **For:** — (fires immediately) | **Component:** All Loki services

**Summary:**
A Loki service component has experienced one or more Go runtime panics. A panic causes the goroutine (and potentially the entire process) to crash, resulting in service disruption and potential data loss.

**Impact:**
The panicking component is unavailable for the duration of the crash and restart. If ingesters panic during a flush, in-memory chunks not yet persisted to S3 may be lost. If crash-loops occur, the component may be unavailable for extended periods.

**Alert Expression:**
```promql
sum by (job, namespace) (increase(loki_panic_total[10m])) > 0
```

**Steps:**

- **Identify the panicking component** from the alert `job` label. Check for crash-loops:
  ```bash
  kubectl get pods -n rhobs-production -l app.kubernetes.io/part-of=lokistack
  ```

- **Collect the panic stack trace** — panics produce a full goroutine dump in stderr:
  ```bash
  kubectl logs -n rhobs-production <panicking-pod> --previous | grep -A 50 "panic:"
  ```
  If the pod has not restarted yet:
  ```bash
  kubectl logs -n rhobs-production <panicking-pod> | grep -A 50 "panic:"
  ```

- **Monitor the panic rate** over time in Grafana:
  ```promql
  sum by (job, namespace) (increase(loki_panic_total[10m]))
  ```

- **Check for recent changes** — panics often follow version upgrades or configuration changes:
  ```bash
  kubectl get deployment,statefulset -n rhobs-production -l app.kubernetes.io/part-of=lokistack -o jsonpath='{range .items[*]}{.metadata.name}: {.spec.template.spec.containers[0].image}{"\n"}{end}'
  ```

- **Check if the issue is query-triggered** — some panics only occur for specific query patterns. Look at the query-frontend logs:
  ```bash
  kubectl logs -n rhobs-production -l app.kubernetes.io/component=query-frontend --tail=200 | grep -i "panic\|error"
  ```

- **If crash-looping**, inspect the LokiStack CR and operator for reconciliation errors:
  ```bash
  kubectl describe lokistack observatorium-lokistack -n rhobs-production
  kubectl logs -n rhobs-production -l app.kubernetes.io/name=loki-operator --tail=100
  ```

- **Check the configuration** in the repository for the affected cluster: `https://gitlab.cee.redhat.com/rhobs/configuration/-/tree/main/resources/clusters/production/<cluster>/logs/bundle`

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs/get/describe` on pods and deployments in `rhobs-production`
- Grafana access to track panic rate

---

## LokiRequestLatency

**Severity:** `warning` | **For:** 15m | **Component:** All Loki services

**Summary:**
The 99th percentile request latency for a Loki service has exceeded 1 second for 15 consecutive minutes. Sustained high latency indicates a performance bottleneck in the query or write path.

**Impact:**
Log queries from dashboards and alerting rules take longer to return results. If latency affects the write path, log shippers may time out, causing data loss.

**Alert Expression:**
```promql
histogram_quantile(
  0.99,
  sum by (job, namespace, route, le) (
    irate(loki_request_duration_seconds_bucket{route!~"(?i).*tail.*"}[2m])
  )
) > 1
```

**Steps:**

- **Check which route is slow** — the alert labels include `job` and `route`:
  - Write path: `/loki/api/v1/push` → investigate distributors/ingesters
  - Query path: `/loki/api/v1/query`, `/loki/api/v1/query_range` → investigate queriers/query-frontend

- **Check latency by component and route** in Grafana:
  ```promql
  histogram_quantile(
    0.99,
    sum by (job, route, le) (
      irate(loki_request_duration_seconds_bucket{
        namespace="rhobs-production",
        route!~"(?i).*tail.*"
      }[2m])
    )
  )
  ```

- **Check the query scheduler queue depth** — a large queue indicates querier saturation:
  ```promql
  cortex_query_scheduler_inflight_requests{namespace="rhobs-production"}
  ```

- **Check query-frontend for slow or expensive queries**:
  ```bash
  kubectl logs -n rhobs-production -l app.kubernetes.io/component=query-frontend --tail=200 | grep -E "took|slow|timeout"
  ```

- **Check querier resource pressure**:
  ```bash
  kubectl top pods -n rhobs-production -l app.kubernetes.io/component=querier
  ```

- **Check S3 storage latency** — storage backend slowness propagates to query latency:
  ```promql
  histogram_quantile(0.99, sum by (operation, le) (
    rate(loki_gcs_request_duration_seconds_bucket{namespace="rhobs-production"}[5m])
  ))
  ```
  Or for S3:
  ```promql
  histogram_quantile(0.99, sum by (operation, le) (
    rate(loki_s3_request_duration_seconds_bucket{namespace="rhobs-production"}[5m])
  ))
  ```

- **Check ingester memory and WAL** — large WAL replay causes write latency:
  ```promql
  sum by (pod) (loki_ingester_wal_bytes_in_use{namespace="rhobs-production"})
  ```

- **Check the configured query timeout** — our global timeout is 5 minutes. Queries nearing this will show as latency spikes:
  ```bash
  kubectl get lokistack observatorium-lokistack -n rhobs-production -o jsonpath='{.spec.limits.global.queries.queryTimeout}'
  ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl top/logs` on pods in `rhobs-production`
- Grafana access to query `<cluster>-prometheus` datasource

---

## LokiTenantRateLimit

**Severity:** `warning` | **For:** 15m | **Component:** Loki Distributor

**Summary:**
At least 10% of requests to a Loki route are being rejected with HTTP 429 (Too Many Requests). A tenant is exceeding the configured ingestion rate limits.

**Impact:**
Log entries from the rate-limited tenant are being dropped. Depending on the log shipper's retry behavior, some logs may be permanently lost.

**Alert Expression:**
```promql
sum by (job, namespace, route) (
  job_namespace_route_statuscode:loki_request_duration_seconds_count:irate1m{status_code="429"}
)
/
sum by (job, namespace, route) (
  job_namespace_route_statuscode:loki_request_duration_seconds_count:irate1m
)
* 100 > 10
```

**Steps:**

- **Identify the affected tenant and discard reason** — query in Grafana:
  ```promql
  sum by (namespace, tenant, reason) (
    irate(loki_discarded_samples_total{namespace="rhobs-production"}[2m])
  )
  ```

- **Map the reason to the configuration limit** and adjust accordingly in the `LokiStack` CR:

  | Reason | LokiStack CRD field |
  |---|---|
  | `rate_limited` | `ingestionRate`, `ingestionBurstSize` |
  | `stream_limit` | `maxGlobalStreamsPerTenant` |
  | `label_name_too_long` | `maxLabelNameLength` |
  | `label_value_too_long` | `maxLabelValueLength` |
  | `line_too_long` | `maxLineSize` |
  | `max_label_names_per_series` | `maxLabelNamesPerSeries` |
  | `per_stream_rate_limit` | `perStreamRateLimit`, `perStreamRateLimitBurst` |

- **Check current global limits** (as configured in `resources/clusters/production/<cluster>/logs/bundle/03-lokistack-LokiStack.yaml`):
  - Global ingestion rate: **20 MB/s** (burst: 256 MB)
  - Per-stream rate limit: **15 MB/s** (burst: 30 MB)
  - Max line size: **2 MB**

- **Check the distributor logs** for rate limiting events (Loki 3.1.0+ provides stream-level detail):
  ```bash
  kubectl logs -n rhobs-production -l app.kubernetes.io/component=distributor --tail=200 | grep -E "rate_limit|429|tenant"
  ```

- **Check the current ingestion rate** per tenant:
  ```promql
  sum by (tenant) (
    irate(loki_distributor_bytes_received_total{namespace="rhobs-production"}[2m])
  )
  ```

- **If the rate limit is justified**, adjust the tenant limits in the LokiStack CR:
  ```yaml
  spec:
    limits:
      tenants:
        <tenant-name>:
          ingestion:
            ingestionRate: <new-rate>
            ingestionBurstSize: <new-burst>
  ```
  Submit via MR to: `https://gitlab.cee.redhat.com/rhobs/configuration/-/tree/main/resources/clusters/production/<cluster>/logs/bundle`

- **If the rate spike is unexpected**, investigate the log shipper or application causing the burst and coordinate with the tenant team to fix misconfigured logging.

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs` on distributor pods in `rhobs-production`
- Grafana access to inspect per-tenant metrics
- MR access to the configuration repository to adjust limits

---

## LokiWritePathHighLoad

**Severity:** `warning` | **For:** 15m | **Component:** Loki Ingester

**Summary:**
The Loki write path is experiencing high load: ingesters are actively replaying WAL segments to flush data to S3. This WAL replay flush is a backpressure mechanism that fires when ingesters fall behind on normal chunk flushing.

**Impact:**
High write path load degrades ingestion throughput and can increase tail latency for log queries. If the condition persists, ingesters may eventually exhaust WAL disk, triggering data loss.

**Alert Expression:**
```promql
sum by (job, namespace) (loki_ingester_wal_replay_flushing) > 0
```

**Steps:**

- **Check which ingesters are under WAL replay pressure** in Grafana:
  ```promql
  loki_ingester_wal_replay_flushing{namespace="rhobs-production"}
  ```

- **Check WAL disk usage** — confirm available space before taking action:
  ```promql
  sum by (pod, namespace) (loki_ingester_wal_bytes_in_use{namespace="rhobs-production"})
  ```

- **Check for WAL disk full failures** — this is the failure mode to avoid:
  ```promql
  sum by (pod, namespace) (rate(loki_ingester_wal_disk_full_failures_total{namespace="rhobs-production"}[5m]))
  ```

- **Check ingester resource usage**:
  ```bash
  kubectl top pods -n rhobs-production -l app.kubernetes.io/component=ingester
  ```

- **Inspect ingester logs** for flush activity and errors:
  ```bash
  kubectl logs -n rhobs-production -l app.kubernetes.io/component=ingester --tail=200 | grep -E "WAL|flush|error"
  ```

- **Check S3 write throughput** — slow storage flushes cause WAL to accumulate:
  ```promql
  sum by (pod) (rate(loki_ingester_chunks_flushed_total{namespace="rhobs-production"}[5m]))
  ```

- **Check ingestion rate vs. configured limits** — the global rate is 20 MB/s. If traffic exceeds this persistently, consider:
  1. Adjusting per-tenant rate limits to better distribute load
  2. Scaling ingesters (currently 3 replicas) — update the `LokiStack` CR: `spec.template.ingester.replicas`

- **Verify S3 connectivity and throughput** from the ingester pods:
  ```bash
  kubectl logs -n rhobs-production -l app.kubernetes.io/component=ingester --tail=200 | grep -i "s3\|storage\|upload"
  ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl top/logs` on ingester pods in `rhobs-production`
- Grafana access to query `<cluster>-prometheus` datasource
- MR access to the configuration repository to scale ingesters if needed

---

## LokiReadPathHighLoad

**Severity:** `warning` | **For:** 15m | **Component:** Loki Querier / Query Frontend

**Summary:**
The Loki read path is under high load: the 99th percentile LogQL query latency exceeds 30 seconds. The query queue is saturated with expensive or concurrent queries.

**Impact:**
Dashboard panels loading logs, log-based alerting rules, and direct API consumers will all experience degraded or timed-out responses. Our configured query timeout is 5 minutes — queries close to this threshold will be cancelled.

**Alert Expression:**
```promql
histogram_quantile(
  0.99,
  sum by (job, namespace, le) (rate(loki_logql_querystats_latency_seconds_bucket[5m]))
) > 30
```

**Steps:**

- **Check the query queue depth** — high inflight count confirms querier saturation:
  ```promql
  cortex_query_scheduler_inflight_requests{namespace="rhobs-production"}
  ```

- **Check p99 query latency trend** in Grafana:
  ```promql
  histogram_quantile(
    0.99,
    sum by (namespace, le) (rate(loki_logql_querystats_latency_seconds_bucket{namespace="rhobs-production"}[5m]))
  )
  ```

- **Check querier resource usage** — CPU saturation is the most common cause:
  ```bash
  kubectl top pods -n rhobs-production -l app.kubernetes.io/component=querier
  ```

- **Inspect query-frontend logs** for slow query patterns and time ranges being queried:
  ```bash
  kubectl logs -n rhobs-production -l app.kubernetes.io/component=query-frontend --tail=200 | grep -E "stats|took|latency|timeout"
  ```

- **Check querier logs** for errors processing queries:
  ```bash
  kubectl logs -n rhobs-production -l app.kubernetes.io/component=querier --tail=200 | grep -i "error\|timeout\|cancel"
  ```

- **Scale queriers** if CPU/memory saturation is confirmed — current replicas: 2. Update the `LokiStack` CR:
  ```bash
  # To check current replica count
  kubectl get lokistack observatorium-lokistack -n rhobs-production -o jsonpath='{.spec.template.querier.replicas}'
  ```
  Then submit a MR to increase `spec.template.querier.replicas` in `resources/clusters/production/<cluster>/logs/bundle/03-lokistack-LokiStack.yaml`

- **Check S3 read performance** — slow chunk retrieval from S3 cascades into query latency:
  ```promql
  histogram_quantile(0.99, sum by (operation, le) (
    rate(loki_chunk_store_index_lookup_cache_requests_total{namespace="rhobs-production"}[5m])
  ))
  ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl top/logs` on querier and query-frontend pods in `rhobs-production`
- Grafana access to query `<cluster>-prometheus` datasource
- MR access to the configuration repository to scale queriers

---

## LokiDiscardedSamplesWarning

**Severity:** `warning` | **For:** 15m | **Component:** Loki Distributor

**Summary:**
Loki is permanently discarding log samples (entries) because they fail validation. Unlike rate-limited samples (which may be retried), these failures are for **non-retryable** validation errors — the log data is lost.

**Impact:**
Log entries from affected streams are permanently dropped. No retry will recover them. The alert only fires for non-retryable reasons (excludes `per_stream_rate_limit`, `rate_limited`, `stream_limit` which are handled separately by [LokiTenantRateLimit](#lokitenantratelimit)).

**Alert Expression:**
```promql
sum by (namespace, tenant, reason) (
  irate(
    loki_discarded_samples_total{
      reason!="per_stream_rate_limit",
      reason!="rate_limited",
      reason!="stream_limit"
    }[2m]
  )
) > 0
```

**Steps:**

- **Identify the tenant and reason** from alert labels. The `reason` label indicates the validation failure:

  | Reason | Description | Fix |
  |---|---|---|
  | `line_too_long` | Log line exceeds max line size (currently **2 MB**) | Fix log shipper to truncate, or increase `maxLineSize` |
  | `label_name_too_long` | Label name exceeds max length | Fix log shipper label configuration |
  | `label_value_too_long` | Label value exceeds max length | Fix log shipper or application labels |
  | `max_label_names_per_series` | Too many labels per stream | Reduce cardinality in log shipper config |
  | `out_of_order` | Log timestamp is older than the ingestion window | Fix clock skew or log shipper backfill config |

- **Get a live view of discard rate by tenant and reason** in Grafana:
  ```promql
  sum by (namespace, tenant, reason) (
    irate(loki_discarded_samples_total{namespace="rhobs-production"}[2m])
  )
  ```

- **Inspect distributor logs** for stream-level detail (available since Loki 3.1.0):
  ```bash
  kubectl logs -n rhobs-production -l app.kubernetes.io/component=distributor --tail=200 | grep -E "discard|validation|invalid"
  ```

- **Check current line size limit** (currently 2097152 bytes / 2 MB):
  ```bash
  kubectl get lokistack observatorium-lokistack -n rhobs-production -o jsonpath='{.spec.limits.global.ingestion.maxLineSize}'
  ```

- **Decide on remediation**:
  - **Preferred**: Fix the emitting application or log shipper (e.g., truncate long lines, normalize labels)
  - **If adjustment is justified**: Update the relevant limit in the `LokiStack` CR and submit a MR to: `https://gitlab.cee.redhat.com/rhobs/configuration/-/tree/main/resources/clusters/production/<cluster>/logs/bundle`
  - **Contact the tenant team** if the source is a specific application workload

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs` on distributor pods in `rhobs-production`
- Grafana access to query `<cluster>-prometheus` datasource
- MR access to the configuration repository if limit adjustment is required

---

## LokiIngesterFlushFailureRateCritical

**Severity:** `critical` | **For:** 15m | **Component:** Loki Ingester

**Summary:**
One or more Loki ingesters are failing to flush >20% of their chunks to S3 backend storage. Chunks that cannot be flushed accumulate in the WAL on disk, which has finite capacity. This is a **data loss risk** requiring immediate intervention.

**Impact:**
In-memory and WAL-buffered chunks are not being persisted to S3. If the WAL fills up, new ingestion will be rejected. If an ingester restarts with a corrupted WAL, buffered data may be permanently lost.

**Alert Expression:**
```promql
sum by (namespace, pod) (
  rate(loki_ingester_chunks_flush_failures_total[5m])
  /
  rate(loki_ingester_chunks_flush_requests_total[5m])
) > 0.2
```

**Steps:**

> **Immediate priority:** Identify whether the failure is authentication, connectivity, or capacity — each has a different fix path. Do not restart pods before understanding the root cause, as a restart under WAL pressure can trigger data loss.

- **Check the flush failure rate per pod** in Grafana (datasource: `<cluster>-prometheus`):
  ```promql
  sum by (pod, namespace) (
    rate(loki_ingester_chunks_flush_failures_total{namespace="rhobs-production"}[5m])
    /
    rate(loki_ingester_chunks_flush_requests_total{namespace="rhobs-production"}[5m])
  )
  ```

- **Check WAL disk pressure** — if WAL is nearly full, action is more urgent:
  ```promql
  sum by (pod, namespace) (loki_ingester_wal_bytes_in_use{namespace="rhobs-production"})
  ```
  ```promql
  sum by (pod, namespace) (rate(loki_ingester_wal_disk_full_failures_total{namespace="rhobs-production"}[5m]))
  ```

- **Inspect ingester pod logs** to identify the flush error:
  ```bash
  kubectl logs -n rhobs-production <ingester-pod> --tail=200 | grep -E "flush|error|storage|s3"
  ```

- **Verify the storage secret is valid and mounted**:
  ```bash
  kubectl get secret loki-default-bucket -n rhobs-production
  kubectl describe secret loki-default-bucket -n rhobs-production
  ```

- **Check LokiStack CR conditions** for storage-related errors:
  ```bash
  kubectl describe lokistack observatorium-lokistack -n rhobs-production
  ```

- **Root cause and resolution matrix:**

  | Root cause | Indicators in logs | Resolution |
  |---|---|---|
  | **S3 auth failure** | `AccessDenied`, `401`, `403` | Rotate or recreate `loki-default-bucket` secret |
  | **S3 connectivity** | `connection refused`, `timeout`, `no route` | Check NetworkPolicies, security groups, VPC endpoints |
  | **S3 bucket full / quota** | `BucketFull`, `QuotaExceeded` | Increase storage quota or apply lifecycle policies |
  | **Invalid S3 config** | `NoSuchBucket`, `InvalidBucketName` | Correct `spec.storage.secret` in the LokiStack CR |

- **For authentication issues** — update the S3 credentials secret. The LokiStack operator will detect the change:
  ```bash
  # Verify the new credentials are correct before applying
  kubectl create secret generic loki-default-bucket \
    --from-literal=endpoint=<s3-endpoint> \
    --from-literal=bucketnames=<bucket> \
    --from-literal=access_key_id=<key-id> \
    --from-literal=access_key_secret=<key-secret> \
    -n rhobs-production --dry-run=client -o yaml | kubectl apply -f -
  ```

- **Verify recovery** — flush failure rate should drop to 0 within a few minutes:
  ```promql
  sum by (pod, namespace) (
    rate(loki_ingester_chunks_flush_failures_total{namespace="rhobs-production"}[5m])
    /
    rate(loki_ingester_chunks_flush_requests_total{namespace="rhobs-production"}[5m])
  )
  ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs/get/describe` on pods and secrets in `rhobs-production`
- Grafana access to track flush failure and WAL metrics
- AWS/S3 console or CLI access to verify bucket health and credentials
- **This is a critical alert — escalate immediately if WAL disk is filling**

---

## LokistackComponentsNotReadyWarning

**Severity:** `warning` | **For:** 15m | **Component:** LokiStack Operator

**Summary:**
The LokiStack CR `observatorium-lokistack` reports that one or more of its managed components have not reached the `Ready` state. The operator tracks component health via `lokistack_status_condition`.

**Impact:**
Depending on which component is unavailable, ingestion or querying may be degraded or completely unavailable. A not-ready `distributor` blocks all writes; a not-ready `querier` blocks all queries.

**Alert Expression:**
```promql
sum by (stack_name, namespace) (
  label_replace(
    lokistack_status_condition{reason="ReadyComponents",status="false"},
    "namespace",
    "$1",
    "stack_namespace",
    "(.+)"
  )
) > 0
```

**Steps:**

- **Inspect the LokiStack CR status conditions** — this is the primary source of truth:
  ```bash
  kubectl describe lokistack observatorium-lokistack -n rhobs-production
  ```
  Look for `status.conditions` entries with `status: "False"` and read the `reason` and `message` fields.

- **Check pod readiness for all LokiStack components**:
  ```bash
  kubectl get pods -n rhobs-production -l app.kubernetes.io/part-of=lokistack
  ```
  Components to verify: `distributor`, `ingester`, `querier`, `query-frontend`, `index-gateway`, `compactor`, `gateway`

- **For any non-Ready pod**, describe it to find the root cause:
  ```bash
  kubectl describe pod <failing-pod> -n rhobs-production
  ```
  Common causes to look for in `Events` section:
  - `OOMKilled` → resource limits too low
  - `CrashLoopBackOff` → application error (check logs)
  - `ImagePullBackOff` → image registry issue
  - `Pending` → insufficient cluster resources or PVC binding failure

- **Check pod logs** for application-level errors:
  ```bash
  kubectl logs -n rhobs-production <failing-pod> --previous
  kubectl logs -n rhobs-production <failing-pod>
  ```

- **Check Kubernetes events** for recent cluster-level issues:
  ```bash
  kubectl get events -n rhobs-production --sort-by='.lastTimestamp' | tail -30
  ```

- **Check the Loki Operator controller logs** for reconciliation errors:
  ```bash
  kubectl logs -n rhobs-production -l app.kubernetes.io/name=loki-operator --tail=100
  ```

- **Validate storage secret exists and is correct** — missing secret blocks operator reconciliation:
  ```bash
  kubectl get secret loki-default-bucket -n rhobs-production
  ```

- **Check node resource availability** — insufficient CPU/memory causes pod scheduling failures:
  ```bash
  kubectl describe nodes | grep -A 5 "Allocated resources"
  ```

- **Verify PVCs are bound** — ingesters use persistent storage for WAL:
  ```bash
  kubectl get pvc -n rhobs-production -l app.kubernetes.io/part-of=lokistack
  ```

- **Review the configuration** for the cluster: `https://gitlab.cee.redhat.com/rhobs/configuration/-/tree/main/resources/clusters/production/<cluster>/logs/bundle`

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl get/describe/logs` on pods, events, PVCs, and the `LokiStack` CR in `rhobs-production`
- `kubectl logs` on the Loki Operator deployment
- Grafana access to correlate with ingestion/query metrics during the outage window
