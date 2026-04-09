# Thanos Alerts Runbook

## Table of Contents

### Component Availability
- [ThanosCompactIsDown](#thanoscompactisdown)
- [ThanosQueryIsDown](#thanosqueryisdown)
- [ThanosReceiveRouterIsDown](#thanosreceiverouterisdown)
- [ThanosReceiveIngesterIsDown](#thanosreceiveingesterisdown)
- [ThanosRuleIsDown](#thanosruleisdown)
- [ThanosStoreIsDown](#thanosstoreisdown)

### Thanos Compact
- [ThanosCompactMultipleRunning](#thanoscompactmultiplerunning)
- [ThanosCompactHalted](#thanoscompacthalted)
- [ThanosCompactHighCompactionFailures](#thanoscompacthighcompactionfailures)
- [ThanosCompactBucketHighOperationFailures](#thanoscompactbuckethighoperationfailures)
- [ThanosCompactHasNotRun](#thanoscompacthasnotrun)

### Thanos Query
- [ThanosQueryHttpRequestQueryErrorRateHigh](#thanosqueryhttprequestqueryerrorratehigh)
- [ThanosQueryGrpcServerErrorRate](#thanosquerygrpcservererrorrate)
- [ThanosQueryGrpcClientErrorRate](#thanosquerygrpcclienterrorrate)
- [ThanosQueryHighDNSFailures](#thanosqueryhighdnsfailures)
- [ThanosQueryInstantLatencyHigh](#thanosqueryinstantlatencyhigh)

### Thanos Receive
- [ThanosReceiveHttpRequestErrorRateHigh](#thanosreceivehttprequesterrorratehigh)
- [ThanosReceiveHttpRequestLatencyHigh](#thanosreceivehttprequestlatencyhigh)
- [ThanosReceiveHighReplicationFailures](#thanosreceivehighreplicationfailures)
- [ThanosReceiveHighForwardRequestFailures](#thanosreceivehighforwardrequestfailures)
- [ThanosReceiveHighHashringFileRefreshFailures](#thanosreceivehighhashringfilerefreshfailures)
- [ThanosReceiveConfigReloadFailure](#thanosreceiveconfigreloadfailure)
- [ThanosReceiveNoUpload](#thanosreceivenoupload)
- [ThanosReceiveLimitsConfigReloadFailure](#thanosreceivelimitsconfigreloadfailure)
- [ThanosReceiveLimitsHighMetaMonitoringQueriesFailureRate](#thanosreceivelimitshighmetamonitoringqueriesfailurerate)
- [ThanosReceiveTenantLimitedByHeadSeries](#thanosreceivetenantlimitedbyheadseries)

### Thanos Store
- [ThanosStoreGrpcErrorRate](#thanosstoregrpcerrorrate)
- [ThanosStoreBucketHighOperationFailures](#thanosstorebuckethighoperationfailures)
- [ThanosStoreObjstoreOperationLatencyHigh](#thanosstoreobjstoreoperationlatencyhigh)

### Thanos Rule
- [ThanosRuleQueueIsDroppingAlerts](#thanosrulequeueisdroppingalerts)
- [ThanosRuleSenderIsFailingAlerts](#thanosrulesenderisfailingalerts)
- [ThanosRuleHighRuleEvaluationFailures](#thanosrulehighruleevaluationfailures)
- [ThanosRuleHighRuleEvaluationWarnings](#thanosrulehighruleevaluationwarnings)
- [ThanosRuleRuleEvaluationLatencyHigh](#thanosruleruleevaluationlatencyhigh)
- [ThanosRuleGrpcErrorRate](#thanosrulegrpcerrorrate)
- [ThanosRuleConfigReloadFailure](#thanosruleconfigreloadfailure)
- [ThanosRuleQueryHighDNSFailures](#thanosrulequeryhighdnsfailures)
- [ThanosRuleAlertmanagerHighDNSFailures](#thanosrulealertmanagerhighdnsfailures)
- [ThanosRuleNoEvaluationFor10Intervals](#thanosrulenoevaluationfor10intervals)
- [ThanosNoRuleEvaluations](#thanosnoruleevaluations)

---

## Environment & Access Information

### Multi-Cluster Environment

This operator runs across multiple private Kubernetes clusters. To troubleshoot alerts:

- **Identify the cluster** from the alert labels (typically `cluster` or `prometheus` label)
- **Access Grafana for metrics:**
   - Navigate to Thanos dashboards (linked in alert) or use Explore view for PromQL queries
   - URL: `https://grafana.app-sre.devshift.net/?orgId=1`
   - Select datasource: `<cluster>-prometheus` (e.g., `rhobsp02ue1-prometheus`)
- **Access the cluster (for kubectl commands):**
   - Visit the cluster page: `https://visual-app-interface.devshift.net/clusters/<cluster-name>`
   - Follow the sshuttle access instructions provided on the cluster page
   - For `kubectl` access, navigate to `https://oauth-openshift.apps.<cluster_name>.openshiftapps.com/oauth/token/request` (find the cluster name from the console URL on the cluster page in the visual app interface), copy the `oc login` command shown there, and run it — this grants `kubectl` access to the cluster
- **View Configuration:**
   - Configuration repository: `https://gitlab.cee.redhat.com/rhobs/configuration/-/tree/main/resources/clusters/production/<cluster>/metrics/bundle`
   - Contains ThanosOperator and all Thanos CRs (ThanosQuery, ThanosReceive, ThanosRuler, ThanosStore, ThanosCompact)

### Prometheus Query Access

For all PromQL metric queries in this runbook:
- **Preferred Method:** Use **Grafana** at `https://grafana.app-sre.devshift.net/?orgId=1`
  - Select the `<cluster>-prometheus` datasource (e.g., `rhobsp01ue1-prometheus`)
  - Use the Thanos Operator dashboard for pre-built visualizations
  - Copy-paste PromQL queries from investigative steps for ad-hoc exploration
- **Alternative:** Access Prometheus UI directly via sshuttle tunnel after cluster access
- **Query Placeholders:**
  - Replace `<cluster>` with actual cluster name from alert labels
  - Replace `<resource-name>` with the actual resource name (e.g. `rhobs` for all Thanos CRs)
  - Namespace is `rhobs-production` across all clusters (already filled in throughout this runbook)

---

## Component Availability Alerts

> These alerts fire when a component has completely disappeared from Prometheus service discovery. Start with the component's overview dashboard to confirm absence, then use `kubectl` to find the root cause.
>
> - Compact: [Thanos / Compact / Overview](https://grafana.app-sre.devshift.net/d/thanos-compact-overview/thanos-compact-overview)
> - Query: [Thanos / Query / Overview](https://grafana.app-sre.devshift.net/d/thanos-query-overview/thanos-query-overview)
> - Receive: [Thanos / Receive / Overview](https://grafana.app-sre.devshift.net/d/thanos-receive-overview/thanos-receive-overview)
> - Store: [Thanos / Store Gateway / Overview](https://grafana.app-sre.devshift.net/d/thanos-store-overview/thanos-store-overview)
> - Rule: [Thanos / Ruler / Overview](https://grafana.app-sre.devshift.net/d/thanos-ruler-overview/thanos-ruler-overview)
>
> Select the `<cluster>-prometheus` datasource from the dropdown on each dashboard.

---

## ThanosCompactIsDown

**Severity:** `high` | **For:** 5m | **Component:** Thanos Compact

**Summary:**
The Thanos Compact component has disappeared from Prometheus service discovery. No `up` metric with `job=~"thanos-compact.*"` equals 1 can be found.

**Impact:**
Compaction of object storage blocks has stopped. Without compaction, TSDB blocks accumulate in object storage without being merged, causing: increased query latency as queries must scan more blocks, increased object storage costs, and eventual failure of retention policy enforcement.

**Alert Expression:**
```promql
absent(up{job=~"thanos-compact.*"} == 1)
```

**Steps:**

- **Check pod status** — confirm whether the pod exists and is running:
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=thanos-compact
   ```

- **Check the ThanosCompact CR** managed by the operator:
   ```bash
   kubectl get thanoscompact -n rhobs-production
   kubectl describe thanoscompact <name> -n rhobs-production
   ```

- **If pod exists but is crash-looping**, inspect logs:
   ```bash
   kubectl logs -n rhobs-production <compact-pod> --previous
   kubectl describe pod -n rhobs-production <compact-pod>
   ```

- **Check Service and ServiceMonitor** so Prometheus can scrape the target:
   ```bash
   kubectl get svc,endpoints -n rhobs-production | grep compact
   kubectl get servicemonitor -n rhobs-production | grep compact
   ```

- **In Grafana**, verify the scrape target is present:
   ```promql
   up{job=~"thanos-compact.*", namespace="rhobs-production"}
   ```

- **Check for recent operator events** that may have deleted or misconfigured the component:
   ```bash
   kubectl get events -n rhobs-production --sort-by='.lastTimestamp' | grep -i compact
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl get/describe` on pods, services, endpoints, events in the namespace
- `kubectl get` on `ThanosCompact` CRs
- Grafana access to verify scrape targets

---

## ThanosQueryIsDown

**Severity:** `high` | **For:** 5m | **Component:** Thanos Query

**Summary:**
The Thanos Query component has disappeared from Prometheus service discovery. No `up` metric with `job=~"thanos-query.*"` equals 1 can be found.

**Impact:**
All PromQL queries against historical and real-time data will fail. Dashboards, alerting rules evaluated via Thanos Query, and any application relying on the Thanos query endpoint will be broken.

**Alert Expression:**
```promql
absent(up{job=~"thanos-query.*"} == 1)
```

**Steps:**

- **Check pod status:**
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=thanos-query
   ```

- **Check the ThanosQuery CR:**
   ```bash
   kubectl get thanosquery -n rhobs-production
   kubectl describe thanosquery <name> -n rhobs-production
   ```

- **Inspect crash-looping pod logs:**
   ```bash
   kubectl logs -n rhobs-production <query-pod> --previous
   kubectl describe pod -n rhobs-production <query-pod>
   ```

- **Check the Deployment/ReplicaSet and events:**
   ```bash
   kubectl get deployment -n rhobs-production | grep query
   kubectl get events -n rhobs-production --sort-by='.lastTimestamp' | grep -i query
   ```

- **Verify Service and ServiceMonitor:**
   ```bash
   kubectl get svc,endpoints -n rhobs-production | grep query
   kubectl get servicemonitor -n rhobs-production | grep query
   ```

- **In Grafana**, confirm scrape target is absent:
   ```promql
   up{job=~"thanos-query.*", namespace="rhobs-production"}
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl get/describe` on pods, deployments, services, endpoints, events
- `kubectl get` on `ThanosQuery` CRs
- Grafana access

---

## ThanosReceiveRouterIsDown

**Severity:** `high` | **For:** 5m | **Component:** Thanos Receive Router

**Summary:**
The Thanos Receive Router has disappeared from Prometheus service discovery. No `up` metric with `job=~"thanos-receive-router.*"` equals 1 can be found.

**Impact:**
All incoming remote write requests from Prometheus instances will fail immediately. Metrics ingestion across all tenants is completely broken. This results in data gaps in monitoring.

**Alert Expression:**
```promql
absent(up{job=~"thanos-receive-router.*"} == 1)
```

**Steps:**

- **Check router pod status** (the operator sets `component=thanos-receive-router`):
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/component=thanos-receive-router
   ```

- **Check the ThanosReceive CR** — the operator manages one Deployment for the router per CR:
   ```bash
   kubectl get thanosreceive -n rhobs-production
   kubectl describe thanosreceive <name> -n rhobs-production
   ```

- **Inspect pod logs for startup failures:**
   ```bash
   kubectl logs -n rhobs-production <receive-router-pod> --previous
   kubectl describe pod -n rhobs-production <receive-router-pod>
   ```

- **Check if the operator-generated hashring ConfigMap exists** — the operator builds this automatically from ingester EndpointSlices and mounts it to the router. It is **not** user-managed:
   ```bash
   kubectl get configmap -n rhobs-production thanos-receive-router-<receive-name> -o yaml
   ```
   If it's missing, the operator hasn't reconciled yet. Check operator logs: `kubectl logs -n rhobs-production -l control-plane=controller-manager | grep -i "hashring\|ThanosReceive"`

- **Verify Service and ServiceMonitor:**
   ```bash
   kubectl get svc,endpoints -n rhobs-production | grep receive-router
   ```

- **In Grafana**, check for any remaining router scrape targets:
   ```promql
   up{job=~"thanos-receive-router.*", namespace="rhobs-production"}
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl get/describe` on pods, configmaps, services, endpoints
- `kubectl get` on `ThanosReceive` CRs
- Grafana access

---

## ThanosReceiveIngesterIsDown

**Severity:** `high` | **For:** 5m | **Component:** Thanos Receive Ingester

**Summary:**
The Thanos Receive Ingester has disappeared from Prometheus service discovery. No `up` metric with `job=~"thanos-receive-ingester.*"` equals 1 can be found.

**Impact:**
Data replication from the router to ingesters fails. Depending on the replication factor, this may cause data loss for some tenants. Upload of TSDB blocks to object storage also stops for affected ingesters.

**Alert Expression:**
```promql
absent(up{job=~"thanos-receive-ingester.*"} == 1)
```

**Steps:**

- **Check ingester StatefulSet and pod status** — the operator creates one StatefulSet per hashring, named `<receive-name>-<hashring-name>-ingester`. Component label is `thanos-receive-ingester`:
   ```bash
   kubectl get statefulset -n rhobs-production -l app.kubernetes.io/component=thanos-receive-ingester
   kubectl get pods -n rhobs-production -l app.kubernetes.io/component=thanos-receive-ingester
   ```

- **Check the ThanosReceive CR:**
   ```bash
   kubectl get thanosreceive -n rhobs-production
   kubectl describe thanosreceive <name> -n rhobs-production
   ```

- **Check PVC status** — ingesters use persistent storage for the TSDB WAL:
   ```bash
   kubectl get pvc -n rhobs-production | grep receive-ingester
   ```
   If PVCs are `Pending`, check storage class availability and node disk pressure.

- **Inspect pod logs:**
   ```bash
   kubectl logs -n rhobs-production <receive-ingester-pod> --previous
   kubectl describe pod -n rhobs-production <receive-ingester-pod>
   ```

- **Verify Service and ServiceMonitor:**
   ```bash
   kubectl get svc,endpoints -n rhobs-production | grep receive-ingester
   ```

- **In Grafana**, check scrape target health:
   ```promql
   up{job=~"thanos-receive-ingester.*", namespace="rhobs-production"}
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl get/describe` on pods, StatefulSets, PVCs, services
- `kubectl get` on `ThanosReceive` CRs
- Grafana access

---

## ThanosRuleIsDown

**Severity:** `high` | **For:** 5m | **Component:** Thanos Rule

**Summary:**
The Thanos Rule component has disappeared from Prometheus service discovery. No `up` metric with `job=~"thanos-ruler.*"` equals 1 can be found.

**Impact:**
All recording rules and alerting rules managed by Thanos Rule stop being evaluated. Derived metrics stop being produced and no new alerts will fire from this ruler. This is a critical failure of the alerting pipeline.

**Alert Expression:**
```promql
absent(up{job=~"thanos-ruler.*"} == 1)
```

**Steps:**

- **Check pod and StatefulSet status:**
   ```bash
   kubectl get statefulset -n rhobs-production | grep ruler
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=thanos-ruler
   ```

- **Check the ThanosRuler CR:**
   ```bash
   kubectl get thanosruler.monitoring.thanos.io -n rhobs-production
   kubectl describe thanosruler.monitoring.thanos.io <name> -n rhobs-production
   ```

- **Inspect logs for startup errors** (common: invalid rule files, unreachable query endpoint):
   ```bash
   kubectl logs -n rhobs-production <ruler-pod> --previous
   ```

- **Verify rule ConfigMaps are present:**
   ```bash
   kubectl get configmap -n rhobs-production | grep rule
   ```

- **Check Alertmanager and Query endpoints** are correctly configured in the CR — a misconfigured endpoint can prevent startup.

- **In Grafana**, confirm scrape target absence:
   ```promql
   up{job=~"thanos-ruler.*", namespace="rhobs-production"}
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl get/describe` on pods, StatefulSets, configmaps
- `kubectl get` on `ThanosRuler` CRs
- Grafana access

---

## ThanosStoreIsDown

**Severity:** `high` | **For:** 5m | **Component:** Thanos Store

**Summary:**
The Thanos Store Gateway has disappeared from Prometheus service discovery. No `up` metric with `job=~"thanos-store.*"` equals 1 can be found.

**Impact:**
Queries for historical data (data beyond the Prometheus retention window) will fail or return partial results. Thanos Query will report store endpoints as unhealthy, degrading query coverage.

**Alert Expression:**
```promql
absent(up{job=~"thanos-store.*"} == 1)
```

**Steps:**

- **Check StatefulSet and pod status:**
   ```bash
   kubectl get statefulset -n rhobs-production | grep store
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=thanos-store
   ```

- **Check the ThanosStore CR:**
   ```bash
   kubectl get thanosstore -n rhobs-production
   kubectl describe thanosstore <name> -n rhobs-production
   ```

- **Inspect logs** — common causes: object storage credential errors, bucket access failure on startup:
   ```bash
   kubectl logs -n rhobs-production <store-pod> --previous
   ```

- **Check the object storage secret** referenced in the CR:
   ```bash
   kubectl get secret -n rhobs-production <objstore-secret-name>
   ```

- **Check PVC status** if the store uses a local index cache:
   ```bash
   kubectl get pvc -n rhobs-production | grep store
   ```

- **In Grafana**, confirm scrape target absence:
   ```promql
   up{job=~"thanos-store.*", namespace="rhobs-production"}
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl get/describe` on pods, StatefulSets, PVCs, secrets
- `kubectl get` on `ThanosStore` CRs
- Grafana access

---

## Thanos Compact Alerts

> **Grafana Dashboard:** [Thanos / Compact / Overview](https://grafana.app-sre.devshift.net/d/thanos-compact-overview/thanos-compact-overview) — select the `<cluster>-prometheus` datasource from the dropdown. Panel references in steps below correspond to panels in this dashboard.

---

## ThanosCompactMultipleRunning

**Severity:** `medium` | **For:** 5m | **Component:** Thanos Compact

**Summary:**
More than one Thanos Compact instance is running simultaneously. The alert fires when the sum of `up` for `thanos-compact.*` jobs exceeds 1 in a namespace.

**Impact:**
Running multiple compactors against the same object storage bucket causes block overlap conflicts, data corruption, and compaction failures. Thanos Compact is explicitly designed to be a singleton — it does not support HA mode.

**Alert Expression:**
```promql
sum by (namespace, job) (up{job=~"thanos-compact.*"} > 1)
```

**Steps:**

- **Identify all running compactor pods:**
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=thanos-compact
   ```

- **Check the ThanosCompact CR for the shards configuration** — the operator creates one StatefulSet per shard, named `<compact-name>-shard-<N>`. Multiple pods are **expected and correct** for sharded deployments:
   ```bash
   kubectl get thanoscompact <name> -n rhobs-production -o jsonpath='{.spec.shards}'
   kubectl get statefulset -n rhobs-production -l app.kubernetes.io/name=thanos-compact
   ```
   If `spec.shards > 1`, each shard compacts a distinct set of blocks (by external label). This is intentional and **not** a problem. The alert fires only when `up > 1` aggregated `by (namespace, job)` — in sharded mode, each shard has a distinct job label so this should not fire spuriously.

- **If shards = 1 but multiple pods are running**, there is a duplicate CR or StatefulSet:
   ```bash
   kubectl get thanoscompact -n rhobs-production
   ```
   Remove duplicate CRs. **Do not** scale StatefulSets directly — let the operator manage replicas.

- **In Grafana**, confirm the count of running instances per job:
   ```promql
   sum by (namespace, job) (up{job=~"thanos-compact.*", namespace="rhobs-production"})
   ```

- **Never run two compact instances against the same bucket** — the operator enforces a prune-first strategy when changing shards. If you need to change the shard count, update the CR; the operator will remove old shards before creating new ones.

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl get/patch` on `ThanosCompact` CRs and StatefulSets
- Grafana access

---

## ThanosCompactHalted

**Severity:** `medium` | **For:** 5m | **Component:** Thanos Compact

**Summary:**
The Thanos Compact process has encountered a fatal error and set `thanos_compact_halted` to `1`. It will not attempt any further compactions until restarted.

**Impact:**
Compaction stops entirely. Blocks accumulate indefinitely in object storage. Query performance degrades over time as the number of uncompacted blocks grows. Retention enforcement also stops.

**Alert Expression:**
```promql
thanos_compact_halted{job=~"thanos-compact.*"} == 1
```

**Steps:**

- **In Grafana**, confirm the halted state and when it started:
   ```promql
   thanos_compact_halted{job=~"thanos-compact.*", namespace="rhobs-production"}
   ```

- **Read compactor logs** to find the reason for the halt — search for `halt` or `critical`:
   ```bash
   kubectl logs -n rhobs-production <compact-pod> | grep -iE "halt|critical|error|fatal"
   ```

- **Common halt causes and remediation:**
   - **Corrupted blocks:** Run `thanos tools bucket verify` to identify and optionally repair blocks.
   - **Block overlap:** Indicates a prior multi-compactor scenario — inspect bucket contents.
   - **Object storage errors:** Check credentials, bucket permissions, and network access.
   - **Insufficient disk space:** The compactor downloads blocks locally before merging — check node disk.
     ```bash
     kubectl exec -n rhobs-production <compact-pod> -- df -h
     ```

- **Check object storage secret** is valid and not rotated:
   ```bash
   kubectl get secret -n rhobs-production <objstore-secret-name> -o yaml
   ```

- **After fixing the root cause**, restart the pod to clear the halt:
   ```bash
   kubectl delete pod -n rhobs-production <compact-pod>
   ```
   Or via automated actions (if you lack delete permissions):
   ```bash
   automated-actions openshift-workload-delete --cluster <cluster> --namespace rhobs-production --kind Pod --name <compact-pod>
   ```

- **In Grafana**, confirm `thanos_compact_halted` returns to 0 after restart:
   ```promql
   thanos_compact_halted{job=~"thanos-compact.*", namespace="rhobs-production"}
   ```

- **Check compaction backlog** — the **TODO Compaction Blocks** and **TODO Compactions** panels show how much work has accumulated since the halt:
   ```promql
   sum by (namespace, job) (thanos_compact_todo_compaction_blocks{job=~"thanos-compact.*", namespace="rhobs-production"})
   ```
   ```promql
   sum by (namespace, job) (thanos_compact_todo_compactions{job=~"thanos-compact.*", namespace="rhobs-production"})
   ```

- **Check garbage collection failure ratio** — the **Garbage Collection Errors** panel. GC and compaction failing together indicate a shared root cause (storage errors or block corruption):
   ```promql
   (
     sum by (namespace, job) (rate(thanos_compact_garbage_collection_failures_total{job=~"thanos-compact.*", namespace="rhobs-production"}[5m]))
     /
     sum by (namespace, job) (rate(thanos_compact_garbage_collection_total{job=~"thanos-compact.*", namespace="rhobs-production"}[5m]))
   ) * 100
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs/exec/delete` on compact pods
- Object storage access (to inspect/repair blocks)
- Grafana access

---

## ThanosCompactHighCompactionFailures

**Severity:** `medium` | **For:** 15m | **Component:** Thanos Compact

**Summary:**
More than 5% of compaction attempts are failing, sustained for 15 minutes. The alert measures the ratio of `thanos_compact_group_compactions_failures_total` to `thanos_compact_group_compactions_total`.

**Impact:**
Failed compactions mean blocks are not being merged. Over time, query performance degrades because queries must scan an ever-increasing number of small blocks. Object storage costs also increase.

**Alert Expression:**
```promql
(
    sum by (namespace, job) (
      rate(thanos_compact_group_compactions_failures_total{job=~"thanos-compact.*"}[5m])
    )
  /
    sum by (namespace, job) (rate(thanos_compact_group_compactions_total{job=~"thanos-compact.*"}[5m]))
)
* 100 > 5
```

**Steps:**

- **In Grafana**, plot the failure rate over time to understand the trend:
   ```promql
   (
     sum by (namespace, job) (rate(thanos_compact_group_compactions_failures_total{job=~"thanos-compact.*", namespace="rhobs-production"}[5m]))
     /
     sum by (namespace, job) (rate(thanos_compact_group_compactions_total{job=~"thanos-compact.*", namespace="rhobs-production"}[5m]))
   ) * 100
   ```

- **Read compactor logs** for the specific failure message:
   ```bash
   kubectl logs -n rhobs-production <compact-pod> | grep -iE "compaction.*fail|error|group"
   ```

- **Check resource pressure** — compaction is CPU and memory intensive:
   ```bash
   kubectl top pod -n rhobs-production <compact-pod>
   ```
   If limits are too low, edit the `ThanosCompact` CR to increase them.

- **Check object storage operation errors** alongside compaction failures:
   ```promql
   sum by (namespace, job) (rate(thanos_objstore_bucket_operation_failures_total{job=~"thanos-compact.*", namespace="rhobs-production"}[5m]))
   ```
   If this is also elevated, the root cause is object storage — see [ThanosCompactBucketHighOperationFailures](#thanoscompactbuckethighoperationfailures).

- **Check for specific block groups** causing repeated failures in the logs — a single corrupted block group can skew the failure rate significantly.

- **Check block metadata sync error rate** — the **Sync Meta Errors** panel shows whether the compactor is failing to sync block metadata from object storage (a prerequisite for compaction):
   ```promql
   (
     sum by (namespace, job) (rate(thanos_blocks_meta_sync_failures_total{job=~"thanos-compact.*", namespace="rhobs-production"}[5m]))
     /
     sum by (namespace, job) (rate(thanos_blocks_meta_syncs_total{job=~"thanos-compact.*", namespace="rhobs-production"}[5m]))
   ) * 100
   ```

- **Check garbage collection failure ratio** — the **Garbage Collection Errors** panel. GC and compaction failing together suggest object storage or block-level corruption as the common root cause:
   ```promql
   (
     sum by (namespace, job) (rate(thanos_compact_garbage_collection_failures_total{job=~"thanos-compact.*", namespace="rhobs-production"}[5m]))
     /
     sum by (namespace, job) (rate(thanos_compact_garbage_collection_total{job=~"thanos-compact.*", namespace="rhobs-production"}[5m]))
   ) * 100
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs/top` on compact pods
- `kubectl edit` on `ThanosCompact` CR (to adjust resources if needed)
- Grafana access

---

## ThanosCompactBucketHighOperationFailures

**Severity:** `medium` | **For:** 15m | **Component:** Thanos Compact

**Summary:**
More than 5% of object storage (bucket) operations performed by the compactor are failing, sustained for 15 minutes.

**Impact:**
Compactor cannot read from or write to object storage reliably. This blocks compaction and prevents new blocks from being uploaded or old blocks from being cleaned up after merging.

**Alert Expression:**
```promql
(
    sum by (namespace, job) (
      rate(thanos_objstore_bucket_operation_failures_total{job=~"thanos-compact.*"}[5m])
    )
  /
    sum by (namespace, job) (rate(thanos_objstore_bucket_operations_total{job=~"thanos-compact.*"}[5m]))
)
* 100 > 5
```

**Steps:**

- **In Grafana**, check failure rate by operation type:
   ```promql
   sum by (namespace, job, operation) (
     rate(thanos_objstore_bucket_operation_failures_total{job=~"thanos-compact.*", namespace="rhobs-production"}[5m])
   )
   ```

- **Read compactor logs** for storage errors:
   ```bash
   kubectl logs -n rhobs-production <compact-pod> | grep -iE "bucket|s3|gcs|azure|storage|error"
   ```

- **Verify the object storage secret** hasn't expired or been rotated:
   ```bash
   kubectl get secret -n rhobs-production <objstore-secret-name>
   ```

- **Check for network issues** between the compactor pod and the storage endpoint — look for DNS resolution failures or TLS errors in logs.

- **Check object storage service status** (AWS S3, GCS, Azure Blob) in the cloud provider console. Throttling or regional outages will manifest as operation failures.

- **Check IAM permissions** — the compactor needs `GetObject`, `PutObject`, `DeleteObject`, `ListBucket` on the bucket.

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs` on compact pods
- Cloud provider console access (object storage health, IAM)
- `kubectl get` on secrets in the namespace
- Grafana access

---

## ThanosCompactHasNotRun

**Severity:** `medium` | **For:** 5m (no-for clause — fires immediately) | **Component:** Thanos Compact

**Summary:**
The compactor has not successfully uploaded anything to object storage in the last 24 hours. Measured via `thanos_objstore_bucket_last_successful_upload_time`.

**Impact:**
Blocks are not being compacted or that compacted blocks are not being persisted. Queries accumulate overhead from uncompacted small blocks, and object storage costs grow unchecked.

**Alert Expression:**
```promql
(
    time()
  -
    max by (namespace, job) (
      max_over_time(thanos_objstore_bucket_last_successful_upload_time{job=~"thanos-compact.*"}[1d])
    )
)
/ 60 / 60 > 24
```

**Steps:**

- **In Grafana**, check last successful upload timestamp:
   ```promql
   max by (namespace, job) (
     thanos_objstore_bucket_last_successful_upload_time{job=~"thanos-compact.*", namespace="rhobs-production"}
   )
   ```
   Convert the Unix timestamp to a human-readable time to understand how long it's been.

- **Check if the compactor is running at all:**
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=thanos-compact
   ```

- **Check if there's anything to compact** — if the bucket is newly provisioned or all blocks are already at max compaction level, the compactor may not upload anything. This is a false positive in that scenario.

- **Read logs for stuck or failing compaction cycles:**
   ```bash
   kubectl logs -n rhobs-production <compact-pod> --since=25h | grep -iE "upload|compact|error|halt"
   ```

- **Check if compactor is halted** — see [ThanosCompactHalted](#thanoscompacthalted):
   ```promql
   thanos_compact_halted{job=~"thanos-compact.*", namespace="rhobs-production"}
   ```

- **Check compaction rate** to distinguish "nothing to compact" from "stuck":
   ```promql
   sum by (namespace, job) (rate(thanos_compact_group_compactions_total{job=~"thanos-compact.*", namespace="rhobs-production"}[1h]))
   ```

- **Check pending compaction work** — the **TODO Compaction Blocks** panel shows blocks pending compaction. If this is 0, the compactor is idle (no work to do, not stuck):
   ```promql
   sum by (namespace, job) (thanos_compact_todo_compaction_blocks{job=~"thanos-compact.*", namespace="rhobs-production"})
   ```
   If > 0 but no upload is happening, the compactor is genuinely stuck — see [ThanosCompactHalted](#thanoscompacthalted).

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl get/logs` on compact pods
- Grafana access

---

## Thanos Query Alerts

> **Grafana Dashboard:** [Thanos / Query / Overview](https://grafana.app-sre.devshift.net/d/thanos-query-overview/thanos-query-overview) — select the `<cluster>-prometheus` datasource from the dropdown.

---

## ThanosQueryHttpRequestQueryErrorRateHigh

**Severity:** `high` | **For:** 5m | **Component:** Thanos Query

**Summary:**
More than 5% of HTTP requests to the Thanos Query `/api/v1/query` endpoint are returning 5xx errors, sustained for 5 minutes.

**Impact:**
Instant queries from Grafana, alerting rules, and any tooling using the Thanos Query HTTP API are failing. Dashboards will show errors. Alerting rules evaluated via this endpoint will fail to evaluate.

**Alert Expression:**
```promql
(
    sum by (namespace, job) (
      rate(http_requests_total{code=~"5..",handler="query",job=~"thanos-query.*"}[5m])
    )
  /
    sum by (namespace, job) (rate(http_requests_total{handler="query",job=~"thanos-query.*"}[5m]))
)
* 100 > 5
```

**Steps:**

- **In Grafana**, plot HTTP error rate for the query handler:
   ```promql
   (
     sum by (namespace, job, code) (rate(http_requests_total{handler="query",job=~"thanos-query.*", namespace="rhobs-production"}[5m]))
   )
   ```
   Inspect which HTTP status codes are being returned (502, 503, 504).

- **Check backend store health** — 5xx errors are often caused by all store endpoints being down or timing out:
   ```promql
   sum by (namespace, job) (thanos_store_nodes_grpc_connections{job=~"thanos-query.*", namespace="rhobs-production"})
   ```
   If this is 0 or very low, the query frontend has no backends to query.

- **Check query pod logs** for error patterns:
   ```bash
   kubectl logs -n rhobs-production -l app.kubernetes.io/name=thanos-query | grep -E '"code":"5[0-9]{2}"'
   ```

- **Verify store and ingester endpoints** are reachable from the query pod:
   ```bash
   kubectl get endpoints -n rhobs-production | grep -E "store|receive-ingester"
   ```

- **Check query pod resource usage** — OOM or CPU throttling causes request failures:
   ```bash
   kubectl top pod -n rhobs-production -l app.kubernetes.io/name=thanos-query
   ```

- **Check the ThanosQuery CR** for misconfigured store endpoints:
   ```bash
   kubectl describe thanosquery <name> -n rhobs-production
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs/top/describe` on query pods
- `kubectl get` on endpoints and `ThanosQuery` CRs
- Grafana access

---

## ThanosQueryGrpcServerErrorRate

**Severity:** `medium` | **For:** 5m | **Component:** Thanos Query

**Summary:**
More than 5% of gRPC server requests handled by Thanos Query are returning error codes (`Unknown`, `ResourceExhausted`, `Internal`, `Unavailable`, `DataLoss`, `DeadlineExceeded`), sustained for 5 minutes.

**Impact:**
Components that query Thanos via gRPC (such as Thanos Rule) are receiving errors. Alerting rule evaluation against this Thanos Query instance is degraded.

**Alert Expression:**
```promql
(
    sum by (namespace, job) (
      rate(
        grpc_server_handled_total{grpc_code=~"Unknown|ResourceExhausted|Internal|Unavailable|DataLoss|DeadlineExceeded",job=~"thanos-query.*"}[5m]
      )
    )
  /
    sum by (namespace, job) (rate(grpc_server_started_total{job=~"thanos-query.*"}[5m]))
)
* 100 > 5
```

**Steps:**

- **In Grafana**, break down gRPC errors by code to understand the failure mode:
   ```promql
   sum by (namespace, job, grpc_code, grpc_method) (
     rate(grpc_server_handled_total{grpc_code!="OK",job=~"thanos-query.*", namespace="rhobs-production"}[5m])
   )
   ```
   - `ResourceExhausted` → memory/concurrency limit hit
   - `DeadlineExceeded` → slow backend stores causing timeouts
   - `Unavailable` → query pod itself is struggling

- **Check concurrent query gate saturation** — if the gate is full, new requests get `ResourceExhausted`:
   ```promql
   max_over_time(thanos_query_concurrent_gate_queries_max{job=~"thanos-query.*", namespace="rhobs-production"}[5m])
   - avg_over_time(thanos_query_concurrent_gate_queries_in_flight{job=~"thanos-query.*", namespace="rhobs-production"}[5m])
   ```
   If this is near 0, the query pod is overloaded.

- **Check store backend latency** — slow stores cause `DeadlineExceeded`:
   ```promql
   histogram_quantile(0.99, sum by (namespace, job, le) (
     rate(thanos_store_series_fetch_duration_seconds_bucket{job=~"thanos-query.*", namespace="rhobs-production"}[5m])
   ))
   ```

- **Read query pod logs** for gRPC error context:
   ```bash
   kubectl logs -n rhobs-production <query-pod> | grep -iE "grpc|error|deadline|exhausted"
   ```

- **Check query pod resources:**
   ```bash
   kubectl top pod -n rhobs-production -l app.kubernetes.io/name=thanos-query
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs/top` on query pods
- Grafana access

---

## ThanosQueryGrpcClientErrorRate

**Severity:** `medium` | **For:** 5m | **Component:** Thanos Query

**Summary:**
More than 5% of outbound gRPC client calls made by Thanos Query to its store endpoints are failing (any non-`OK` gRPC code), sustained for 5 minutes.

**Impact:**
Thanos Query is unable to reach one or more store backends (Store Gateways, Receive Ingesters, Sidecars). Queries will return partial or no results, and some time ranges may be entirely missing from query responses.

**Alert Expression:**
```promql
(
    sum by (namespace, job) (rate(grpc_client_handled_total{grpc_code!="OK",job=~"thanos-query.*"}[5m]))
  /
    sum by (namespace, job) (rate(grpc_client_started_total{job=~"thanos-query.*"}[5m]))
)
* 100 > 5
```

**Steps:**

- **In Grafana**, check which gRPC codes are failing and to which endpoints:
   ```promql
   sum by (namespace, job, grpc_code, grpc_method) (
     rate(grpc_client_handled_total{grpc_code!="OK",job=~"thanos-query.*", namespace="rhobs-production"}[5m])
   )
   ```

- **Check if the target store/ingester pods are healthy:**
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=thanos-store
   kubectl get pods -n rhobs-production -l app.kubernetes.io/component=thanos-receive-ingester
   ```

- **Check gRPC connection count** to each backend — a drop to 0 means complete connectivity loss:
   ```promql
   thanos_store_nodes_grpc_connections{job=~"thanos-query.*", namespace="rhobs-production"}
   ```

- **Check for TLS/certificate issues** in query pod logs:
   ```bash
   kubectl logs -n rhobs-production <query-pod> | grep -iE "tls|certificate|dial|connection refused"
   ```

- **Verify the operator-labeled store services exist** — the operator labels all Store/Receive services it creates with `operator.thanos.io/store-api=true` so ThanosQuery can discover them. If a service is missing this label, the query won't connect:
   ```bash
   kubectl get svc -n rhobs-production -l operator.thanos.io/store-api=true,app.kubernetes.io/part-of=thanos
   ```

- **Verify network policies** aren't blocking gRPC traffic (typically port 10901) between query and store pods:
   ```bash
   kubectl get networkpolicy -n rhobs-production
   ```

- **Check DNS resolution** from the query pod for store service names — DNS failures prevent gRPC connections from being established. See also [ThanosQueryHighDNSFailures](#thanosqueryhighdnsfailures).

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl get/logs` on query, store, and receive-ingester pods
- `kubectl get` on NetworkPolicies
- Grafana access

---

## ThanosQueryHighDNSFailures

**Severity:** `medium` | **For:** 15m | **Component:** Thanos Query

**Summary:**
More than 1% of DNS lookups performed by Thanos Query for store endpoint discovery are failing, sustained for 15 minutes.

**Impact:**
Thanos Query uses DNS to discover store endpoints dynamically (especially when configured with `dnssrv+` or `dns+` prefixes). DNS failures mean new or recovered store endpoints are not discovered, and existing endpoint lists may become stale.

**Alert Expression:**
```promql
(
    sum by (namespace, job) (
      rate(thanos_query_store_apis_dns_failures_total{job=~"thanos-query.*"}[5m])
    )
  /
    sum by (namespace, job) (rate(thanos_query_store_apis_dns_lookups_total{job=~"thanos-query.*"}[5m]))
)
* 100 > 1
```

**Steps:**

- **In Grafana**, check the DNS failure ratio over time:
   ```promql
   (
     sum by (namespace, job) (rate(thanos_query_store_apis_dns_failures_total{job=~"thanos-query.*", namespace="rhobs-production"}[5m]))
     /
     sum by (namespace, job) (rate(thanos_query_store_apis_dns_lookups_total{job=~"thanos-query.*", namespace="rhobs-production"}[5m]))
   ) * 100
   ```

- **Check query pod logs** for the specific hostname failing DNS resolution:
   ```bash
   kubectl logs -n rhobs-production <query-pod> | grep -iE "dns|lookup|resolve"
   ```

- **Test DNS resolution manually** from within the query pod:
   ```bash
   kubectl exec -n rhobs-production <query-pod> -- nslookup thanos-store-default-shard-0.rhobs-production.svc.cluster.local
   kubectl exec -n rhobs-production <query-pod> -- nslookup thanos-receive-ingester-rhobs-default.rhobs-production.svc.cluster.local
   ```

- **Verify the operator-labeled store/receive services exist** — the operator labels all ThanosStore and ThanosReceive services with `operator.thanos.io/store-api=true`. ThanosQuery uses these for endpoint discovery. If they don't exist, the query has nothing to connect to:
   ```bash
   kubectl get svc -n rhobs-production -l operator.thanos.io/store-api=true
   ```

- **Check the ThanosQuery CR for `storeLabelSelector`** — an overly narrow selector will prevent services from being discovered:
   ```bash
   kubectl get thanosquery <name> -n rhobs-production -o jsonpath='{.spec.storeLabelSelector}'
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl exec` into query pods
- `kubectl describe` on `ThanosQuery` CRs
- Grafana access

---

## ThanosQueryInstantLatencyHigh

**Severity:** `high` | **For:** 10m | **Component:** Thanos Query

**Summary:**
The 99th percentile latency for HTTP instant queries (`/api/v1/query`) exceeds 90 seconds, sustained for 10 minutes. Only fires when there is active query traffic.

**Impact:**
End-user queries and Grafana dashboards experience severe timeouts. Alerting rule evaluations that use Thanos Query as the backend will time out and fail.

**Alert Expression:**
```promql
histogram_quantile(
  0.99,
  sum by (namespace, job, le) (
    rate(http_request_duration_seconds_bucket{handler="query",job=~"thanos-query.*"}[5m])
  )
)
> 90
and
sum by (namespace, job) (
  rate(http_request_duration_seconds_count{handler="query",job=~"thanos-query.*"}[5m])
) > 0
```

**Steps:**

- **In Grafana**, plot latency percentiles to understand distribution:
   ```promql
   histogram_quantile(0.99, sum by (namespace, job, le) (
     rate(http_request_duration_seconds_bucket{handler="query",job=~"thanos-query.*", namespace="rhobs-production"}[5m])
   ))
   ```
   Also check p50 and p90 to understand if this is systemic or tail latency only.

- **Check store backend latency** — slow stores are the most common cause:
   ```promql
   histogram_quantile(0.99, sum by (namespace, job, le) (
     rate(thanos_store_series_fetch_duration_seconds_bucket{job=~"thanos-query.*", namespace="rhobs-production"}[5m])
   ))
   ```

- **Check object storage latency** from the Store Gateway:
   ```promql
   histogram_quantile(0.99, sum by (namespace, job, le) (
     rate(thanos_objstore_bucket_operation_duration_seconds_bucket{job=~"thanos-store.*", namespace="rhobs-production"}[5m])
   ))
   ```

- **Check if the query concurrency gate is saturated** (queries queueing behind gate):
   ```promql
   max_over_time(thanos_query_concurrent_gate_queries_max{job=~"thanos-query.*", namespace="rhobs-production"}[5m])
   - avg_over_time(thanos_query_concurrent_gate_queries_in_flight{job=~"thanos-query.*", namespace="rhobs-production"}[5m])
   ```

- **Check query pod resource usage:**
   ```bash
   kubectl top pod -n rhobs-production -l app.kubernetes.io/name=thanos-query
   ```

- **Look for slow queries in logs** — queries scanning large time ranges or high-cardinality selectors:
   ```bash
   kubectl logs -n rhobs-production <query-pod> | grep -iE "slow|duration|timeout"
   ```

- **Scale query pods** if load is the issue:
   ```bash
   kubectl patch thanosquery <name> -n rhobs-production --type=merge -p '{"spec":{"replicas":<new-count>}}'
   ```

- **Check Store Gateway series fetch duration** — the **Query Operation Durations** / **Get All Series Duration** panels in the [Store Gateway dashboard](https://grafana.app-sre.devshift.net/d/thanos-store-overview/thanos-store-overview) show how long the store spends fetching series from object storage (typically the dominant cost):
   ```promql
   histogram_quantile(0.99, sum by (namespace, job, le) (
     rate(thanos_bucket_store_series_get_all_duration_seconds_bucket{job=~"thanos-store.*", namespace="rhobs-production"}[5m])
   ))
   ```

- **Check Store Gateway merge duration** — the **Merge Durations** panel. High merge durations indicate too many series being returned across too many blocks (compaction lag):
   ```promql
   histogram_quantile(0.99, sum by (namespace, job, le) (
     rate(thanos_bucket_store_series_merge_duration_seconds_bucket{job=~"thanos-store.*", namespace="rhobs-production"}[5m])
   ))
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl top/logs/patch` on query pods and `ThanosQuery` CRs
- Grafana access

---

## Thanos Receive Alerts

> **Grafana Dashboard:** [Thanos / Receive / Overview](https://grafana.app-sre.devshift.net/d/thanos-receive-overview/thanos-receive-overview) — select the `<cluster>-prometheus` datasource from the dropdown. For TSDB-level ingester health (WAL, head compaction), also use [Thanos / TSDB Monitoring](https://grafana.app-sre.devshift.net/d/thanos-tsdb-monitoring/thanos-tsdb-monitoring).

---

## ThanosReceiveHttpRequestErrorRateHigh

**Severity:** `high` | **For:** 5m | **Component:** Thanos Receive Router

**Summary:**
More than 5% of HTTP remote write requests (`handler="receive"`) to the Thanos Receive Router are returning 5xx errors, sustained for 5 minutes.

**Impact:**
Prometheus remote write to this Thanos Receive cluster is failing. Metrics from all affected Prometheus instances are not being ingested, causing data gaps that grow until the issue is resolved. Remote write senders will start buffering and eventually drop data if the outage persists.

**Alert Expression:**
```promql
(
    sum by (namespace, job) (
      rate(http_requests_total{code=~"5..",handler="receive",job=~"thanos-receive-router.*"}[5m])
    )
  /
    sum by (namespace, job) (
      rate(http_requests_total{handler="receive",job=~"thanos-receive-router.*"}[5m])
    )
)
* 100 > 5
```

**Steps:**

- **In Grafana**, plot the 5xx error rate and identify the HTTP status codes:
   ```promql
   sum by (namespace, job, code) (
     rate(http_requests_total{handler="receive",job=~"thanos-receive-router.*", namespace="rhobs-production"}[5m])
   )
   ```

- **Check router logs** for the root cause:
   ```bash
   kubectl logs -n rhobs-production -l app.kubernetes.io/component=thanos-receive-router | grep -E '"status":5[0-9]{2}'
   ```

- **Check ingester availability** — the router forwards to ingesters; if ingesters are down the router returns 5xx:
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/component=thanos-receive-ingester
   ```
   ```promql
   up{job=~"thanos-receive-ingester.*", namespace="rhobs-production"}
   ```

- **Check hashring configuration** — invalid hashring causes routing failures:
   ```promql
   thanos_receive_config_last_reload_successful{job=~"thanos-receive-router.*", namespace="rhobs-production"}
   ```
   If not 1, see [ThanosReceiveConfigReloadFailure](#thanosreceiveconfigreloadfailure).

- **Check replication failures** alongside HTTP errors:
   ```promql
   sum by (namespace, job) (rate(thanos_receive_replications_total{result="error",job=~"thanos-receive-router.*", namespace="rhobs-production"}[5m]))
   ```

- **Check router resource usage:**
   ```bash
   kubectl top pod -n rhobs-production -l app.kubernetes.io/component=thanos-receive-router
   ```

- **Check replication success vs. error breakdown** — the **Remote Write Replication** row in the [Receive dashboard](https://grafana.app-sre.devshift.net/d/thanos-receive-overview/thanos-receive-overview) shows whether router-to-ingester replication is succeeding. Replication errors cause router 5xx responses to senders:
   ```promql
   sum by (namespace, job, result) (
     rate(thanos_receive_replications_total{job=~"thanos-receive-router.*", namespace="rhobs-production"}[5m])
   )
   ```

- **Check ingester TSDB head chunk count** — the **Head Chunks** panel in the Receive dashboard. Excessively large heads indicate ingester write pressure:
   ```promql
   sum by (namespace, job) (prometheus_tsdb_head_chunks{job=~"thanos-receive-ingester.*", namespace="rhobs-production"})
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs/top` on receive-router and receive-ingester pods
- Grafana access

---

## ThanosReceiveHttpRequestLatencyHigh

**Severity:** `high` | **For:** 10m | **Component:** Thanos Receive Router

**Summary:**
The 99th percentile latency for HTTP remote write requests to the Thanos Receive Router exceeds 10 seconds, sustained for 10 minutes.

**Impact:**
Prometheus remote write senders experience slow responses and may time out. Prometheus will log remote write errors and start dropping samples from its WAL queue if the latency persists beyond its configured timeout.

**Alert Expression:**
```promql
histogram_quantile(
  0.99,
  sum by (namespace, job, le) (
    rate(http_request_duration_seconds_bucket{handler="receive",job=~"thanos-receive-router.*"}[5m])
  )
)
> 10
and
sum by (namespace, job) (
  rate(http_request_duration_seconds_count{handler="receive",job=~"thanos-receive-router.*"}[5m])
) > 0
```

**Steps:**

- **In Grafana**, plot receive latency percentiles:
   ```promql
   histogram_quantile(0.99, sum by (namespace, job, le) (
     rate(http_request_duration_seconds_bucket{handler="receive",job=~"thanos-receive-router.*", namespace="rhobs-production"}[5m])
   ))
   ```

- **Check ingester write latency** — ingesters do the actual TSDB write, slow writes propagate to router latency:
   ```promql
   histogram_quantile(0.99, sum by (namespace, job, le) (
     rate(http_request_duration_seconds_bucket{handler="receive",job=~"thanos-receive-ingester.*", namespace="rhobs-production"}[5m])
   ))
   ```

- **Check ingester resource usage** — TSDB head compaction or WAL replay can cause write spikes:
   ```bash
   kubectl top pod -n rhobs-production -l app.kubernetes.io/component=thanos-receive-ingester
   ```

- **Check replication overhead** — high replication factor with slow ingesters multiplies latency:
   ```promql
   thanos_receive_replication_factor{job=~"thanos-receive-router.*", namespace="rhobs-production"}
   ```

- **Check ingester TSDB metrics** for compaction or head series pressure:
   ```promql
   sum by (namespace, job) (prometheus_tsdb_head_series{job=~"thanos-receive-ingester.*", namespace="rhobs-production"})
   ```

- **Check if ingesters are hitting head series limits** (which causes write blocking):
   ```promql
   sum by (namespace, job, tenant) (increase(thanos_receive_head_series_limited_requests_total{job=~"thanos-receive-router.*", namespace="rhobs-production"}[5m]))
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl top` on receive-ingester pods
- Grafana access

---

## ThanosReceiveHighReplicationFailures

**Severity:** `medium` | **For:** 5m | **Component:** Thanos Receive Router

**Summary:**
The replication failure rate among ingesters exceeds the acceptable threshold based on the configured replication factor. Specifically, the error fraction exceeds `floor((replication_factor + 1) / 2) / hashring_nodes`. Only fires when `thanos_receive_replication_factor > 1`.

**Impact:**
Data is not being replicated to the minimum required number of ingesters. If enough ingesters fail and data was only written to failed nodes, that data will be permanently lost. Queries may return incomplete results.

**Alert Expression:**
```promql
thanos_receive_replication_factor > 1
and
(
    (
        sum by (namespace, job) (
          rate(thanos_receive_replications_total{job=~"thanos-receive-router.*",result="error"}[5m])
        )
      /
        sum by (namespace, job) (
          rate(thanos_receive_replications_total{job=~"thanos-receive-router.*"}[5m])
        )
    )
  >
    (
        max by (namespace, job) (
          floor(thanos_receive_replication_factor{job=~"thanos-receive-router.*"} + 1 / 2)
        )
      /
        max by (namespace, job) (thanos_receive_hashring_nodes{job=~"thanos-receive-router.*"})
    )
  * 100
)
```

**Steps:**

- **In Grafana**, check overall replication error rate:
   ```promql
   sum by (namespace, job, result) (
     rate(thanos_receive_replications_total{job=~"thanos-receive-router.*", namespace="rhobs-production"}[5m])
   )
   ```

- **Check ingester pod health** — replication fails if target ingesters are down:
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/component=thanos-receive-ingester
   ```

- **Check hashring node count** against replication factor:
   ```promql
   thanos_receive_hashring_nodes{job=~"thanos-receive-router.*", namespace="rhobs-production"}
   ```
   ```promql
   thanos_receive_replication_factor{job=~"thanos-receive-router.*", namespace="rhobs-production"}
   ```
   If hashring nodes < replication factor, replication is structurally impossible.

- **Check router logs** for specific replication error messages:
   ```bash
   kubectl logs -n rhobs-production <receive-router-pod> | grep -iE "replicate|replication|error"
   ```

- **Check network connectivity** between the router and ingester pods — look for connection refused or timeout errors in logs.

- **Verify the hashring ConfigMap** reflects current ingester endpoints:
   ```bash
   kubectl get configmap -n rhobs-production <hashring-configmap> -o yaml
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl get/logs` on receive-router and receive-ingester pods
- `kubectl get` on hashring ConfigMaps
- Grafana access

---

## ThanosReceiveHighForwardRequestFailures

**Severity:** `high` | **For:** 5m | **Component:** Thanos Receive Router

**Summary:**
More than 20% of forward requests from the Thanos Receive Router to target ingesters are failing, sustained for 5 minutes. Forwarding is how the router sends writes to the correct ingester based on hashring assignment.

**Impact:**
A significant portion of incoming remote write data is not being delivered to the correct ingesters. This causes data loss for the affected time series — data is accepted at the router but not stored anywhere permanently.

**Alert Expression:**
```promql
(
    sum by (namespace, job) (
      rate(thanos_receive_forward_requests_total{job=~"thanos-receive-router.*",result="error"}[5m])
    )
  /
    sum by (namespace, job) (
      rate(thanos_receive_forward_requests_total{job=~"thanos-receive-router.*"}[5m])
    )
)
* 100 > 20
```

**Steps:**

- **In Grafana**, plot forward request error rate:
   ```promql
   sum by (namespace, job, result) (
     rate(thanos_receive_forward_requests_total{job=~"thanos-receive-router.*", namespace="rhobs-production"}[5m])
   )
   ```

- **Check ingester pods** — forward failures are almost always caused by ingesters being unreachable:
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/component=thanos-receive-ingester
   ```

- **Check router logs** for the specific error when forwarding:
   ```bash
   kubectl logs -n rhobs-production <receive-router-pod> | grep -iE "forward|error|connect"
   ```

- **Verify the hashring ConfigMap** contains valid and current ingester addresses:
   ```bash
   kubectl get configmap -n rhobs-production <hashring-configmap> -o yaml
   ```

- **Check if the hashring was recently updated** and if the reload succeeded:
   ```promql
   thanos_receive_config_last_reload_successful{job=~"thanos-receive-router.*", namespace="rhobs-production"}
   ```

- **Check network policies** allow traffic from router to ingester pods on the receive port (default 10908):
   ```bash
   kubectl get networkpolicy -n rhobs-production
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl get/logs` on receive-router and receive-ingester pods
- `kubectl get` on ConfigMaps and NetworkPolicies
- Grafana access

---

## ThanosReceiveHighHashringFileRefreshFailures

**Severity:** `medium` | **For:** 15m | **Component:** Thanos Receive Router

**Summary:**
Any hashring file refresh attempts by the Thanos Receive Router are failing (ratio > 0), sustained for 15 minutes. The router periodically re-reads the hashring file to detect endpoint changes.

**Impact:**
The router's hashring is stale. New ingesters added to the cluster will not receive traffic. Removed ingesters will still receive forwarding attempts, causing forward failures. Dynamic scaling of ingesters is effectively broken.

**Alert Expression:**
```promql
(
    sum by (namespace, job) (
      rate(thanos_receive_hashrings_file_errors_total{job=~"thanos-receive-router.*"}[5m])
    )
  /
    sum by (namespace, job) (
      rate(thanos_receive_hashrings_file_refreshes_total{job=~"thanos-receive-router.*"}[5m])
    )
)
> 0
```

**Steps:**

- **In Grafana**, check the refresh error rate:
   ```promql
   sum by (namespace, job) (
     rate(thanos_receive_hashrings_file_errors_total{job=~"thanos-receive-router.*", namespace="rhobs-production"}[5m])
   )
   ```

- **Read router logs** for the specific error:
   ```bash
   kubectl logs -n rhobs-production <receive-router-pod> | grep -iE "hashring|refresh|file|error"
   ```

- **Verify the operator-generated hashring ConfigMap exists and is mounted** — this ConfigMap (`thanos-receive-router-<receive-name>`) is created and maintained by the operator from ingester EndpointSlices. It is **not** user-editable:
   ```bash
   kubectl get configmap -n rhobs-production thanos-receive-router-<receive-name> -o yaml
   kubectl describe pod -n rhobs-production <receive-router-pod> | grep -A5 "Volumes"
   ```

- **Check operator logs** — if the hashring ConfigMap is malformed or not reconciled, the operator is the source to fix:
   ```bash
   kubectl logs -n rhobs-production -l control-plane=controller-manager | grep -iE "hashring|ThanosReceive|error"
   ```

- **Check the health of ingester EndpointSlices** — the operator builds the hashring from ready ingester endpoints. If EndpointSlices are empty, the operator generates an empty hashring:
   ```bash
   kubectl get endpointslices -n rhobs-production -l app.kubernetes.io/component=thanos-receive-ingester
   ```

- **Check file permissions** inside the router pod to confirm the ConfigMap volume is mounted correctly:
   ```bash
   kubectl exec -n rhobs-production <receive-router-pod> -- ls -la /etc/thanos/hashring/
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl get/exec/describe` on receive-router pods
- `kubectl get` on ConfigMaps
- Grafana access

---

## ThanosReceiveConfigReloadFailure

**Severity:** `medium` | **For:** 5m | **Component:** Thanos Receive Router

**Summary:**
The `thanos_receive_config_last_reload_successful` metric for the Thanos Receive Router is not equal to 1, meaning the last hashring configuration reload failed.

**Impact:**
The router is operating on a previous (possibly stale) hashring configuration. Configuration changes (adding/removing ingesters, changing tenant routing) are not being applied. This may cause routing errors or uneven load distribution.

**Alert Expression:**
```promql
avg by (namespace, job) (
  thanos_receive_config_last_reload_successful{job=~"thanos-receive-router.*"}
)
!= 1
```

**Steps:**

- **In Grafana**, check the reload success gauge:
   ```promql
   thanos_receive_config_last_reload_successful{job=~"thanos-receive-router.*", namespace="rhobs-production"}
   ```

- **Read router logs** immediately after the last reload attempt:
   ```bash
   kubectl logs -n rhobs-production <receive-router-pod> | grep -iE "reload|config|error"
   ```

- **Check the operator-generated hashring ConfigMap** — the operator builds `thanos-receive-router-<receive-name>` from ingester EndpointSlices. If ingesters are unhealthy, the hashring may be empty or invalid:
   ```bash
   kubectl get configmap -n rhobs-production thanos-receive-router-<receive-name> -o jsonpath='{.data.hashrings\.json}'
   kubectl get endpointslices -n rhobs-production -l app.kubernetes.io/component=thanos-receive-ingester
   ```
   If ingester endpoints are empty, fix the ingesters first — the operator will regenerate the hashring automatically.

- **Check operator logs** for reconciliation errors on the ThanosReceive CR:
   ```bash
   kubectl logs -n rhobs-production -l control-plane=controller-manager | grep -iE "ThanosReceive|hashring|error"
   ```

- **Restart the router pod** only after confirming the hashring ConfigMap is valid:
   ```bash
   kubectl delete pod -n rhobs-production <receive-router-pod>
   ```
   Or via automated actions (if you lack delete permissions):
   ```bash
   automated-actions openshift-workload-delete --cluster <cluster> --namespace rhobs-production --kind Pod --name <receive-router-pod>
   ```

- **After restart, verify reload succeeds:**
   ```promql
   thanos_receive_config_last_reload_successful{job=~"thanos-receive-router.*", namespace="rhobs-production"}
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs/delete` on receive-router pods
- `kubectl get` on ConfigMaps
- Grafana access

---

## ThanosReceiveNoUpload

**Severity:** `high` | **For:** 4h | **Component:** Thanos Receive Ingester

**Summary:**
A Thanos Receive Ingester that is running (`up == 1`) has not performed any object storage uploads (`thanos_shipper_uploads_total` has not increased) in the last 4 hours, sustained for 4 hours.

**Impact:**
TSDB blocks accumulated in the ingester's WAL and head are not being shipped to object storage. If the ingester pod is restarted without a successful upload, up to 4+ hours of metrics data will be permanently lost. This is a **data loss risk**.

**Alert Expression:**
```promql
(up{job=~"thanos-receive-ingester.*"} - 1)
+ on (namespace, job, instance)
(
    sum by (namespace, job, instance) (
      increase(thanos_shipper_uploads_total{job=~"thanos-receive-ingester.*"}[4h])
    )
  == 0
)
```

**Steps:**

> **Warning:** Do NOT restart affected ingester pods until the upload issue is resolved. Restarting without a successful upload risks permanent data loss.

- **In Grafana**, check upload rate per ingester instance:
   ```promql
   sum by (namespace, job, instance) (
     rate(thanos_shipper_uploads_total{job=~"thanos-receive-ingester.*", namespace="rhobs-production"}[30m])
   )
   ```

- **Check ingester logs** for shipper/upload errors:
   ```bash
   kubectl logs -n rhobs-production <receive-ingester-pod> | grep -iE "shipper|upload|object|bucket|error"
   ```

- **Verify object storage connectivity** from the ingester pod:
   ```bash
   kubectl exec -n rhobs-production <receive-ingester-pod> -- env | grep -i objstore
   ```
   Check the secret referenced in the ThanosReceive CR is valid.

- **Check object storage operation failures** for the ingester:
   ```promql
   sum by (namespace, job, instance) (
     rate(thanos_objstore_bucket_operation_failures_total{job=~"thanos-receive-ingester.*", namespace="rhobs-production"}[5m])
   )
   ```

- **Check object storage operation failure breakdown by operation type** — distinguishes whether the failure is on reads (iterating blocks) or writes (uploading new blocks):
   ```promql
   sum by (namespace, job, instance, operation) (
     rate(thanos_objstore_bucket_operation_failures_total{job=~"thanos-receive-ingester.*", namespace="rhobs-production"}[5m])
   )
   ```

- **Check time since last successful upload** — the **Time Since Last Block Upload** panel in the [Receive dashboard](https://grafana.app-sre.devshift.net/d/thanos-receive-overview/thanos-receive-overview) tracks this directly. Values over 2h indicate a stuck upload path (alert threshold is 4h):
   ```promql
   time() - max by (namespace, job, instance) (
     thanos_objstore_bucket_last_successful_upload_time{job=~"thanos-receive-ingester.*", namespace="rhobs-production"}
   )
   ```

- **Check TSDB head block state** — if no blocks are ready to ship, no upload occurs:
   ```promql
   sum by (namespace, job, instance) (prometheus_tsdb_head_chunks{job=~"thanos-receive-ingester.*", namespace="rhobs-production"})
   ```

- **After fixing the root cause**, confirm uploads resume before any maintenance on the pod:
   ```promql
   increase(thanos_shipper_uploads_total{job=~"thanos-receive-ingester.*", namespace="rhobs-production"}[15m])
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs/exec` on receive-ingester pods (read-only until upload resumes)
- `kubectl get` on secrets referenced by `ThanosReceive` CR
- Grafana access

---

## ThanosReceiveLimitsConfigReloadFailure

**Severity:** `medium` | **For:** 5m | **Component:** Thanos Receive Router

**Summary:**
The `thanos_receive_limits_config_reload_err_total` counter has increased in the last 5 minutes, meaning the Thanos Receive Router failed to reload its tenant limits configuration.

**Impact:**
Tenant rate limits and head series quotas are running on a stale configuration. New tenants may not have limits applied, or recently updated limits will not take effect. Tenants could exceed their intended quotas.

**Alert Expression:**
```promql
sum by (namespace, job) (
  increase(thanos_receive_limits_config_reload_err_total{job=~"thanos-receive-router.*"}[5m])
) > 0
```

**Steps:**

- **In Grafana**, check how many reload errors have occurred:
   ```promql
   sum by (namespace, job) (
     increase(thanos_receive_limits_config_reload_err_total{job=~"thanos-receive-router.*", namespace="rhobs-production"}[5m])
   )
   ```

- **Read router logs** for the specific parsing or file error:
   ```bash
   kubectl logs -n rhobs-production <receive-router-pod> | grep -iE "limits|config|reload|error"
   ```

- **Locate and validate the limits ConfigMap:**
   ```bash
   kubectl get configmap -n rhobs-production | grep limits
   kubectl get configmap -n rhobs-production <limits-configmap> -o yaml
   ```
   The limits file must be valid YAML with correct Thanos receive limits schema.

- **Check the ThanosReceive CR** to see which ConfigMap is referenced for limits:
   ```bash
   kubectl describe thanosreceive <name> -n rhobs-production
   ```

- **After fixing the ConfigMap**, confirm the reload succeeds by watching the counter stop incrementing.

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs` on receive-router pods
- `kubectl get/edit` on ConfigMaps
- `kubectl describe` on `ThanosReceive` CRs
- Grafana access

---

## ThanosReceiveLimitsHighMetaMonitoringQueriesFailureRate

**Severity:** `medium` | **For:** 5m | **Component:** Thanos Receive Router

**Summary:**
More than 20% of meta-monitoring queries (used to count active head series per tenant) are failing. Meta-monitoring queries run every 15 seconds (20 times per 5 minutes), and this alert fires when the failure rate exceeds 20%.

**Impact:**
The router cannot accurately track per-tenant head series counts. Tenant head series limits (`head_series_limit`) cannot be enforced. Tenants may exceed their configured series limit, which increases ingester memory pressure and degrades overall ingestion performance.

**Alert Expression:**
```promql
(
    sum by (namespace, job) (
      increase(thanos_receive_metamonitoring_failed_queries_total{job=~"thanos-receive-router.*"}[5m])
    )
  /
    20
)
* 100 > 20
```

**Steps:**

- **In Grafana**, check the meta monitoring failure rate:
   ```promql
   (
     sum by (namespace, job) (
       increase(thanos_receive_metamonitoring_failed_queries_total{job=~"thanos-receive-router.*", namespace="rhobs-production"}[5m])
     ) / 20
   ) * 100
   ```

- **Read router logs** for meta-monitoring query errors:
   ```bash
   kubectl logs -n rhobs-production <receive-router-pod> | grep -iE "meta|monitor|query|error"
   ```

- **Check which Prometheus endpoint is configured for meta-monitoring** in the ThanosReceive CR:
   ```bash
   kubectl describe thanosreceive <name> -n rhobs-production | grep -A5 -i "meta"
   ```

- **Verify the meta-monitoring Prometheus target is reachable** from the router pod:
   ```bash
   kubectl exec -n rhobs-production <receive-router-pod> -- wget -q -O- http://<prometheus-endpoint>/api/v1/query?query=up
   ```

- **Check if Prometheus itself is healthy:**
   ```bash
   kubectl get pods -n rhobs-production | grep prometheus
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl exec/logs` on receive-router pods
- `kubectl describe` on `ThanosReceive` CRs
- Grafana access

---

## ThanosReceiveTenantLimitedByHeadSeries

**Severity:** `medium` | **For:** 5m | **Component:** Thanos Receive Router

**Summary:**
A specific tenant's remote write requests are being rejected because they have reached their configured `head_series_limit`. The `thanos_receive_head_series_limited_requests_total` counter is increasing.

**Impact:**
The affected tenant's new time series are being dropped. Existing series continue to be written, but any new label combinations are rejected. From the tenant's Prometheus perspective, remote write errors will appear for the rejected series.

**Alert Expression:**
```promql
sum by (namespace, job, tenant) (
  increase(thanos_receive_head_series_limited_requests_total{job=~"thanos-receive-router.*"}[5m])
) > 0
```

**Steps:**

- **Identify the affected tenant** from the alert label `tenant=`.

- **In Grafana**, check which tenants are being limited and how often:
   ```promql
   sum by (namespace, job, tenant) (
     increase(thanos_receive_head_series_limited_requests_total{job=~"thanos-receive-router.*", namespace="rhobs-production"}[5m])
   )
   ```

- **Check the tenant's current active head series count:**
   ```promql
   sum by (namespace, tenant) (thanos_receive_head_series{namespace="rhobs-production", tenant="<tenant>"})
   ```

- **Find the configured limit** in the limits ConfigMap:
   ```bash
   kubectl get configmap -n rhobs-production <limits-configmap> -o yaml | grep -A5 "<tenant>"
   ```

- **Investigate the tenant's cardinality** — high cardinality is typically caused by:
   - Unbounded label values (e.g., request IDs, UUIDs in labels)
   - Too many unique label combinations from scrape targets
   - Misconfigured relabeling

- **Contact the tenant team** to reduce cardinality. Do not increase the limit without understanding the root cause — unconstrained cardinality growth will exhaust ingester memory.

- **If the limit increase is justified**, update the limits ConfigMap with the new `head_series_limit` value and verify reload succeeds.

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl get/edit` on limits ConfigMaps
- Grafana access (for cardinality analysis)
- Communication channel to the tenant team

---

## Thanos Store Alerts

> **Grafana Dashboard:** [Thanos / Store Gateway / Overview](https://grafana.app-sre.devshift.net/d/thanos-store-overview/thanos-store-overview) — select the `<cluster>-prometheus` datasource from the dropdown.

---

## ThanosStoreGrpcErrorRate

**Severity:** `medium` | **For:** 5m | **Component:** Thanos Store Gateway

**Summary:**
More than 5% of gRPC server requests handled by the Thanos Store Gateway are returning error codes (`Unknown`, `Internal`, `Unavailable`, `DataLoss`, `DeadlineExceeded`), sustained for 5 minutes.

**Impact:**
Thanos Query instances using this Store Gateway receive errors when fetching historical data. Queries that require data beyond the Prometheus retention window will return partial results or fail entirely.

**Alert Expression:**
```promql
(
    sum by (namespace, job) (
      rate(
        grpc_server_handled_total{grpc_code=~"Unknown|ResourceExhausted|Internal|Unavailable|DataLoss|DeadlineExceeded",job=~"thanos-store.*"}[5m]
      )
    )
  /
    sum by (namespace, job) (rate(grpc_server_started_total{job=~"thanos-store.*"}[5m]))
)
* 100 > 5
```

**Steps:**

- **In Grafana**, break down gRPC errors by code to understand the failure type:
   ```promql
   sum by (namespace, job, grpc_code, grpc_method) (
     rate(grpc_server_handled_total{grpc_code!="OK",job=~"thanos-store.*", namespace="rhobs-production"}[5m])
   )
   ```
   - `DeadlineExceeded` → slow object storage operations
   - `ResourceExhausted` → memory/chunk pool exhausted
   - `Internal`/`DataLoss` → corrupted blocks or disk issues

- **Check object storage operation latency** alongside gRPC errors — slow storage is the most common cause:
   ```promql
   histogram_quantile(0.99, sum by (namespace, job, le) (
     rate(thanos_objstore_bucket_operation_duration_seconds_bucket{job=~"thanos-store.*", namespace="rhobs-production"}[5m])
   ))
   ```

- **Check object storage operation failures:**
   ```promql
   sum by (namespace, job, operation) (
     rate(thanos_objstore_bucket_operation_failures_total{job=~"thanos-store.*", namespace="rhobs-production"}[5m])
   )
   ```

- **Read Store Gateway logs** for the specific error:
   ```bash
   kubectl logs -n rhobs-production <store-pod> | grep -iE "grpc|error|chunk|block"
   ```

- **Check store pod resource usage** — chunk pool exhaustion causes `ResourceExhausted`:
   ```bash
   kubectl top pod -n rhobs-production -l app.kubernetes.io/name=thanos-store
   ```

- **Check block index cache** — a cold or absent cache increases object storage pressure:
   ```promql
   sum by (namespace, job) (thanos_store_index_cache_hits_total{job=~"thanos-store.*", namespace="rhobs-production"})
   ```

- **Check block load error rate** — the **Block Load Errors** panel in the [Store Gateway dashboard](https://grafana.app-sre.devshift.net/d/thanos-store-overview/thanos-store-overview) shows whether blocks are failing to load (corrupted or inaccessible blocks will cause gRPC errors on every affected series request):
   ```promql
   (
     sum by (namespace, job) (rate(thanos_bucket_store_block_load_failures_total{job=~"thanos-store.*", namespace="rhobs-production"}[5m]))
     /
     sum by (namespace, job) (rate(thanos_bucket_store_block_loads_total{job=~"thanos-store.*", namespace="rhobs-production"}[5m]))
   ) * 100
   ```

- **Check series fetch duration** — the **Query Operation Durations** / **Get All Series Duration** panels show how long the store is spending per series fetch from object storage:
   ```promql
   histogram_quantile(0.99, sum by (namespace, job, le) (
     rate(thanos_bucket_store_series_get_all_duration_seconds_bucket{job=~"thanos-store.*", namespace="rhobs-production"}[5m])
   ))
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs/top` on store pods
- Grafana access

---

## ThanosStoreBucketHighOperationFailures

**Severity:** `medium` | **For:** 15m | **Component:** Thanos Store Gateway

**Summary:**
More than 5% of object storage (bucket) operations by the Thanos Store Gateway are failing, sustained for 15 minutes.

**Impact:**
The Store Gateway cannot reliably fetch blocks or their metadata from object storage. Historical data queries will fail or return incomplete results. Block synchronization will also fail, meaning newly uploaded blocks are not indexed by the store.

**Alert Expression:**
```promql
(
    sum by (namespace, job) (
      rate(thanos_objstore_bucket_operation_failures_total{job=~"thanos-store.*"}[5m])
    )
  /
    sum by (namespace, job) (rate(thanos_objstore_bucket_operations_total{job=~"thanos-store.*"}[5m]))
)
* 100 > 5
```

**Steps:**

- **In Grafana**, check failure rate by operation type:
   ```promql
   sum by (namespace, job, operation) (
     rate(thanos_objstore_bucket_operation_failures_total{job=~"thanos-store.*", namespace="rhobs-production"}[5m])
   )
   ```
   Operations: `get`, `iter`, `exists`, `get_range`, `attributes`, `upload`, `delete`.

- **Read Store Gateway logs** for the specific storage error:
   ```bash
   kubectl logs -n rhobs-production <store-pod> | grep -iE "bucket|s3|gcs|azure|error|denied|credential"
   ```

- **Verify the object storage secret** is valid and not expired:
   ```bash
   kubectl get secret -n rhobs-production <objstore-secret-name>
   ```
   Check the referenced credentials against your cloud provider console.

- **Check cloud provider status** for the object storage service — regional outages or throttling will appear as sustained failures.

- **Verify IAM permissions** — the Store Gateway needs at minimum: `GetObject`, `ListBucket`, `HeadObject` on the bucket.

- **Check for network connectivity** issues between store pods and the storage endpoint:
   ```bash
   kubectl exec -n rhobs-production <store-pod> -- nslookup <storage-bucket-endpoint>
   ```

- **Check block metadata sync failures and block load errors** — sustained bucket failures cause block sync to lag and block loads to fail. Check the **Sync Meta Errors** panel in the [Compact dashboard](https://grafana.app-sre.devshift.net/d/thanos-compact-overview/thanos-compact-overview) and **Block Load Errors** in the [Store Gateway dashboard](https://grafana.app-sre.devshift.net/d/thanos-store-overview/thanos-store-overview):
   ```promql
   sum by (namespace, job) (rate(thanos_blocks_meta_sync_failures_total{job=~"thanos-store.*", namespace="rhobs-production"}[5m]))
   ```
   ```promql
   (
     sum by (namespace, job) (rate(thanos_bucket_store_block_load_failures_total{job=~"thanos-store.*", namespace="rhobs-production"}[5m]))
     /
     sum by (namespace, job) (rate(thanos_bucket_store_block_loads_total{job=~"thanos-store.*", namespace="rhobs-production"}[5m]))
   ) * 100
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs/exec` on store pods
- `kubectl get` on secrets
- Cloud provider console (object storage health, IAM)
- Grafana access

---

## ThanosStoreObjstoreOperationLatencyHigh

**Severity:** `medium` | **For:** 10m | **Component:** Thanos Store Gateway

**Summary:**
The 99th percentile latency for object storage operations by the Thanos Store Gateway exceeds 7 seconds, sustained for 10 minutes.

**Impact:**
All historical data queries are slow. The Store Gateway's gRPC responses to Thanos Query are delayed, which manifests as high query latency end-to-end. May trigger [ThanosQueryInstantLatencyHigh](#thanosqueryinstantlatencyhigh) as a downstream effect.

**Alert Expression:**
```promql
histogram_quantile(
  0.99,
  sum by (namespace, job, le) (
    rate(thanos_objstore_bucket_operation_duration_seconds_bucket{job=~"thanos-store.*"}[5m])
  )
)
> 7
and
sum by (namespace, job) (
  rate(thanos_objstore_bucket_operation_duration_seconds_count{job=~"thanos-store.*"}[5m])
) > 0
```

**Steps:**

- **In Grafana**, plot latency by operation type to identify which operations are slow:
   ```promql
   histogram_quantile(0.99, sum by (namespace, job, operation, le) (
     rate(thanos_objstore_bucket_operation_duration_seconds_bucket{job=~"thanos-store.*", namespace="rhobs-production"}[5m])
   ))
   ```

- **Correlate with query load** — high query throughput increases concurrent object storage calls:
   ```promql
   sum by (namespace, job) (rate(grpc_server_started_total{job=~"thanos-store.*", namespace="rhobs-production"}[5m]))
   ```

- **Check block index cache hit rate** — a low hit rate forces more object storage fetches:
   ```promql
   sum by (namespace, job) (rate(thanos_store_index_cache_hits_total{job=~"thanos-store.*", namespace="rhobs-production"}[5m]))
   /
   sum by (namespace, job) (rate(thanos_store_index_cache_requests_total{job=~"thanos-store.*", namespace="rhobs-production"}[5m]))
   ```

- **Check the cloud provider's object storage metrics** for the bucket — look for throttling (429s) or high server-side latency.

- **Verify the store is co-located with the bucket** — cross-region access dramatically increases latency. Check bucket region vs. cluster region.

- **Check store pod resource usage** — CPU throttling on the store pod adds to effective latency:
   ```bash
   kubectl top pod -n rhobs-production -l app.kubernetes.io/name=thanos-store
   ```

- **Check series fetch duration** — the **Query Operation Durations** / **Get All Series Duration** panels in the [Store Gateway dashboard](https://grafana.app-sre.devshift.net/d/thanos-store-overview/thanos-store-overview) break down where the store spends its time per query:
   ```promql
   histogram_quantile(0.99, sum by (namespace, job, le) (
     rate(thanos_bucket_store_series_get_all_duration_seconds_bucket{job=~"thanos-store.*", namespace="rhobs-production"}[5m])
   ))
   ```

- **Check series merge duration** — the **Merge Durations** panel shows how long merging results from multiple blocks takes. High values indicate too many overlapping series across blocks (compaction lag → more blocks to merge per query):
   ```promql
   histogram_quantile(0.99, sum by (namespace, job, le) (
     rate(thanos_bucket_store_series_merge_duration_seconds_bucket{job=~"thanos-store.*", namespace="rhobs-production"}[5m])
   ))
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl top` on store pods
- Cloud provider console (object storage metrics)
- Grafana access

---

## Thanos Rule Alerts

> **Grafana Dashboard:** [Thanos / Ruler / Overview](https://grafana.app-sre.devshift.net/d/thanos-ruler-overview/thanos-ruler-overview) — select the `<cluster>-prometheus` datasource from the dropdown.

---

## ThanosRuleQueueIsDroppingAlerts

**Severity:** `high` | **For:** 5m | **Component:** Thanos Rule

**Summary:**
The Thanos Rule alert queue is dropping alerts — `thanos_alert_queue_alerts_dropped_total` is increasing. This means alerts evaluated by the ruler are being discarded before they can be sent to Alertmanager.

**Impact:**
**Critical:** Firing alerts are silently dropped. Alertmanager never receives them, so no notifications (PagerDuty, Slack, email) are sent. The on-call team is blind to real incidents during this period.

**Alert Expression:**
```promql
sum by (namespace, job, instance) (
  rate(thanos_alert_queue_alerts_dropped_total{job=~"thanos-ruler.*"}[5m])
) > 0
```

**Steps:**

- **In Grafana**, confirm the drop rate and when it started:
   ```promql
   sum by (namespace, job, instance) (
     rate(thanos_alert_queue_alerts_dropped_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m])
   )
   ```

- **Check alert queue throughput** — the **Alert Queue** row in the [Ruler dashboard](https://grafana.app-sre.devshift.net/d/thanos-ruler-overview/thanos-ruler-overview) shows pushed vs. popped rates. If the pushed rate significantly exceeds the popped rate, the queue is filling:
   ```promql
   sum by (namespace, job) (rate(thanos_alert_queue_alerts_pushed_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m]))
   ```
   ```promql
   sum by (namespace, job) (rate(thanos_alert_queue_alerts_popped_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m]))
   ```
   Also watch the **Dropped Percentage** panel — any non-zero rate means alerts are being lost:
   ```promql
   sum by (namespace, job) (rate(thanos_alert_queue_alerts_dropped_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m]))
   ```

- **Verify Alertmanager is reachable** from the ruler pod:
   ```bash
   kubectl exec -n rhobs-production <ruler-pod> -- wget -q -O- http://<alertmanager-service>:9093/-/healthy
   ```

- **Check Alertmanager pod status:**
   ```bash
   kubectl get pods -n rhobs-production | grep alertmanager
   ```

- **Read ruler logs** for Alertmanager send errors or queue overflow messages:
   ```bash
   kubectl logs -n rhobs-production <ruler-pod> | grep -iE "queue|drop|alertmanager|error"
   ```

- **Check if Alertmanager DNS resolution is failing** (see [ThanosRuleAlertmanagerHighDNSFailures](#thanosrulealertmanagerhighdnsfailures)):
   ```promql
   sum by (namespace, job, instance) (
     rate(thanos_rule_alertmanagers_dns_failures_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m])
   )
   ```

- **Check ruler resource usage** — OOM or CPU starvation can cause queue management to fail:
   ```bash
   kubectl top pod -n rhobs-production -l app.kubernetes.io/name=thanos-ruler
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl exec/logs/top` on ruler pods
- `kubectl get` on Alertmanager pods
- Grafana access

---

## ThanosRuleSenderIsFailingAlerts

**Severity:** `high` | **For:** 5m | **Component:** Thanos Rule

**Summary:**
The Thanos Rule alert sender is dropping alerts — `thanos_alert_sender_alerts_dropped_total` is increasing. This means alerts have been queued but the sender cannot deliver them to Alertmanager.

**Impact:**
**Critical:** Alertmanager is not receiving any alerts from this ruler. Notifications are not being sent. This is a complete failure of the alerting pipeline for all rules managed by this ruler instance.

**Alert Expression:**
```promql
sum by (namespace, job, instance) (
  rate(thanos_alert_sender_alerts_dropped_total{job=~"thanos-ruler.*"}[5m])
) > 0
```

**Steps:**

- **In Grafana**, confirm the sender drop rate:
   ```promql
   sum by (namespace, job, instance) (
     rate(thanos_alert_sender_alerts_dropped_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m])
   )
   ```

- **Test Alertmanager connectivity** directly from the ruler pod:
   ```bash
   kubectl exec -n rhobs-production <ruler-pod> -- wget -q -O- http://<alertmanager-service>:9093/-/healthy
   kubectl exec -n rhobs-production <ruler-pod> -- wget -q -O- http://<alertmanager-service>:9093/api/v2/status
   ```

- **Check Alertmanager pod and service health:**
   ```bash
   kubectl get pods -n rhobs-production | grep alertmanager
   kubectl get svc,endpoints -n rhobs-production | grep alertmanager
   ```

- **Read ruler logs** for send failure details:
   ```bash
   kubectl logs -n rhobs-production <ruler-pod> | grep -iE "sender|alertmanager|send.*fail|error"
   ```

- **Verify the Alertmanager URL** configured in the ThanosRuler CR is correct:
   ```bash
   kubectl describe thanosruler.monitoring.thanos.io <name> -n rhobs-production | grep -A5 -i alertmanager
   ```

- **Check network policies** allow traffic from ruler to Alertmanager on port 9093:
   ```bash
   kubectl get networkpolicy -n rhobs-production
   ```

- **Check for authentication errors** — if Alertmanager requires auth tokens, verify they're configured correctly in the ruler.

- **In Grafana**, check the **Alerts Dropped** and **Alert Sending Errors** panels — the dashboard shows the error ratio (`sender_errors / alerts_sent`) and latency percentiles:
   ```promql
   sum by (namespace, job) (rate(thanos_alert_sender_errors_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m]))
   /
   sum by (namespace, job) (rate(thanos_alert_sender_alerts_sent_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m]))
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl exec/logs/describe` on ruler pods
- `kubectl get` on Alertmanager pods, services, endpoints, NetworkPolicies
- `kubectl describe` on `ThanosRuler` CRs
- Grafana access

---

## ThanosRuleHighRuleEvaluationFailures

**Severity:** `high` | **For:** 5m | **Component:** Thanos Rule

**Summary:**
More than 5% of rule evaluations by Thanos Rule are failing (`prometheus_rule_evaluation_failures_total` / `prometheus_rule_evaluations_total`), sustained for 5 minutes.

**Impact:**
Recording rules producing derived metrics are failing — those metrics will have gaps or stale values. Alerting rules that fail to evaluate will not fire, creating blind spots in monitoring coverage.

**Alert Expression:**
```promql
(
    sum by (namespace, job, instance) (
      rate(prometheus_rule_evaluation_failures_total{job=~"thanos-ruler.*"}[5m])
    )
  /
    sum by (namespace, job, instance) (
      rate(prometheus_rule_evaluations_total{job=~"thanos-ruler.*"}[5m])
    )
)
* 100 > 5
```

**Steps:**

- **In Grafana**, check the **Rule Group Evaluation Rate** and **Rule Group Evaluation Failures** panels — they break failures down by `rule_group` and `strategy`:
   ```promql
   (
     sum by (namespace, job, instance) (rate(prometheus_rule_evaluation_failures_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m]))
     /
     sum by (namespace, job, instance) (rate(prometheus_rule_evaluations_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m]))
   ) * 100
   ```
   Drill down by rule group and strategy to find the specific offender:
   ```promql
   sum by (namespace, job, rule_group, strategy) (
     rate(prometheus_rule_evaluation_failures_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m])
   )
   ```

- **Read ruler logs** to find which rule groups are failing and why:
   ```bash
   kubectl logs -n rhobs-production <ruler-pod> | grep -iE "eval.*fail|group.*error|query.*fail"
   ```

- **Common failure causes:**
   - **Query endpoint unreachable:** The ruler cannot reach Thanos Query to evaluate rules. Check:
     ```promql
     sum by (namespace, job, instance) (rate(thanos_rule_query_apis_dns_failures_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m]))
     ```
   - **Query timeout:** Complex rules or slow stores cause evaluation timeouts — look for `context deadline exceeded` in logs.
   - **Invalid PromQL:** A recently deployed rule has a syntax error — check config reload status:
     ```promql
     avg by (namespace, job, instance) (thanos_rule_config_last_reload_successful{job=~"thanos-ruler.*", namespace="rhobs-production"})
     ```

- **Verify Thanos Query endpoint is healthy:**
   ```bash
   kubectl exec -n rhobs-production <ruler-pod> -- wget -q -O- http://<thanos-query-service>:9090/-/healthy
   ```

- **Check ruler resource usage:**
   ```bash
   kubectl top pod -n rhobs-production -l app.kubernetes.io/name=thanos-ruler
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl exec/logs/top` on ruler pods
- Grafana access

---

## ThanosRuleHighRuleEvaluationWarnings

**Severity:** `high` | **For:** 15m | **Component:** Thanos Rule

**Summary:**
Thanos Rule is producing evaluation warnings — `thanos_rule_evaluation_with_warnings_total` is increasing — sustained for 15 minutes. Warnings occur when a rule evaluates successfully but with non-fatal issues (e.g., partial data, stale markers).

**Impact:**
Rules are evaluating but may be acting on incomplete data. Alerting rules could produce false negatives (not firing when they should) or false positives due to missing time series or partial query results.

**Alert Expression:**
```promql
sum by (namespace, job, instance) (
  rate(thanos_rule_evaluation_with_warnings_total{job=~"thanos-ruler.*"}[5m])
) > 0
```

**Steps:**

- **In Grafana**, check warning rate over time:
   ```promql
   sum by (namespace, job, instance) (
     rate(thanos_rule_evaluation_with_warnings_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m])
   )
   ```

- **Read ruler logs** for warning messages — Thanos includes the warning text in structured log output:
   ```bash
   kubectl logs -n rhobs-production <ruler-pod> | grep -iE "warn|partial|stale|missing"
   ```

- **Common warning causes:**
   - **Partial results from stores:** One or more store backends returned partial data. Check store health.
   - **Stale series:** A series expected by a rule has stopped being scraped. Check scrape targets.
   - **No data for a subquery range:** A recording rule references a metric with gaps.

- **Check if Store Gateways are returning partial results:**
   ```promql
   sum by (namespace, job) (rate(thanos_store_series_data_fetched_sum{job=~"thanos-store.*", namespace="rhobs-production"}[5m]))
   ```

- **Check the Thanos Query `/api/v1/query` response for `warnings` field** by running a sample query manually through Grafana Explore.

- **Review the specific rule groups** generating warnings — look for rules referencing metrics that may have been removed or renamed.

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs` on ruler pods
- Grafana access (Explore view for manual rule query testing)

---

## ThanosRuleRuleEvaluationLatencyHigh

**Severity:** `medium` | **For:** 5m | **Component:** Thanos Rule

**Summary:**
One or more rule groups are taking longer to evaluate than their configured evaluation interval. Specifically, `prometheus_rule_group_last_duration_seconds` exceeds `prometheus_rule_group_interval_seconds` for the same group.

**Impact:**
Rule groups that take longer than their interval to evaluate will fall behind schedule. This means evaluations are queued and delayed, which can cause alerting rules to fire late (missing SLA-critical notification windows) and recording rules to produce delayed metrics.

**Alert Expression:**
```promql
sum by (namespace, job, instance, rule_group) (
  prometheus_rule_group_last_duration_seconds{job=~"thanos-ruler.*"}
)
>
sum by (namespace, job, instance, rule_group) (
  prometheus_rule_group_interval_seconds{job=~"thanos-ruler.*"}
)
```

**Steps:**

- **In Grafana**, check the **Rule Group Evaluations Too Slow** panel — it directly shows groups where `last_duration_seconds > interval_seconds`:
   ```promql
   sum by (namespace, job, rule_group) (
     prometheus_rule_group_last_duration_seconds{job=~"thanos-ruler.*", namespace="rhobs-production"}
   )
   >
   sum by (namespace, job, rule_group) (
     prometheus_rule_group_interval_seconds{job=~"thanos-ruler.*", namespace="rhobs-production"}
   )
   ```
   Also check the **Rule Group Evaluations Missed** panel — missed iterations pile up when groups consistently over-run:
   ```promql
   sum by (namespace, job, rule_group, strategy) (
     rate(prometheus_rule_group_iterations_missed_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m])
   )
   ```

- **Check Thanos Query latency** for the queries backing those slow rules:
   ```promql
   histogram_quantile(0.99, sum by (namespace, job, le) (
     rate(http_request_duration_seconds_bucket{handler="query",job=~"thanos-query.*", namespace="rhobs-production"}[5m])
   ))
   ```

- **Read ruler logs** for slow group evaluation messages:
   ```bash
   kubectl logs -n rhobs-production <ruler-pod> | grep -iE "group|duration|slow|latency"
   ```

- **Optimize slow rule groups:**
   - Break large groups into smaller ones so they can evaluate in parallel
   - Replace complex range queries with recording rules
   - Increase the rule group interval if the current interval is unrealistically tight

- **Check ruler resource usage** — CPU throttling causes all evaluations to slow down:
   ```bash
   kubectl top pod -n rhobs-production -l app.kubernetes.io/name=thanos-ruler
   ```
   If CPU is constrained, increase limits in the `ThanosRuler` CR.

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl top` on ruler pods
- `kubectl edit` on `ThanosRuler` CR (if resource adjustment needed)
- Grafana access
- Access to rule file configuration (to optimize rules)

---

## ThanosRuleGrpcErrorRate

**Severity:** `medium` | **For:** 5m | **Component:** Thanos Rule

**Summary:**
More than 5% of gRPC server requests handled by Thanos Rule are returning error codes (`Unknown`, `ResourceExhausted`, `Internal`, `Unavailable`, `DataLoss`, `DeadlineExceeded`), sustained for 5 minutes.

**Impact:**
Clients querying Thanos Rule's gRPC API (e.g., for rule group status) are receiving errors. This can affect observability tooling and dashboards that display rule evaluation status.

**Alert Expression:**
```promql
(
    sum by (namespace, job, instance) (
      rate(
        grpc_server_handled_total{grpc_code=~"Unknown|ResourceExhausted|Internal|Unavailable|DataLoss|DeadlineExceeded",job=~"thanos-ruler.*"}[5m]
      )
    )
  /
    sum by (namespace, job, instance) (rate(grpc_server_started_total{job=~"thanos-ruler.*"}[5m]))
)
* 100 > 5
```

**Steps:**

- **In Grafana**, break down gRPC errors by code and method:
   ```promql
   sum by (namespace, job, instance, grpc_code, grpc_method) (
     rate(grpc_server_handled_total{grpc_code!="OK",job=~"thanos-ruler.*", namespace="rhobs-production"}[5m])
   )
   ```

- **Read ruler logs** for the gRPC server error context:
   ```bash
   kubectl logs -n rhobs-production <ruler-pod> | grep -iE "grpc|error|unavailable|exhausted"
   ```

- **Check ruler pod resources** — `ResourceExhausted` typically means OOM or goroutine limit:
   ```bash
   kubectl top pod -n rhobs-production -l app.kubernetes.io/name=thanos-ruler
   kubectl describe pod -n rhobs-production <ruler-pod> | grep -A5 "Limits"
   ```

- **Check if the ruler's Thanos Query backend is healthy** — an unhealthy query endpoint causes cascading gRPC failures in rule evaluation which may spill into gRPC server errors.

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs/top/describe` on ruler pods
- Grafana access

---

## ThanosRuleConfigReloadFailure

**Severity:** `high` | **For:** 5m | **Component:** Thanos Rule

**Summary:**
The `thanos_rule_config_last_reload_successful` metric for Thanos Rule is not equal to 1, meaning the last configuration reload failed. Thanos Rule reloads its rule files on SIGHUP or when ConfigMaps change.

**Impact:**
Rule file changes (new alerts, modified recording rules, updated thresholds) are not being applied. The ruler continues running on its last successfully loaded configuration. If the initial load failed, the ruler may have no rules loaded at all.

**Alert Expression:**
```promql
avg by (namespace, job, instance) (thanos_rule_config_last_reload_successful{job=~"thanos-ruler.*"})
!= 1
```

**Steps:**

- **In Grafana**, check the reload success status per instance:
   ```promql
   thanos_rule_config_last_reload_successful{job=~"thanos-ruler.*", namespace="rhobs-production"}
   ```

- **Read ruler logs** for the config reload error:
   ```bash
   kubectl logs -n rhobs-production <ruler-pod> | grep -iE "reload|config|rule.*error|parse"
   ```

- **Understand the operator's rule loading flow:** The operator discovers rules from two sources and generates bucketed ConfigMaps (label: `operator.thanos.io/rule-file=true`) that the ruler mounts:
   - **PrometheusRule CRs** matching `spec.ruleSelector` on the ThanosRuler CR
   - **User ConfigMaps** matching `spec.ruleConfigSelector` on the ThanosRuler CR

- **Check operator-generated rule ConfigMaps** — the operator creates these, not users:
   ```bash
   kubectl get configmap -n rhobs-production -l operator.thanos.io/rule-file=true
   kubectl describe pod -n rhobs-production <ruler-pod> | grep -A10 "Volumes"
   ```

- **Check PrometheusRule syntax** — a bad PrometheusRule will cause the operator to generate invalid YAML, which the ruler then fails to load:
   ```bash
   kubectl get prometheusrule -n rhobs-production
   # Validate a specific rule:
   kubectl get prometheusrule -n rhobs-production <rule-name> -o jsonpath='{.spec}' | python3 -c "import sys,json; json.load(sys.stdin)"
   ```
   Or locally with `promtool check rules`.

- **Check operator logs** for ConfigMap generation errors:
   ```bash
   kubectl logs -n rhobs-production -l control-plane=controller-manager | grep -iE "ThanosRuler|configmap|rule|error"
   ```

- **Check the ThanosRuler CR** selectors to confirm they match your PrometheusRules/ConfigMaps:
   ```bash
   kubectl get thanosruler.monitoring.thanos.io <name> -n rhobs-production -o jsonpath='{.spec.ruleSelector} {.spec.ruleConfigSelector}'
   ```

- **After fixing the source rule**, the operator will regenerate ConfigMaps and the ruler will auto-reload. If still stuck, delete the ruler pod:
   ```bash
   kubectl delete pod -n rhobs-production <ruler-pod>
   ```
   Or via automated actions (if you lack delete permissions):
   ```bash
   automated-actions openshift-workload-delete --cluster <cluster> --namespace rhobs-production --kind Pod --name <ruler-pod>
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs/delete` on ruler pods
- `kubectl get/edit` on PrometheusRule CRs and user ConfigMaps
- `kubectl describe` on `ThanosRuler` CRs
- `kubectl logs` on operator pods
- `promtool` for rule validation

---

## ThanosRuleQueryHighDNSFailures

**Severity:** `medium` | **For:** 15m | **Component:** Thanos Rule

**Summary:**
More than 1% of DNS lookups performed by Thanos Rule for its configured Thanos Query endpoints are failing, sustained for 15 minutes.

**Impact:**
Thanos Rule cannot discover or connect to its Thanos Query backend. Rule evaluations that depend on querying Thanos will fail or use stale endpoint information. This can cascade into rule evaluation failures.

**Alert Expression:**
```promql
(
    sum by (namespace, job, instance) (
      rate(thanos_rule_query_apis_dns_failures_total{job=~"thanos-ruler.*"}[5m])
    )
  /
    sum by (namespace, job, instance) (
      rate(thanos_rule_query_apis_dns_lookups_total{job=~"thanos-ruler.*"}[5m])
    )
)
* 100 > 1
```

**Steps:**

- **In Grafana**, check the DNS failure ratio:
   ```promql
   (
     sum by (namespace, job, instance) (rate(thanos_rule_query_apis_dns_failures_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m]))
     /
     sum by (namespace, job, instance) (rate(thanos_rule_query_apis_dns_lookups_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m]))
   ) * 100
   ```

- **Read ruler logs** for the hostname failing DNS resolution:
   ```bash
   kubectl logs -n rhobs-production <ruler-pod> | grep -iE "dns|resolve|lookup|query.*api"
   ```

- **Test DNS from the ruler pod:**
   ```bash
   kubectl exec -n rhobs-production <ruler-pod> -- nslookup thanos-query-rhobs.rhobs-production.svc.cluster.local
   ```

- **Verify the Thanos Query Service exists:**
   ```bash
   kubectl get svc -n rhobs-production | grep query
   ```

- **Check the ThanosRuler CR and how the operator discovers query endpoints** — the operator auto-discovers Thanos Query services labeled `operator.thanos.io/query-api=true` + `app.kubernetes.io/part-of=thanos`. If the ThanosQuery CR exists, the operator labels its service automatically:
   ```bash
   kubectl get svc -n rhobs-production -l operator.thanos.io/query-api=true,app.kubernetes.io/part-of=thanos
   kubectl describe thanosruler.monitoring.thanos.io <name> -n rhobs-production | grep -A5 -i "query"
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl exec/logs/describe` on ruler pods
- `kubectl get` on services and `ThanosRuler` CRs
- Grafana access

---

## ThanosRuleAlertmanagerHighDNSFailures

**Severity:** `medium` | **For:** 15m | **Component:** Thanos Rule

**Summary:**
More than 1% of DNS lookups performed by Thanos Rule for its configured Alertmanager endpoints are failing, sustained for 15 minutes.

**Impact:**
Thanos Rule cannot resolve the Alertmanager service address. If the ruler cannot connect to Alertmanager, firing alerts will queue up in the ruler and eventually be dropped — see [ThanosRuleQueueIsDroppingAlerts](#thanosrulequeueisdroppingalerts). Act quickly to prevent alert delivery loss.

**Alert Expression:**
```promql
(
    sum by (namespace, job, instance) (
      rate(thanos_rule_alertmanagers_dns_failures_total{job=~"thanos-ruler.*"}[5m])
    )
  /
    sum by (namespace, job, instance) (
      rate(thanos_rule_alertmanagers_dns_lookups_total{job=~"thanos-ruler.*"}[5m])
    )
)
* 100 > 1
```

**Steps:**

- **In Grafana**, check the Alertmanager DNS failure ratio:
   ```promql
   (
     sum by (namespace, job, instance) (rate(thanos_rule_alertmanagers_dns_failures_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m]))
     /
     sum by (namespace, job, instance) (rate(thanos_rule_alertmanagers_dns_lookups_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m]))
   ) * 100
   ```

- **Read ruler logs** for the specific Alertmanager hostname failing:
   ```bash
   kubectl logs -n rhobs-production <ruler-pod> | grep -iE "alertmanager.*dns|dns.*alertmanager|resolve"
   ```

- **Test DNS resolution** from the ruler pod:
   ```bash
   kubectl exec -n rhobs-production <ruler-pod> -- nslookup alertmanager.rhobs-production.svc.cluster.local
   ```

- **Verify the Alertmanager Service and Endpoints exist:**
   ```bash
   kubectl get svc,endpoints -n rhobs-production | grep alertmanager
   ```

- **Check the Alertmanager URL** configured in the ThanosRuler CR:
   ```bash
   kubectl describe thanosruler.monitoring.thanos.io <name> -n rhobs-production | grep -A3 -i alertmanager
   ```

- **Also check** whether alert queue drops are already occurring (more severe):
   ```promql
   sum by (namespace, job, instance) (
     rate(thanos_alert_queue_alerts_dropped_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m])
   )
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl exec/logs/describe` on ruler pods
- `kubectl get` on Alertmanager services and endpoints
- Grafana access

---

## ThanosRuleNoEvaluationFor10Intervals

**Severity:** `high` | **For:** 5m | **Component:** Thanos Rule

**Summary:**
One or more rule groups have not been evaluated for at least 10 times their configured interval. For example, a group with a 1-minute interval has not evaluated in over 10 minutes.

**Impact:**
All rules in the stuck group are completely suspended. No alerting rules in that group will fire, and recording rules will stop producing metrics. This is effectively a partial outage of the alerting pipeline.

> **Note:** This alert can produce false positives if a rule group is configured but has no rules in it — `prometheus_rule_group_last_evaluation_timestamp_seconds` will be zero in that case.

**Alert Expression:**
```promql
(
    time()
  -
    max by (namespace, job, instance, group) (
      prometheus_rule_group_last_evaluation_timestamp_seconds{job=~"thanos-ruler.*"}
    )
)
>
(
    10
  *
    max by (namespace, job, instance, group) (
      prometheus_rule_group_interval_seconds{job=~"thanos-ruler.*"}
    )
)
```

**Steps:**

- **In Grafana**, identify which group is stuck and how long it has been:
   ```promql
   (
     time()
     - max by (namespace, job, instance, group) (
         prometheus_rule_group_last_evaluation_timestamp_seconds{job=~"thanos-ruler.*", namespace="rhobs-production"}
       )
   )
   ```

- **Read ruler logs** for the stuck group — look for a blocking query or panic:
   ```bash
   kubectl logs -n rhobs-production <ruler-pod> | grep -iE "<group-name>|stuck|block|deadline|panic"
   ```

- **Check if the ruler itself is healthy** — if all groups are stuck, the ruler may be in a bad state:
   ```promql
   sum by (namespace, job, instance) (rate(prometheus_rule_evaluations_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m]))
   ```
   If this is 0, see [ThanosNoRuleEvaluations](#thanosnoruleevaluations).

- **Check Thanos Query response times** — a query taking longer than 10x the interval will cause this:
   ```promql
   histogram_quantile(0.99, sum by (namespace, job, le) (
     rate(http_request_duration_seconds_bucket{handler="query",job=~"thanos-query.*", namespace="rhobs-production"}[5m])
   ))
   ```

- **Restart the ruler pod** if it is confirmed stuck (goroutine leak or deadlock):
   ```bash
   kubectl delete pod -n rhobs-production <ruler-pod>
   ```
   Or via automated actions (if you lack delete permissions):
   ```bash
   automated-actions openshift-workload-delete --cluster <cluster> --namespace rhobs-production --kind Pod --name <ruler-pod>
   ```

- **After restart, verify groups resume evaluation:**
   ```promql
   max by (namespace, job, instance, group) (
     prometheus_rule_group_last_evaluation_timestamp_seconds{job=~"thanos-ruler.*", namespace="rhobs-production"}
   )
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl logs/delete` on ruler pods
- Grafana access

---

## ThanosNoRuleEvaluations

**Severity:** `critical` | **For:** 5m | **Component:** Thanos Rule

**Summary:**
Thanos Rule has rules loaded (`thanos_rule_loaded_rules > 0`) but the total rule evaluation rate has dropped to zero — no evaluations are happening at all.

**Impact:**
**All** alerting and recording rules managed by this ruler instance have stopped evaluating. No alerts will fire, no recording rule metrics will be produced. This is a complete failure of the alerting and recording pipeline for this ruler.

**Alert Expression:**
```promql
sum by (namespace, job, instance) (
  rate(prometheus_rule_evaluations_total{job=~"thanos-ruler.*"}[5m])
)
<= 0
and
sum by (namespace, job, instance) (thanos_rule_loaded_rules{job=~"thanos-ruler.*"}) > 0
```

**Steps:**

- **In Grafana**, confirm zero evaluations with loaded rules:
   ```promql
   sum by (namespace, job, instance) (rate(prometheus_rule_evaluations_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m]))
   ```
   ```promql
   sum by (namespace, job, instance) (thanos_rule_loaded_rules{job=~"thanos-ruler.*", namespace="rhobs-production"})
   ```

- **Check ruler pod status** — it may be in a crash loop or OOMKilled:
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=thanos-ruler
   kubectl describe pod -n rhobs-production <ruler-pod>
   ```

- **Read ruler logs** for a fatal error, panic, or deadlock:
   ```bash
   kubectl logs -n rhobs-production <ruler-pod> --previous
   kubectl logs -n rhobs-production <ruler-pod> | tail -100
   ```

- **Check evaluation failure rate** — if evaluations are happening but all failing (vs. truly zero), the metric may appear as ≤0 due to counter reset:
   ```promql
   sum by (namespace, job, instance) (rate(prometheus_rule_evaluation_failures_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[5m]))
   ```

- **Check if the Thanos Query endpoint is completely unreachable** — if the ruler cannot execute any queries, evaluations will return errors and the rate metric may appear as 0:
   ```bash
   kubectl exec -n rhobs-production <ruler-pod> -- wget -q -O- http://<thanos-query-service>:9090/-/ready
   ```

- **Check ruler resource usage** — OOM can cause the evaluation goroutine pool to be exhausted:
   ```bash
   kubectl top pod -n rhobs-production -l app.kubernetes.io/name=thanos-ruler
   ```

- **Restart the ruler pod** — this is justified given the severity:
   ```bash
   kubectl delete pod -n rhobs-production <ruler-pod>
   ```
   Or via automated actions (if you lack delete permissions):
   ```bash
   automated-actions openshift-workload-delete --cluster <cluster> --namespace rhobs-production --kind Pod --name <ruler-pod>
   ```

- **After restart**, confirm evaluations resume within the first 2 minutes:
   ```promql
   sum by (namespace, job, instance) (rate(prometheus_rule_evaluations_total{job=~"thanos-ruler.*", namespace="rhobs-production"}[2m]))
   ```

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl get/describe/logs/delete/exec/top` on ruler pods
- Grafana access
