# Alertmanager Alerts Runbook

## Table of Contents

- [Alertmanager Alerts Runbook](#alertmanager-alerts-runbook)
  - [Table of Contents](#table-of-contents)
  - [Environment \& Access Information](#environment--access-information)
    - [Multi-Cluster Environment](#multi-cluster-environment)
    - [Prometheus Query Access](#prometheus-query-access)
  - [AlertmanagerFailedReload](#alertmanagerfailedreload)
  - [AlertmanagerMembersInconsistent](#alertmanagermembersinconsistent)
  - [AlertmanagerFailedToSendAlerts](#alertmanagerfailedtosendalerts)
  - [AlertmanagerClusterFailedToSendAlerts](#alertmanagerclusterfailedtosendalerts)
  - [AlertmanagerConfigInconsistent](#alertmanagerconfiginconsistent)
  - [AlertmanagerClusterDown](#alertmanagerclusterdown)
  - [AlertmanagerClusterCrashlooping](#alertmanagerclustercrashlooping)

---

## Environment & Access Information

### Multi-Cluster Environment

This operator runs across multiple private Kubernetes clusters. To troubleshoot alerts:

- **Identify the cluster** from the alert labels (typically `cluster` or `prometheus` label)
- **Access Grafana for metrics:**
  - Navigate to AM dashboards (linked in alert) or use Explore view for PromQL queries
  - URL: `https://grafana.app-sre.devshift.net/?orgId=1`
  - Select datasource: `<cluster>-prometheus` (e.g., `rhobsp02ue1-prometheus`)
- **Access the cluster (for kubectl commands):**
  - Visit the cluster page: `https://visual-app-interface.devshift.net/clusters/<cluster-name>`
  - Follow the sshuttle access instructions provided on the cluster page
  - For `kubectl` access, navigate to `https://oauth-openshift.apps.<cluster_name>.openshiftapps.com/oauth/token/request` (find the cluster name from the console URL on the cluster page in the visual app interface), copy the `oc login` command shown there, and run it — this grants `kubectl` access to the cluster
- **View Configuration:**
  - Configuration repository: `https://gitlab.cee.redhat.com/rhobs/configuration/-/tree/main/resources/clusters/production/<cluster>/metrics/bundle`
  - Contains ThanosOperator and all Thanos CRs (ThanosQuery, ThanosReceive, ThanosRuler, ThanosStore, ThanosCompact)
  - Alertmanager manifests: `https://gitlab.cee.redhat.com/rhobs/configuration/-/tree/main/resources/clusters/production/<cluster>/alertmanager/bundle`
  - Alertmanager config secret: `https://gitlab.cee.redhat.com/service/app-interface/-/blob/master/resources/rhobs/production/<cluster>/alertmanager-config-rhobs-hcp.secret.yaml`

### Prometheus Query Access

For all PromQL metric queries in this runbook:

- **Preferred Method:** Use **Grafana** at `https://grafana.app-sre.devshift.net/?orgId=1`
  - Select the `<cluster>-prometheus` datasource (e.g., `rhobsp01ue1-prometheus`)
  - Use the **Alertmanager / Overview** dashboard for pre-built visualizations
  - Copy-paste PromQL queries from investigative steps for ad-hoc exploration in the Explore view
- **Alternative:** Access Prometheus UI directly via sshuttle tunnel after cluster access
- **Query Placeholders:**
  - Replace `<cluster>` with actual cluster name from alert labels
  - Replace `<namespace>` with `rhobs-production` (the namespace used across all clusters)
  - Replace `<pod>` with the specific pod name (e.g., `alertmanager-0` or `alertmanager-1`)
  - Replace `<integration>` with the integration name from alert labels (e.g., `pagerduty`, `slack`)

---

## AlertmanagerFailedReload

**Severity:** `warning` | **For:** 10m | **Component:** Alertmanager

**Summary:**
Alertmanager failed to reload its configuration from disk. Any changes to alert routing, inhibit rules, receivers, or silences will not take effect until this is resolved.

**Impact:**
The running configuration is stale. New routing rules, inhibit rules, and receiver updates are not in effect. Users may see alerts routed incorrectly or notifications going to outdated destinations. Changes will accumulate until the reload issue is resolved.

**Alert Expression:**
```promql
max_over_time(alertmanager_config_last_reload_successful{job="alertmanager"}[5m]) == 0
```

**Steps:**

- **Identify the failing instance** — query in Grafana Explore (datasource: `<cluster>-prometheus`); note which pod (instance label) returns `0`:
   ```promql
   max_over_time(alertmanager_config_last_reload_successful{job="alertmanager"}[5m])
   ```

- **Check pod logs for reload errors** — look for config parse or validation failures:
   ```bash
   kubectl logs -n rhobs-production alertmanager-0 -c alertmanager | grep -i "error\|failed\|reload" | tail -40
   kubectl logs -n rhobs-production alertmanager-1 -c alertmanager | grep -i "error\|failed\|reload" | tail -40
   ```

- **Verify the configuration secret is present and readable:**
   ```bash
   kubectl get secret alertmanager-config -n rhobs-production
   kubectl get secret alertmanager-config -n rhobs-production -o jsonpath='{.data.alertmanager\.yaml}' | base64 -d | head -60
   ```

- **Check the config currently mounted in the pod** — compare against the secret to detect a propagation lag:
   ```bash
   kubectl exec -n rhobs-production alertmanager-0 -c alertmanager -- cat /etc/alertmanager/config/alertmanager.yaml
   ```

- **Manually trigger a reload** — send a POST to the reload endpoint or SIGHUP to PID 1:
   ```bash
   kubectl exec -n rhobs-production alertmanager-0 -c alertmanager -- wget -qO- -S --post-data='' http://localhost:9093/-/reload
   # or
   kubectl exec -n rhobs-production alertmanager-0 -c alertmanager -- kill -HUP 1
   ```

- **Confirm reload success** — all instances should return `1`:
   ```promql
   max_over_time(alertmanager_config_last_reload_successful{job="alertmanager"}[5m])
   ```

- **Review the configuration in Git** for the affected cluster and fix any invalid YAML or bad references:
   - `https://gitlab.cee.redhat.com/service/app-interface/-/blob/master/resources/rhobs/production/<cluster>/alertmanager-config-rhobs-hcp.secret.yaml`

**Access Required:**
- Cluster access via sshuttle (see [Environment & Access Information](#environment--access-information))
- `kubectl exec` and `kubectl logs` permissions in `rhobs-production`
- Read access to `alertmanager-config` secret

---

## AlertmanagerMembersInconsistent

**Severity:** `warning` | **For:** 15m | **Component:** Alertmanager

**Summary:**
One or more Alertmanager cluster members cannot see all peers. The cluster gossip ring is incomplete.

**Impact:**
Deduplication and silencing across instances may break. An alert that is silenced on one instance may still fire from another. Users may see duplicate notifications or alerts that appear stuck firing.

**Alert Expression:**
```promql
max_over_time(alertmanager_cluster_members{job="alertmanager"}[5m])
  < on(job) group_left
  count by(job) (max_over_time(alertmanager_cluster_members{job="alertmanager"}[5m]))
```

**Steps:**

- **Check the member count per instance** — expected value is `2`; any instance reporting `1` cannot see its peer:
   ```promql
   max_over_time(alertmanager_cluster_members{job="alertmanager"}[5m])
   ```

- **Check pod status** — both `alertmanager-0` and `alertmanager-1` must be `Running` and `Ready`:
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=alertmanager,app.kubernetes.io/instance=observatorium
   ```

- **Check cluster gossip logs** — look for peer connection failures or memberlist errors:
   ```bash
   kubectl logs -n rhobs-production alertmanager-0 -c alertmanager | grep -i "peer\|memberlist\|mesh\|cluster" | tail -30
   kubectl logs -n rhobs-production alertmanager-1 -c alertmanager | grep -i "peer\|memberlist\|mesh\|cluster" | tail -30
   ```

- **Verify the cluster API reports the expected peers** — `cluster.peers` should contain both pod addresses:
   ```bash
   kubectl exec -n rhobs-production alertmanager-0 -c alertmanager -- \
     wget -qO- http://localhost:9093/api/v2/status | python3 -m json.tool | grep -A 20 cluster
   ```

- **Check the headless service endpoints** — the `alertmanager-cluster` service (port 9094) must have both pod IPs:
   ```bash
   kubectl get service alertmanager-cluster -n rhobs-production -o yaml
   kubectl get endpoints alertmanager-cluster -n rhobs-production
   ```

- **Check NetworkPolicies** — ensure port 9094 is not blocked between pods:
   ```bash
   kubectl get networkpolicy -n rhobs-production
   ```

- **Check for recent pod restarts** — a restart can disrupt gossip cluster formation:
   ```bash
   kubectl describe pods -n rhobs-production -l app.kubernetes.io/name=alertmanager | grep -A 5 "Restart Count"
   ```

- **Verify the StatefulSet cluster peer args** — expected peers should be the two headless DNS entries on port 9094:
   ```bash
   kubectl get statefulset alertmanager -n rhobs-production -o yaml | grep -A 2 cluster.peer
   ```

- **View the Alertmanager UI Status tab** after establishing a sshuttle tunnel — the "Status" page shows discovered cluster peers; compare their IPs against both pod IPs.

**Access Required:**
- Cluster access via sshuttle
- `kubectl exec`, `kubectl logs`, `kubectl get/describe` in `rhobs-production`

---

## AlertmanagerFailedToSendAlerts

**Severity:** `warning` | **For:** 5m | **Component:** Alertmanager

**Summary:**
At least one Alertmanager instance is failing to deliver notifications to an integration. The failure rate exceeds 1% over a 15-minute window.

**Impact:**
Low — the other instance in the cluster may still be delivering successfully. This becomes critical if `AlertmanagerClusterFailedToSendAlerts` also fires. Notification gaps are unlikely but possible for routes hitting the failing instance.

**Alert Expression:**
```promql
(
  rate(alertmanager_notifications_failed_total{job="alertmanager"}[15m])
  /
  ignoring(reason) group_left rate(alertmanager_notifications_total{job="alertmanager"}[15m])
)
> 0.01
```

**Steps:**

- **Identify the failing instance and integration** from alert labels (`instance`, `integration`, `reason`).

- **Check the failure rate broken down by reason** — `reason` indicates `clientError` (4xx), `serverError` (5xx), or `timeout`:
   ```promql
   rate(alertmanager_notifications_failed_total{job="alertmanager"}[15m])
   ```

- **Check the overall notification failure ratio across all integrations:**
   ```promql
   sum by(instance, integration) (rate(alertmanager_notifications_failed_total{job="alertmanager"}[15m]))
   /
   sum by(instance, integration) (rate(alertmanager_notifications_total{job="alertmanager"}[15m]))
   ```

- **Check the notification send rate trend** — a sudden drop to zero differs from a gradual rise in failures:
   ```promql
   sum by(instance, integration) (rate(alertmanager_notifications_total{job="alertmanager"}[15m]))
   ```

- **Check notification latency** — elevated p99 before failures often signals network or timeout issues:
   ```promql
   histogram_quantile(0.99,
     sum by(le, instance, integration) (
       rate(alertmanager_notification_latency_seconds_bucket{job="alertmanager"}[15m])
     )
   )
   ```

- **Examine pod logs for the failing instance:**
   ```bash
   kubectl logs -n rhobs-production alertmanager-0 -c alertmanager | grep -i "notification\|integration\|failed\|error" | tail -50
   ```

- **Verify connectivity to the integration endpoint from the failing pod:**
   ```bash
   kubectl exec -n rhobs-production alertmanager-0 -c alertmanager -- \
     wget --spider --timeout=5 <integration-endpoint-url>
   ```

- **Check integration credentials in the config secret:**
   ```bash
   kubectl get secret alertmanager-config -n rhobs-production -o jsonpath='{.data.alertmanager\.yaml}' | base64 -d | grep -A 5 <integration>
   ```

- **Review the integration's own status page** for outages (PagerDuty: status.pagerduty.com, Slack: status.slack.com).

**Access Required:**
- Cluster access via sshuttle
- `kubectl logs`, `kubectl exec` in `rhobs-production`
- Read access to `alertmanager-config` secret

---

## AlertmanagerClusterFailedToSendAlerts

**Severity:** `warning` | **For:** 5m | **Component:** Alertmanager

**Summary:**
Every Alertmanager instance in the cluster failed to send notifications to a specific integration. No successful delivery is occurring from any replica.

**Impact:**
On-call will not be paged for new critical alerts if PagerDuty is the affected integration. All notifications for the affected integration are silently dropped until resolved.

**Alert Expression:**
```promql
# Critical integrations
min by(job, integration) (
  rate(alertmanager_notifications_failed_total{job="alertmanager", integration=~".*"}[15m])
  /
  ignoring(reason) group_left rate(alertmanager_notifications_total{job="alertmanager", integration=~".*"}[15m])
  > 0
)
> 0.01

# Non-critical integrations (same expression, different integration matcher — severity: warning)
min by(job, integration) (
  rate(alertmanager_notifications_failed_total{job="alertmanager", integration!~".*"}[15m])
  /
  ignoring(reason) group_left rate(alertmanager_notifications_total{job="alertmanager", integration!~".*"}[15m])
  > 0
)
> 0.01
```

**Steps:**

- **Identify the affected integration** from alert labels — determine if it is a critical path (PagerDuty, Slack for on-call) or non-critical.

- **Confirm both instances are failing** — both `alertmanager-0` and `alertmanager-1` should show non-zero failure rates:
   ```promql
   rate(alertmanager_notifications_failed_total{job="alertmanager", integration="<integration>"}[15m])
   ```

- **Compare total vs. failed notification volume:**
   ```promql
   sum by(instance, integration) (rate(alertmanager_notifications_total{job="alertmanager"}[15m]))
   sum by(instance, integration) (rate(alertmanager_notifications_failed_total{job="alertmanager"}[15m]))
   ```

- **Check logs on both pods:**
   ```bash
   kubectl logs -n rhobs-production alertmanager-0 -c alertmanager | grep -i "<integration>\|notification\|failed\|error" | tail -40
   kubectl logs -n rhobs-production alertmanager-1 -c alertmanager | grep -i "<integration>\|notification\|failed\|error" | tail -40
   ```

- **Test connectivity from both pods to the integration endpoint:**
   ```bash
   for pod in alertmanager-0 alertmanager-1; do
     echo "=== $pod ===" && \
     kubectl exec -n rhobs-production $pod -c alertmanager -- \
       wget --spider --timeout=5 <integration-endpoint-url> 2>&1
   done
   ```

- **Inspect credentials and configuration** and cross-reference with app-interface Git for the affected cluster:
   ```bash
   kubectl get secret alertmanager-config -n rhobs-production -o jsonpath='{.data.alertmanager\.yaml}' | base64 -d
   ```

- **Check for a recent config change** that may have rotated or broken credentials:
   ```bash
   kubectl get secret alertmanager-config -n rhobs-production -o yaml | grep -i "creationTimestamp\|resourceVersion"
   ```

- **Check for egress NetworkPolicies** blocking traffic to the integration:
   ```bash
   kubectl get networkpolicy -n rhobs-production -o yaml | grep -A 20 egress
   ```

- **Check notification latency** — `clientError` with elevated latency often indicates rate limiting by the receiver:
   ```promql
   histogram_quantile(0.99,
     sum by(le, integration) (
       rate(alertmanager_notification_latency_seconds_bucket{job="alertmanager", integration="<integration>"}[15m])
     )
   )
   ```

- **If the integration service is healthy and credentials are valid**, review recent config changes in Git and consider rolling back:
   - `https://gitlab.cee.redhat.com/service/app-interface/-/blob/master/resources/rhobs/production/<cluster>/alertmanager-config-rhobs-hcp.secret.yaml`

**Access Required:**
- Cluster access via sshuttle
- `kubectl logs`, `kubectl exec`, `kubectl get secret` in `rhobs-production`
- Integration service admin/status access

---

## AlertmanagerConfigInconsistent

**Severity:** `warning` | **For:** 20m | **Component:** Alertmanager

**Summary:**
The two Alertmanager instances within the cluster are running different configurations. A hash mismatch was detected — instances diverged after a reload that only partially applied across replicas.

**Impact:**
The two replicas may route alerts to different receivers, apply different inhibit rules, or handle silences differently. This leads to unpredictable notification behavior until all instances converge on the same config.

**Alert Expression:**
```promql
count by(job) (
  count_values by(job) ("config_hash", alertmanager_config_hash{job="alertmanager"})
)
!= 1
```

**Steps:**

- **Check current config hash per instance** — the two instances should return identical hash values:
   ```promql
   alertmanager_config_hash{job="alertmanager"}
   ```

- **Check reload success per instance** — an instance returning `0` is still running the old config:
   ```promql
   max_over_time(alertmanager_config_last_reload_successful{job="alertmanager"}[5m])
   ```

- **Check logs on both pods for reload errors:**
   ```bash
   kubectl logs -n rhobs-production alertmanager-0 -c alertmanager | grep -i "reload\|error\|config" | tail -30
   kubectl logs -n rhobs-production alertmanager-1 -c alertmanager | grep -i "reload\|error\|config" | tail -30
   ```

- **Compare the mounted config file hash across pods** — differing md5sums indicate the secret hasn't propagated yet (kubelet sync can take up to ~60s):
   ```bash
   kubectl exec -n rhobs-production alertmanager-0 -c alertmanager -- md5sum /etc/alertmanager/config/alertmanager.yaml
   kubectl exec -n rhobs-production alertmanager-1 -c alertmanager -- md5sum /etc/alertmanager/config/alertmanager.yaml
   ```

- **Verify the secret's current resourceVersion** to confirm which version is expected:
   ```bash
   kubectl get secret alertmanager-config -n rhobs-production -o yaml | grep resourceVersion
   ```

- **Force a reload on the lagging instance** and then re-check the config hash metric:
   ```bash
   kubectl exec -n rhobs-production alertmanager-0 -c alertmanager -- kill -HUP 1
   ```

- **If reload keeps failing, perform a rolling restart of the StatefulSet:**
   ```bash
   kubectl rollout restart statefulset/alertmanager -n rhobs-production
   kubectl rollout status statefulset/alertmanager -n rhobs-production
   ```

- **Confirm both instances converged** — should return `1`:
   ```promql
   count by(job) (count_values by(job) ("config_hash", alertmanager_config_hash{job="alertmanager"}))
   ```

**Access Required:**
- Cluster access via sshuttle
- `kubectl exec`, `kubectl logs`, `kubectl rollout restart` in `rhobs-production`
- Read access to `alertmanager-config` secret

---

## AlertmanagerClusterDown

**Severity:** `warning` | **For:** 5m | **Component:** Alertmanager

**Summary:**
Half or more of the Alertmanager instances within the cluster have been unavailable for more than half of the past 5 minutes. For a 2-replica cluster this fires when at least 1 pod is down.

**Impact:**
Alertmanager's HA redundancy is broken. If the remaining instance also goes down, all alert notifications stop entirely. The surviving instance continues to process alerts but cannot deduplicate across the cluster.

**Alert Expression:**
```promql
(
  count by(job) (avg_over_time(up{job="alertmanager"}[5m]) < 0.5)
  /
  count by(job) (up{job="alertmanager"})
)
>= 0.5
```

**Steps:**

- **Check which instances are down** — instances returning `0` are not reachable by Prometheus:
   ```promql
   up{job="alertmanager"}
   ```

- **Check pod status:**
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=alertmanager,app.kubernetes.io/instance=observatorium -o wide
   ```

- **Describe the failing pod** — look for `OOMKilled`, `Error`, `CrashLoopBackOff`, node pressure, failed scheduling, or PVC mount failures:
   ```bash
   kubectl describe pod -n rhobs-production alertmanager-0
   ```

- **Check recent namespace events:**
   ```bash
   kubectl get events -n rhobs-production --sort-by='.lastTimestamp' | grep -i "alertmanager\|error\|warning" | tail -30
   ```

- **Retrieve logs from the previous (crashed) container:**
   ```bash
   kubectl logs -n rhobs-production alertmanager-0 -c alertmanager --previous
   ```

- **Check PersistentVolumeClaim status** — each pod has a 1Gi `gp2` PVC that must be `Bound`:
   ```bash
   kubectl get pvc -n rhobs-production | grep alertmanager
   kubectl describe pvc alertmanager-data-alertmanager-0 -n rhobs-production
   ```

- **Check node health** where the pod is/was scheduled:
   ```bash
   kubectl get nodes -o wide
   kubectl describe node <node-name> | grep -A 10 "Conditions\|Allocatable"
   ```

- **Check memory usage trend** — memory limit is 5Gi per pod; approaching this limit may explain pod termination:
   ```promql
   container_memory_working_set_bytes{namespace="rhobs-production", pod=~"alertmanager-.*", container="alertmanager"}
   ```

- **Check the current alert volume** — an unusually high number of active alerts can stress the instance:
   ```promql
   sum by(job, instance) (alertmanager_alerts{job="alertmanager"})
   ```

- **If the pod is stuck in Pending**, check for scheduling constraints — the StatefulSet has preferred pod anti-affinity per node:
   ```bash
   kubectl describe pod -n rhobs-production alertmanager-0 | grep -A 20 Events
   ```

- **If the pod needs to be rescheduled**, delete it to trigger StatefulSet reconciliation:
   ```bash
   kubectl delete pod -n rhobs-production alertmanager-0
   ```

**Access Required:**
- Cluster access via sshuttle
- `kubectl get/describe/delete pod`, `kubectl get pvc`, `kubectl get nodes` in `rhobs-production`
- Node-level access if investigating host-side issues

---

## AlertmanagerClusterCrashlooping

**Severity:** `warning` | **For:** 5m | **Component:** Alertmanager

**Summary:**
Half or more of the Alertmanager instances have restarted more than 4 times in the last 10 minutes. For a 2-replica cluster, at least one pod is crashlooping.

**Impact:**
The cluster cannot reliably process or deliver alerts while replicas keep restarting; alerting may be intermittent or unavailable.

**Alert Expression:**
```promql
(
  count by(job) (changes(process_start_time_seconds{job="alertmanager"}[10m]) > 4)
  /
  count by(job) (up{job="alertmanager"})
)
>= 0.5
```

**Steps:**

- **Check the restart rate per instance** — values above 4 identify the crashlooping instance:
   ```promql
   changes(process_start_time_seconds{job="alertmanager"}[10m])
   ```

- **Check pod restart counts and current status:**
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=alertmanager,app.kubernetes.io/instance=observatorium
   ```

- **Retrieve logs from the previous (crashed) container** — common causes: invalid config (`error loading config`, `yaml: unmarshal errors`), OOMKilled (memory limit 5Gi), storage issues (`permission denied` on `/data`), liveness probe failure (HTTP GET `/-/healthy` on port 9093, fails after 8 attempts every 30s):
   ```bash
   kubectl logs -n rhobs-production alertmanager-0 -c alertmanager --previous --tail=100
   ```

- **Check the last termination reason:**
   ```bash
   kubectl describe pod -n rhobs-production alertmanager-0 | grep -A 10 "Last State"
   ```

- **Check memory usage** — if memory was approaching 5Gi at time of crash, OOM is the cause:
   ```promql
   container_memory_working_set_bytes{namespace="rhobs-production", pod=~"alertmanager-.*", container="alertmanager"}
   ```

- **Check the active alert count** — a spike can drive memory usage up:
   ```promql
   sum by(job, instance) (alertmanager_alerts{job="alertmanager"})
   ```

- **Check the alert receive rate** — an ingestion storm can trigger OOM:
   ```promql
   sum by(job, instance) (rate(alertmanager_alerts_received_total{job="alertmanager"}[5m]))
   ```

- **Check for invalid alerts being received:**
   ```promql
   sum by(job, instance) (rate(alertmanager_alerts_invalid_total{job="alertmanager"}[5m]))
   ```

- **If crashing due to invalid configuration**, decode the secret and validate locally with `amtool check-config`, then fix in app-interface Git and re-apply the secret:
   ```bash
   kubectl get secret alertmanager-config -n rhobs-production -o jsonpath='{.data.alertmanager\.yaml}' | base64 -d
   ```

- **If crashing due to a full PVC** — the PVC is 1Gi on `gp2`; consider deleting old silence/nflog data or expanding the PVC:
   ```bash
   kubectl get pvc alertmanager-data-alertmanager-0 -n rhobs-production
   kubectl exec -n rhobs-production alertmanager-0 -c alertmanager -- df -h /data
   ```

- **After fixing the root cause**, monitor the restart rate to confirm stability — should return `0` or small values:
   ```promql
   changes(process_start_time_seconds{job="alertmanager"}[10m])
   ```

**Access Required:**
- Cluster access via sshuttle
- `kubectl logs --previous`, `kubectl describe pod`, `kubectl get pvc`, `kubectl exec` in `rhobs-production`
- Git access to app-interface for config changes
