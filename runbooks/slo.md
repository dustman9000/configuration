# RHOBS SLO Alerts Runbook

## Table of Contents

### SLO Metric Absence
- [SLOMetricAbsent](#slometricabsent)

### Metrics Write Path
- [APIMetricsWriteAvailabilityErrorBudgetBurning](#apimetricswriteavailabilityerrorbudgetburning)
- [APIMetricsWriteLatencyErrorBudgetBurning](#apimetricswritelatencyerrorbudgetburning)

### Metrics Query Path
- [APIMetricsQueryAvailabilityErrorBudgetBurning](#apimetricsqueryavailabilityerrorbudgetburning)
- [APIMetricsQueryRangeAvailabilityErrorBudgetBurning](#apimetricsqueryrangeavailabilityerrorbudgetburning)

### Alerting Path
- [APIAlertmanagerAvailabilityErrorBudgetBurning](#apialertmanageravailabilityerrorbudgetburning)
- [APIAlertmanagerNotificationsAvailabilityErrorBudgetBurning](#apialertmanagernotificationsavailabilityerrorbudgetburning)

### Logs Write Path
- [APILogsWriteAvailabilityErrorBudgetBurning](#apilogswriteavailabilityerrorbudgetburning)

### Logs Query Path
- [APILogsQueryAvailabilityErrorBudgetBurning](#apilogsqueryavailabilityerrorbudgetburning)

---

## Understanding SLO Alerts

**What are SLOs?**
Service Level Objectives (SLOs) define the target reliability for a service. These alerts fire when error budgets are being consumed too quickly, indicating that the service may fail to meet its reliability targets.

**Error Budget Burning:**
- **Fast burn** (2d exhaustion): Critical, requires immediate action
- **Moderate burn** (4d exhaustion): Urgent, investigate soon
- **Slow burn** (2w/4w exhaustion): Warning, trending toward SLO violation

**SLO for this service:**
- Availability SLO: 99.9% (0.1% error budget)
- Latency SLO: 90% of requests < 5 seconds

---

## SLOMetricAbsent

**Severity:** `medium` | **For:** 2m | **Component:** Various (see Affected Handlers)

**Summary:**
Required metrics for SLO calculations are missing. SLO tracking cannot function properly. This alert fires for different handlers depending on which metrics are absent.

**Impact:**
SLO burn rate calculations cannot run for the affected handler. The absence of these metrics means SLO budget violations will not be detected or alerted on until the metrics are restored.

**Affected Handlers:**
- `/receive` (metrics write)
- `/query` (metrics instant query)
- `/query_range` (metrics range query)
- `/otlp` (logs write)
- `/query` and `/query_range` (logs query)
- Alertmanager alert sender
- Alertmanager notifications

**Alert Expression:**
```promql
absent(<metric>{<label-selectors>}) == 1
```
(See the YAML for the specific metric and label selectors per handler — one `SLOMetricAbsent` rule exists per SLO group.)

**Steps:**

- **Identify which handler is affected** from alert labels (`handler`, `group`, `slo` labels)

- **Check if the component is running:**
   ```bash
   # For metrics write/query (Thanos)
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=thanos-receive
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=thanos-query

   # For logs (Loki)
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=rhobs-gateway
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=lokistack

   # For alerting (Thanos Rule, Alertmanager)
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=thanos-ruler
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=alertmanager
   ```

- **Check if the metrics endpoint is accessible:**
   ```bash
   kubectl exec -n rhobs-production <pod> -- wget -O- localhost:<metrics-port>/metrics
   ```

- **Verify Prometheus is scraping the targets:**
   ```bash
   kubectl get servicemonitor -n rhobs-production
   # Access Prometheus UI and check /targets
   ```

- **Check for recent deployments or configuration changes**

- **Verify service and endpoints:**
   ```bash
   kubectl get svc,endpoints -n rhobs-production | grep <component>
   ```

- **Review component logs for startup or metric export errors:**
   ```bash
   kubectl logs -n rhobs-production <pod> | grep -i "metric\|export"
   ```

- **If pod is running but metrics are missing:**
  - Check metric registration in code
  - Verify metric name matches expected pattern
  - Check for labeling issues

**Access Required:**
- Cluster access via sshuttle (see the relevant component runbook's [Environment & Access Information](thanos.md#environment--access-information))
- Prometheus access
- ServiceMonitor configuration access
- Component logs access

---

## APIMetricsWriteAvailabilityErrorBudgetBurning

**Severity:** `critical` (2d/4d exhaustion) / `warning` (2w/4w exhaustion) | **Component:** Thanos Receive (rhobs-gateway)

**Summary:**
The `/receive` handler (metrics ingestion API) is burning too much error budget. More than 0.1% of write requests are failing with 5xx errors.

**Impact:**
Metrics ingestion is failing for tenants. Sustained failures will exhaust the availability error budget and breach the 99.9% SLO. If left unresolved, historical metric data will be lost for affected tenants.

**Burn Rate Windows:**
| Severity | Short window | Long window | Fires after |
|---|---|---|---|
| `critical` | 5m | 1h | 2 minutes |
| `critical` | 30m | 6h | 15 minutes |
| `warning` | 2h | 1d | 1 hour |
| `warning` | 6h | 4d | 3 hours |

**Alert Expression:**
```promql
http_requests:burnrate5m{group="metricsv1",handler="receive",job="rhobs-gateway",slo="api-metrics-write-availability-slo"}
  > (14 * (1-0.999)) and
http_requests:burnrate1h{group="metricsv1",handler="receive",job="rhobs-gateway",slo="api-metrics-write-availability-slo"}
  > (14 * (1-0.999))
```

**Steps:**

- **Check for related alerts firing** — these component-level alerts are the most likely root cause:
  - [`ThanosReceiveHttpRequestErrorRateHigh`](thanos.md#thanosreceivehttprequesterrorratehigh) — high error rate on the receive path
  - [`ThanosReceiveHighReplicationFailures`](thanos.md#thanosreceivehighreplicationfailures) — ingesters failing to replicate
  - [`ThanosReceiveHighForwardRequestFailures`](thanos.md#thanosreceivehighforwardrequestfailures) — router failing to forward to ingesters
  - [`ThanosReceiveRouterIsDown`](thanos.md#thanosreceiverouterisdown) / [`ThanosReceiveIngesterIsDown`](thanos.md#thanosreceiveingesterisdown) — component completely down
  - [`ThanosReceiveNoUpload`](thanos.md#thanosreceivenoupload) — ingesters not flushing to object storage
  - [`ThanosReceiveNoHashringsConfigured`](thanos-operator.md#thanosreceivenohashrings configured) / [`ThanosReceiveHashringNoEndpoints`](thanos-operator.md#thanosreceivehashringnoendpoints) — operator configuration issue
  - [`ThanosStoreBucketHighOperationFailures`](thanos.md#thanosstorebuckethighoperationfailures) — object storage failures affecting ingesters

- **Review the Receive dashboard in Grafana** — check error rate by status code, replication status, ingester queue depth, and object storage write metrics. Dashboard link is in the alert annotations.

- **Check Thanos Receive workload health:**
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/component=thanos-receive-router
   kubectl get pods -n rhobs-production -l app.kubernetes.io/component=thanos-receive-ingester
   ```
   Look for pods in `CrashLoopBackOff`, `Pending`, or `OOMKilled` state.

- **Identify error patterns from logs:**
   ```bash
   kubectl logs -n rhobs-production -l app.kubernetes.io/component=thanos-receive-router --tail=100 | grep -E "level=error|5[0-9]{2}"
   kubectl logs -n rhobs-production -l app.kubernetes.io/component=thanos-receive-ingester --tail=100 | grep -E "level=error|storage|upload"
   ```

- **If hashring or operator issues are suspected**, check the `ThanosReceive` CR status and any `ThanosReceive*` operator alerts — the operator may have failed to reconcile hashring endpoints

- **If object storage is suspect**, check the Store component: look for `ThanosStoreBucketHighOperationFailures` and verify S3/GCS credentials and bucket accessibility

- **Escalate** to the component runbooks above based on which related alert is firing — remediation should be driven by the root cause alert, not this SLO alert

**Access Required:**
- Cluster access via sshuttle — see the relevant component runbook's Environment section: [Thanos](thanos.md#environment--access-information) | [Loki](loki.md#environment--access-information) | [Alertmanager](alertmanager.md#environment--access-information)
- Grafana dashboard access
- Object storage admin access (if storage is suspect)

---

## APIMetricsWriteLatencyErrorBudgetBurning

**Severity:** `critical` (2d/4d exhaustion) / `warning` (2w/4w exhaustion) | **Component:** Thanos Receive (rhobs-gateway)

**Summary:**
The `/receive` handler latency is burning too much error budget. More than 10% of successful requests are taking longer than 5 seconds.

**Impact:**
Write requests are completing successfully but slowly, degrading tenant experience. Sustained high latency will exhaust the latency error budget and breach the 90% within 5s SLO.

**Burn Rate Windows:**
| Severity | Short window | Long window | Fires after |
|---|---|---|---|
| `critical` | 5m | 1h | 2 minutes |
| `critical` | 30m | 6h | 15 minutes |
| `warning` | 2h | 1d | 1 hour |
| `warning` | 6h | 4d | 3 hours |

**Alert Expression:**
```promql
http_request_duration_seconds:burnrate5m{group="metricsv1",handler="receive",job="rhobs-gateway",slo="api-metrics-write-latency-slo"}
  > (14 * (1-0.9)) and
http_request_duration_seconds:burnrate1h{group="metricsv1",handler="receive",job="rhobs-gateway",slo="api-metrics-write-latency-slo"}
  > (14 * (1-0.9))
```

**Steps:**

- **Check for related alerts firing** — these component-level alerts point to the root cause:
  - [`ThanosReceiveHttpRequestLatencyHigh`](thanos.md#thanosreceivehttprequestlatencyhigh) — direct latency alert on the receive path
  - [`ThanosReceiveNoUpload`](thanos.md#thanosreceivenoupload) — ingesters blocked flushing to object storage, causing back-pressure
  - [`ThanosReceiveHighReplicationFailures`](thanos.md#thanosreceivehighreplicationfailures) — replication retries adding latency
  - [`ThanosStoreObjstoreOperationLatencyHigh`](thanos.md#thanosstoreobjstoreoperationlatencyhigh) — slow object storage affecting ingester flush latency
  - [`ThanosReceiveIngesterIsDown`](thanos.md#thanosreceiveingesterisdown) — fewer ingesters means remaining ones are overloaded
  - [`ThanosReceiveNoHashringsConfigured`](thanos-operator.md#thanosreceivenohashrings configured) / [`ThanosReceiveHashringNoEndpoints`](thanos-operator.md#thanosreceivehashringnoendpoints) — routing problems causing retries

- **Review the Receive dashboard in Grafana** — check p99 latency per handler, ingester WAL flush duration, object storage write latency, and replication overhead. Dashboard link is in the alert annotations.

- **Check Thanos Receive workload health:**
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/component=thanos-receive-ingester
   ```
   Look for pods in degraded state or recent restarts (`kubectl describe pod` for events).

- **Check ingester logs for latency signals:**
   ```bash
   kubectl logs -n rhobs-production <ingester-pod> --tail=100 | grep -E "level=warn|level=error|WAL|flush|slow"
   ```

- **If object storage latency is suspected** — look for `ThanosStoreObjstoreOperationLatencyHigh` or `ThanosStoreBucketHighOperationFailures` and check S3/GCS region and bucket health

- **Escalate** to the component runbooks above based on which related alert is firing — remediation should be driven by the root cause alert, not this SLO alert

**Access Required:**
- Cluster access via sshuttle — see the relevant component runbook's Environment section: [Thanos](thanos.md#environment--access-information) | [Loki](loki.md#environment--access-information) | [Alertmanager](alertmanager.md#environment--access-information)
- Grafana dashboard access
- Object storage metrics access (if storage is suspect)

---

## APIMetricsQueryAvailabilityErrorBudgetBurning

**Severity:** `warning` (all burn rates) | **Component:** Thanos Query (rhobs-gateway)

**Summary:**
The `/query` handler (metrics instant queries) is burning too much error budget. More than 0.1% of query requests are failing with 5xx errors.

**Impact:**
Metrics instant queries are failing for tenants. Sustained failures will exhaust the availability error budget and breach the 99.9% SLO for the query path.

**Burn Rate Windows:**
| Severity | Short window | Long window | Fires after |
|---|---|---|---|
| `warning` | 5m | 1h | 2 minutes |
| `warning` | 30m | 6h | 15 minutes |
| `warning` | 2h | 1d | 1 hour |
| `warning` | 6h | 4d | 3 hours |

**Alert Expression:**
```promql
http_requests:burnrate5m{group="metricsv1",handler="query",job="rhobs-gateway",slo="api-metrics-query-availability-slo"}
  > (14 * (1-0.999)) and
http_requests:burnrate1h{group="metricsv1",handler="query",job="rhobs-gateway",slo="api-metrics-query-availability-slo"}
  > (14 * (1-0.999))
```

**Steps:**

- **Check for related alerts firing** — these component-level alerts are the most likely root cause:
  - [`ThanosQueryHttpRequestQueryErrorRateHigh`](thanos.md#thanosqueryhttprequestqueryerrorratehigh) — direct error rate alert on the query path
  - [`ThanosQueryGrpcServerErrorRate`](thanos.md#thanosquerygrpcservererrorrate) / [`ThanosQueryGrpcClientErrorRate`](thanos.md#thanosquerygrpcclienterrorrate) — gRPC errors to/from store endpoints
  - [`ThanosQueryHighDNSFailures`](thanos.md#thanosqueryhighdnsfailures) — store endpoint discovery failing
  - [`ThanosQueryIsDown`](thanos.md#thanosqueryisdown) — Query component completely unavailable
  - [`ThanosQueryNoEndpointsConfigured`](thanos-operator.md#thanosquerynoendpointsconfigured) — operator has not configured any store endpoints
  - [`ThanosQueryServiceWatchReconcileStorm`](thanos-operator.md#thanosqueryservicewatchreconcilestorm) — operator reconciliation loop thrashing
  - [`ThanosStoreGrpcErrorRate`](thanos.md#thanosstoregrpcerrorrate) — store gateway returning errors to query
  - [`ThanosStoreBucketHighOperationFailures`](thanos.md#thanosstorebuckethighoperationfailures) — object storage failures causing query failures
  - [`ThanosStoreIsDown`](thanos.md#thanosstoreisdown) — store gateway completely unavailable

- **Review the Query dashboard in Grafana** — check error rate by status code, gRPC error rates by store endpoint, store endpoint health, and DNS resolution status. Dashboard link is in the alert annotations.

- **Check Thanos Query workload health:**
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=thanos-query
   ```
   Look for pods in degraded state or recent restarts.

- **Check Query logs for error patterns:**
   ```bash
   kubectl logs -n rhobs-production -l app.kubernetes.io/component=query --tail=100 | grep -E "level=error|store|endpoint"
   ```

- **Check Store Gateway health** if gRPC errors are to a store endpoint:
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=thanos-store
   ```

- **Escalate** to the component runbooks above based on which related alert is firing — remediation should be driven by the root cause alert, not this SLO alert

**Access Required:**
- Cluster access via sshuttle — see the relevant component runbook's Environment section: [Thanos](thanos.md#environment--access-information) | [Loki](loki.md#environment--access-information) | [Alertmanager](alertmanager.md#environment--access-information)
- Grafana dashboard access

---

## APIMetricsQueryRangeAvailabilityErrorBudgetBurning

**Severity:** `warning` (all burn rates) | **Component:** Thanos Query (rhobs-gateway)

**Summary:**
The `/query_range` handler (metrics range queries) is burning too much error budget. More than 0.1% of range query requests are failing.

**Impact:**
Metrics range queries are failing for tenants. Sustained failures will exhaust the availability error budget and breach the 99.9% SLO for the range query path. Dashboards and alerts relying on range queries will stop working.

**Burn Rate Windows:**
| Severity | Short window | Long window | Fires after |
|---|---|---|---|
| `warning` | 5m | 1h | 2 minutes |
| `warning` | 30m | 6h | 15 minutes |
| `warning` | 2h | 1d | 1 hour |
| `warning` | 6h | 4d | 3 hours |

**Alert Expression:**
```promql
http_requests:burnrate5m{group="metricsv1",handler="query_range",job="rhobs-gateway",slo="api-metrics-query-range-availability-slo"}
  > (14 * (1-0.999)) and
http_requests:burnrate1h{group="metricsv1",handler="query_range",job="rhobs-gateway",slo="api-metrics-query-range-availability-slo"}
  > (14 * (1-0.999))
```

**Steps:**

- **Check for related alerts firing** — same component-level alerts as for instant queries:
  - [`ThanosQueryHttpRequestQueryErrorRateHigh`](thanos.md#thanosqueryhttprequestqueryerrorratehigh) — direct error rate alert on the query path
  - [`ThanosQueryGrpcServerErrorRate`](thanos.md#thanosquerygrpcservererrorrate) / [`ThanosQueryGrpcClientErrorRate`](thanos.md#thanosquerygrpcclienterrorrate) — gRPC errors to/from store endpoints
  - [`ThanosQueryInstantLatencyHigh`](thanos.md#thanosqueryinstantlatencyhigh) — high latency causing range query timeouts
  - [`ThanosQueryHighDNSFailures`](thanos.md#thanosqueryhighdnsfailures) — store endpoint discovery failing
  - [`ThanosQueryIsDown`](thanos.md#thanosqueryisdown) — Query component completely unavailable
  - [`ThanosQueryNoEndpointsConfigured`](thanos-operator.md#thanosquerynoendpointsconfigured) — operator has not configured any store endpoints
  - [`ThanosStoreGrpcErrorRate`](thanos.md#thanosstoregrpcerrorrate) — store gateway returning errors to query
  - [`ThanosStoreBucketHighOperationFailures`](thanos.md#thanosstorebuckethighoperationfailures) — object storage failures causing query failures

- **Review the Query dashboard in Grafana** — focus on `/query_range` handler specifically, check gRPC error rates, store endpoint health, and query duration histograms. Dashboard link is in the alert annotations.

- **Check Thanos Query workload health:**
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=thanos-query
   ```

- **Check Query logs for range-query-specific errors:**
   ```bash
   kubectl logs -n rhobs-production -l app.kubernetes.io/component=query --tail=100 | grep -E "level=error|query_range|timeout"
   ```

- **Range queries are more sensitive to store latency** — if errors are timeouts rather than hard failures, check `ThanosQueryInstantLatencyHigh` and `ThanosStoreObjstoreOperationLatencyHigh`

- **Escalate** to the component runbooks above based on which related alert is firing — remediation should be driven by the root cause alert, not this SLO alert

**Access Required:**
- Cluster access via sshuttle — see the relevant component runbook's Environment section: [Thanos](thanos.md#environment--access-information) | [Loki](loki.md#environment--access-information) | [Alertmanager](alertmanager.md#environment--access-information)
- Grafana dashboard access

---

## APIAlertmanagerAvailabilityErrorBudgetBurning

**Severity:** `warning` (all burn rates) | **Component:** Thanos Rule (thanos-ruler)

**Summary:**
Thanos Rule is failing to send alerts to Alertmanager, burning error budget. More than 0.1% of alerts are being dropped when sending to Alertmanager.

**Impact:**
The alert pipeline is degraded. Alerts fired by Thanos Rule are not reaching Alertmanager, meaning on-call notifications may not be delivered for active incidents.

**Burn Rate Windows:**
| Severity | Short window | Long window | Fires after |
|---|---|---|---|
| `warning` | 5m | 1h | 2 minutes |
| `warning` | 30m | 6h | 15 minutes |
| `warning` | 2h | 1d | 1 hour |
| `warning` | 6h | 4d | 3 hours |

**Alert Expression:**
```promql
thanos_alert_sender_alerts_dropped:burnrate5m{container="thanos-ruler",slo="api-alerting-availability-slo"}
  > (14 * (1-0.999)) and
thanos_alert_sender_alerts_dropped:burnrate1h{container="thanos-ruler",slo="api-alerting-availability-slo"}
  > (14 * (1-0.999))
```

**Steps:**

- **Alert pipeline is degraded** — alerts fired by Thanos Rule may not be reaching Alertmanager

- **Check for related alerts firing** — these component-level alerts identify the root cause:
  - [`ThanosRuleSenderIsFailingAlerts`](thanos.md#thanosrulesenderisfailingalerts) — direct alert for the rule → alertmanager send path failing
  - [`ThanosRuleQueueIsDroppingAlerts`](thanos.md#thanosrulequeueisdroppingalerts) — alert queue full, alerts being dropped
  - [`ThanosRuleAlertmanagerHighDNSFailures`](thanos.md#thanosrulealertmanagerhighdnsfailures) — Thanos Rule cannot resolve Alertmanager address
  - [`ThanosRuleIsDown`](thanos.md#thanosruleisdown) — Thanos Rule component completely down
  - [`ThanosRulerNoQueryEndpointsConfigured`](thanos-operator.md#thanosrulernoquery endpointsconfigured) — operator misconfiguration causing rule failure
  - [`ThanosRulerWatchReconcileStorm`](thanos-operator.md#thanosrulerwatchreconcilestorm) — operator reconciliation thrashing
  - [`AlertmanagerClusterDown`](alertmanager.md#alertmanagerclusterdown) — Alertmanager itself is down, so Rule cannot send to it

- **Review the Thanos Ruler dashboard in Grafana** — check alert sender error rate, queue depth, and Alertmanager connectivity metrics. Dashboard link is in the alert annotations.

- **Check Thanos Rule workload health:**
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=thanos-ruler
   ```

- **Check Rule logs for send failures:**
   ```bash
   kubectl logs -n rhobs-production -l app.kubernetes.io/component=rule --tail=100 | grep -E "level=error|alertmanager|send"
   ```

- **Check Alertmanager health** — if `AlertmanagerClusterDown` is also firing, resolve that first as it is the upstream cause

- **Verify the ThanosRuler CR has correct Alertmanager URL configured:**
   ```bash
   kubectl get thanosruler.monitoring.thanos.io -n rhobs-production -o yaml | grep alertmanager
   ```

- **Escalate** to the component runbooks above based on which related alert is firing — remediation should be driven by the root cause alert, not this SLO alert

**Access Required:**
- Cluster access via sshuttle — see the relevant component runbook's Environment section: [Thanos](thanos.md#environment--access-information) | [Loki](loki.md#environment--access-information) | [Alertmanager](alertmanager.md#environment--access-information)
- Grafana dashboard access

---

## APIAlertmanagerNotificationsAvailabilityErrorBudgetBurning

**Severity:** `warning` (all burn rates) | **Component:** Alertmanager

**Summary:**
Alertmanager is failing to deliver alerts to upstream targets (Slack, PagerDuty, etc.), burning error budget. More than 0.1% of notifications are failing.

**Impact:**
On-call notifications are not reaching their destinations. Active alerts may not trigger the expected Slack messages, PagerDuty incidents, or other integrations, silently dropping notifications.

**Burn Rate Windows:**
| Severity | Short window | Long window | Fires after |
|---|---|---|---|
| `warning` | 5m | 1h | 2 minutes |
| `warning` | 30m | 6h | 15 minutes |
| `warning` | 2h | 1d | 1 hour |
| `warning` | 6h | 4d | 3 hours |

**Alert Expression:**
```promql
alertmanager_notifications_failed:burnrate5m{job="alertmanager",slo="api-alerting-notif-availability-slo"}
  > (14 * (1-0.999)) and
alertmanager_notifications_failed:burnrate1h{job="alertmanager",slo="api-alerting-notif-availability-slo"}
  > (14 * (1-0.999))
```

**Steps:**

- **Notifications may not be delivered** — on-call engineers may not receive pages for active incidents

- **Check for related alerts firing** — these component-level alerts identify the root cause:
  - [`AlertmanagerFailedToSendAlerts`](alertmanager.md#alertmanagerfailedtosend alerts) — Alertmanager failing to send to a specific integration
  - [`AlertmanagerClusterFailedToSendAlerts`](alertmanager.md#alertmanagerclusterfailedtosend alerts) — cluster-wide send failure
  - [`AlertmanagerClusterDown`](alertmanager.md#alertmanagerclusterdown) — Alertmanager cluster is not quorate
  - [`AlertmanagerClusterCrashlooping`](alertmanager.md#alertmanagerclustercrashlooping) — Alertmanager pods are crash-looping

- **Review the Alertmanager dashboard in Grafana** — check notification failure rate by integration, check which integrations are failing. Dashboard link is in the alert annotations.

- **Identify which integration is failing:**
   ```bash
   kubectl exec -n rhobs-production <alertmanager-pod> -- wget -O- localhost:9093/metrics | grep notifications_failed
   ```

- **Check Alertmanager logs for the failing integration:**
   ```bash
   kubectl logs -n rhobs-production -l app.kubernetes.io/name=alertmanager --tail=100 | grep -E "level=error|notification|integration"
   ```

- **Common causes per integration:**
  - **Slack**: Webhook URL revoked or rate limited (1 msg/s limit)
  - **PagerDuty**: API key expired or service suspended
  - **Email**: SMTP credentials expired or relay rejected
  - **Webhook**: Upstream endpoint down or timing out

- **Check the external service status page** for the failing integration (Slack status, PagerDuty status, etc.)

- **If credentials have expired**, update the Alertmanager config secret and reload — follow the [`AlertmanagerFailedToSendAlerts` runbook](alertmanager.md#alertmanagerfailedtosend alerts) for the specific remediation steps

- **Post-incident**: Verify all critical alerts were eventually delivered via the integration's delivery logs

**Access Required:**
- Cluster access via sshuttle — see the relevant component runbook's Environment section: [Thanos](thanos.md#environment--access-information) | [Loki](loki.md#environment--access-information) | [Alertmanager](alertmanager.md#environment--access-information)
- Grafana dashboard access
- Integration service admin access (Slack, PagerDuty, etc.)
- Alertmanager configuration / secrets access

---

## APILogsWriteAvailabilityErrorBudgetBurning

**Severity:** `critical` (2d/4d exhaustion) / `warning` (2w/4w exhaustion) | **Component:** Loki Gateway (rhobs-gateway)

**Summary:**
Loki OTLP ingestion API is burning too much error budget. More than 0.1% of log ingestion requests are failing with 5xx errors.

**Impact:**
Log ingestion is failing for tenants. Sustained failures will exhaust the availability error budget and breach the 99.9% SLO. Log data will be lost for affected tenants during the outage period.

**Burn Rate Windows:**
| Severity | Short window | Long window | Fires after |
|---|---|---|---|
| `critical` | 5m | 1h | 2 minutes |
| `critical` | 30m | 6h | 15 minutes |
| `warning` | 2h | 1d | 1 hour |
| `warning` | 6h | 4d | 3 hours |

**Alert Expression:**
```promql
http_requests:burnrate5m{group="logsv1",handler="otlp",job="rhobs-gateway",slo="api-logs-write-availability-slo"}
  > (14 * (1-0.999)) and
http_requests:burnrate1h{group="logsv1",handler="otlp",job="rhobs-gateway",slo="api-logs-write-availability-slo"}
  > (14 * (1-0.999))
```

**Steps:**

- **Check for related alerts firing** — these component-level alerts are the most likely root cause:
  - [`LokiRequestErrors`](loki.md#lokirequesterrors) — high 5xx rate on Loki write path (distributor/ingester)
  - [`LokiWritePathHighLoad`](loki.md#lokiwritepathhighload) — write path under high load, causing failures
  - [`LokiIngesterFlushFailureRateCritical`](loki.md#lokiingesterflushfailureratecritical) — ingesters failing to flush to object storage
  - [`LokiDiscardedSamplesWarning`](loki.md#lokidiscardedsampleswarning) — samples being dropped (rate limiting or schema violations)
  - [`LokiTenantRateLimit`](loki.md#lokitenantratelimit) — a tenant hitting ingestion rate limits
  - [`LokistackComponentsNotReadyWarning`](loki.md#lokistackcomponentsnotreadywarning) — Loki Operator has not reconciled the LokiStack to a ready state

- **Review the LokiStack Writes dashboard in Grafana** — check error rate by status code, ingestion throughput, distributor and ingester health, and object storage write metrics. Dashboard link is in the alert annotations.

- **Check LokiStack component health:**
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=lokistack,app.kubernetes.io/component=distributor
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=lokistack,app.kubernetes.io/component=ingester
   ```
   Look for pods in `CrashLoopBackOff`, `Pending`, or `OOMKilled` state.

- **Check the LokiStack CR status** for any operator-reported degradation:
   ```bash
   kubectl get lokistack observatorium-lokistack -n rhobs-production -o yaml | grep -A 20 status
   ```

- **Check distributor/ingester logs for error patterns:**
   ```bash
   kubectl logs -n rhobs-production -l app.kubernetes.io/name=lokistack,app.kubernetes.io/component=distributor --tail=100 | grep -E "level=error|rate.*limit|5[0-9]{2}"
   kubectl logs -n rhobs-production -l app.kubernetes.io/name=lokistack,app.kubernetes.io/component=ingester --tail=100 | grep -E "level=error|storage|flush"
   ```

- **If ingester flush failures are suspected** — check `LokiIngesterFlushFailureRateCritical` and verify S3 bucket credentials and accessibility

- **If rate limiting is causing failures** — check `LokiTenantRateLimit` and `LokiDiscardedSamplesWarning` to identify the offending tenant

- **Escalate** to the component runbooks above based on which related alert is firing — the LokiStack is managed by the Loki Operator; replica counts and resource limits are set in the `LokiStack` CR, not manually scaled

**Access Required:**
- Cluster access via sshuttle — see the relevant component runbook's Environment section: [Thanos](thanos.md#environment--access-information) | [Loki](loki.md#environment--access-information) | [Alertmanager](alertmanager.md#environment--access-information)
- Grafana dashboard access
- Object storage access (if flush failures are suspected)

---

## APILogsQueryAvailabilityErrorBudgetBurning

**Severity:** `warning` (all burn rates) | **Component:** Loki Gateway (rhobs-gateway)

**Summary:**
Loki query handlers (`/query` and `/query_range`) are burning too much error budget. More than 0.1% of log query requests are failing with 5xx errors.

**Impact:**
Log queries are failing for tenants. Sustained failures will exhaust the availability error budget and breach the 99.9% SLO for the log query path. Log-based dashboards and investigations will be impacted.

**Burn Rate Windows:**
| Severity | Short window | Long window | Fires after |
|---|---|---|---|
| `warning` | 5m | 1h | 2 minutes |
| `warning` | 30m | 6h | 15 minutes |
| `warning` | 2h | 1d | 1 hour |
| `warning` | 6h | 4d | 3 hours |

**Alert Expression:**
```promql
http_requests:burnrate5m{group="logsv1",handler=~"query(_range)?",job="rhobs-gateway",slo="api-logs-query-availability-slo"}
  > (14 * (1-0.999)) and
http_requests:burnrate1h{group="logsv1",handler=~"query(_range)?",job="rhobs-gateway",slo="api-logs-query-availability-slo"}
  > (14 * (1-0.999))
```

**Steps:**

- **Check for related alerts firing** — these component-level alerts identify the root cause:
  - [`LokiRequestErrors`](loki.md#lokirequesterrors) — high 5xx rate on Loki read path (querier/query-frontend)
  - [`LokiReadPathHighLoad`](loki.md#lokireadpathhighload) — query path under high load, causing timeouts or failures
  - [`LokiRequestLatency`](loki.md#lokirequestlatency) — high latency causing query timeouts manifesting as errors
  - [`LokiRequestPanics`](loki.md#lokirequestpanics) — querier or query-frontend panicking on certain queries
  - [`LokistackComponentsNotReadyWarning`](loki.md#lokistackcomponentsnotreadywarning) — Loki Operator has not reconciled the LokiStack to a ready state

- **Review the LokiStack Reads dashboard in Grafana** — check error rate by handler (`/query` vs `/query_range`), query latency histograms, querier and query-frontend health, and object storage read metrics. Dashboard link is in the alert annotations.

- **Check LokiStack query component health:**
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=lokistack,app.kubernetes.io/component=querier
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=lokistack,app.kubernetes.io/component=query-frontend
   ```
   Look for pods in degraded state or recent restarts.

- **Check the LokiStack CR status** for any operator-reported degradation:
   ```bash
   kubectl get lokistack observatorium-lokistack -n rhobs-production -o yaml | grep -A 20 status
   ```

- **Check querier and query-frontend logs for error patterns:**
   ```bash
   kubectl logs -n rhobs-production -l app.kubernetes.io/name=lokistack,app.kubernetes.io/component=querier --tail=100 | grep -E "level=error|timeout|storage|panic"
   kubectl logs -n rhobs-production -l app.kubernetes.io/name=lokistack,app.kubernetes.io/component=query-frontend --tail=100 | grep -E "level=error|timeout"
   ```

- **If panics are present** — check `LokiRequestPanics` runbook; the query may be hitting a bug or corrupt chunk in object storage

- **If latency-induced failures** — check `LokiRequestLatency` and `LokiReadPathHighLoad`; the issue is object storage read speed or querier resource pressure, not something to scale manually

- **Escalate** to the component runbooks above based on which related alert is firing — the LokiStack is managed by the Loki Operator; resource configuration changes must go through the `LokiStack` CR

**Access Required:**
- Cluster access via sshuttle — see the relevant component runbook's Environment section: [Thanos](thanos.md#environment--access-information) | [Loki](loki.md#environment--access-information) | [Alertmanager](alertmanager.md#environment--access-information)
- Grafana dashboard access
- Object storage access (if storage read errors are suspected)
