# Synthetics Blackbox Exporter Alerts Runbook

## Table of Contents

- [SyntheticsBlackboxExporterDown](#syntheticsblackboxexporterdown)
- [SyntheticsBlackboxExporterConfigReloadFailed](#syntheticsblackboxexporterconfigreloadfailed)
- [SyntheticsBlackboxExporterUnknownModule](#syntheticsblackboxexporterunknownmodule)

---

## Environment & Access Information

The blackbox-exporter is **not deployed via a static manifest**. It is created dynamically at runtime by the `synthetics-agent` using the `BlackBoxProberManager`. When the agent starts and receives probe configurations from the synthetics-api, it creates a `Deployment` and `Service` for the exporter in the same namespace.

- **Namespace:** `rhobs-production` (on all RHOBS regional clusters, e.g. `rhobsp01apne1`)
- **Deployment name:** `synthetics-blackbox-prober-default`
- **Service name:** `synthetics-blackbox-prober-default-service` (port `9115`, named `http`)
- **Image:** configured via `blackbox.deployment.image` in the synthetics-agent ConfigMap (default: `quay.io/prometheus/blackbox-exporter:latest`)
- **ServiceMonitor:** `synthetics-bb-exporter` (created by the configuration repo, selects `app.kubernetes.io/name: blackbox-exporter`)

**Access:**
- Identify the cluster from the alert labels (`namespace`, `cluster`, or `instance`)
- Cluster access via sshuttle: `https://visual-app-interface.devshift.net/clusters/<cluster-name>`
- Grafana: `https://grafana.app-sre.devshift.net/?orgId=1` — select `<cluster>-prometheus` datasource
- `kubectl` access: obtain token from the cluster's OpenShift OAuth endpoint

---

## SyntheticsBlackboxExporterDown

**Severity:** `critical` | **For:** 5m

**Summary:**
The synthetics blackbox-exporter is unreachable. Either the pod does not exist (the `up` metric is absent) or the scrape is failing (`up == 0`). Probe results for public endpoints will not be collected.

**Alert Expression:**
```promql
absent(up{job="synthetics-blackbox-prober-default-service"})
  or up{job="synthetics-blackbox-prober-default-service"} == 0
```

**Impact:**
All probe results from this RHOBS cell stop being collected. `probe_success` metrics go stale and SLO burn rate alerts based on probe data may fire or stop firing incorrectly.

**Steps:**

1. **Check if the pod exists:**
   ```bash
   kubectl get pods -n rhobs-production -l prober.synthetics-agent.rhobs=default
   ```
   If no pods are returned, the synthetics-agent has not created the blackbox-exporter deployment yet, or it was deleted.

2. **Check the deployment:**
   ```bash
   kubectl get deployment synthetics-blackbox-prober-default -n rhobs-production
   kubectl describe deployment synthetics-blackbox-prober-default -n rhobs-production
   ```

3. **If the pod exists but is crash-looping, inspect logs:**
   ```bash
   kubectl logs -n rhobs-production -l prober.synthetics-agent.rhobs=default --previous
   kubectl describe pod -n rhobs-production <pod-name>
   ```
   Common causes: bad image, missing config, OOMKilled.

4. **Check the synthetics-agent is healthy** — the blackbox-exporter is created by the agent, so if the agent itself is down, the exporter may not have been created:
   ```bash
   kubectl get pods -n rhobs-production -l app.kubernetes.io/name=synthetics-agent
   kubectl logs -n rhobs-production -l app.kubernetes.io/name=synthetics-agent
   ```

5. **Check the Service and endpoints are healthy:**
   ```bash
   kubectl get svc synthetics-blackbox-prober-default-service -n rhobs-production
   kubectl get endpoints synthetics-blackbox-prober-default-service -n rhobs-production
   ```
   If endpoints are empty, the pod selector may not match or the pod is not ready.

6. **Verify the ServiceMonitor exists and is picking up the service:**
   ```bash
   kubectl get servicemonitor synthetics-bb-exporter -n rhobs-production
   ```

7. **In Grafana**, confirm the scrape target state:
   ```promql
   up{job="synthetics-blackbox-prober-default-service", namespace="rhobs-production"}
   ```

---

## SyntheticsBlackboxExporterConfigReloadFailed

**Severity:** `warning` | **For:** 5m

**Summary:**
The blackbox-exporter failed to reload its configuration. `blackbox_exporter_config_last_reload_successful` is `0`.

**Alert Expression:**
```promql
blackbox_exporter_config_last_reload_successful == 0
```

**Impact:**
The exporter is still running with its previous (last known good) configuration. Any changes to probe modules — e.g. a new module added to the synthetics-agent ConfigMap — will not take effect. Probes that depend on the new module will fail with an unknown module error.

**Steps:**

1. **Check when the last successful reload was:**
   ```promql
   blackbox_exporter_config_last_reload_success_timestamp_seconds{job="synthetics-blackbox-prober-default-service"}
   ```
   Convert the timestamp to a human-readable time to understand how long ago it succeeded.

2. **Check the exporter logs for config parse errors:**
   ```bash
   kubectl logs -n rhobs-production -l prober.synthetics-agent.rhobs=default
   ```
   Look for lines containing `error`, `failed`, or `config`.

3. **Inspect the synthetics-agent ConfigMap** — the blackbox-exporter's config is passed via the agent's ConfigMap and mounted into the exporter:
   ```bash
   kubectl get configmap synthetics-agent-config -n rhobs-production -o yaml
   ```
   Look at the `blackbox` section for malformed YAML or invalid module definitions.

4. **Check if the agent recently updated the ConfigMap** — a bad update could have introduced a syntax error:
   ```bash
   kubectl describe configmap synthetics-agent-config -n rhobs-production
   ```

5. **Trigger a manual config reload** (if the exporter is running):
   ```bash
   kubectl exec -n rhobs-production <pod-name> -- wget -qO- http://localhost:9115/-/reload
   ```
   Check logs again after the reload attempt.

---

## SyntheticsBlackboxExporterUnknownModule

**Severity:** `warning` | **For:** 5m

**Summary:**
The blackbox-exporter is receiving probe requests for a module not defined in its configuration. `blackbox_module_unknown_total` is increasing.

**Alert Expression:**
```promql
rate(blackbox_module_unknown_total{job="synthetics-blackbox-prober-default-service"}[5m]) > 0
```

**Impact:**
Probes using the unknown module name silently fail — `probe_success` will be `0` for all affected targets. This is not a connectivity issue with the probed endpoints; the exporter simply cannot process the request.

**Steps:**

1. **Confirm the rate of unknown module requests:**
   ```promql
   rate(blackbox_module_unknown_total{job="synthetics-blackbox-prober-default-service", namespace="rhobs-production"}[5m])
   ```

2. **Check what module the agent is configured to use** — look at the `BLACKBOX_MODULE` parameter, which defaults to `http_2xx`:
   ```bash
   kubectl get configmap synthetics-agent-config -n rhobs-production -o yaml
   ```
   Look for the `module:` field under the `blackbox.probing` section.

3. **Check what modules are defined in the exporter config:**
   ```bash
   kubectl exec -n rhobs-production <pod-name> -- wget -qO- http://localhost:9115/config
   ```
   Verify the module referenced by the agent (`http_2xx` by default) is present in the output.

4. **Check the Probe CRs** to see what module they are requesting:
   ```bash
   kubectl get probes -n rhobs-production -o yaml | grep module
   ```
   If the module name here doesn't match a defined module in the exporter config, that is the mismatch.

5. **Common cause:** The agent was reconfigured to use a new module name (e.g. via a `BLACKBOX_MODULE` parameter change in the saas file) but the blackbox-exporter config was not updated with that module definition — or the config reload failed (check [SyntheticsBlackboxExporterConfigReloadFailed](#syntheticsblackboxexporterconfigreloadfailed)).
