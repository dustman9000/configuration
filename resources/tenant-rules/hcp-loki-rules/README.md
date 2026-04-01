# HCP Loki Tenant Rules

This directory contains Loki rules templates for ROSA HCP (Hosted Control Plane) monitoring, organized by functional domain for improved maintainability.

## Layout

```
hcp-loki-rules/
├── README.md           # This file
└── alerts/             # Templates for Loki AlertingRules
    └── observability.yaml
```

**RecordingRule:** not generated currently. When needed, add a `rules/` tree and extend `scripts/generate-hcp-loki-rules.sh` (or a sibling script) to emit a separate `hcp-loki-recording-rules.yaml`.

## Files by Functional Domain

### observability.yaml

Monitoring infrastructure health:

- `watchdog` - HCPWatchdogDown heartbeat

## Usage

Each file is a standalone OpenShift Template that can be processed with `oc process`:

```bash
# Process a single domain
oc process -f api-server.yaml \
  -p NAMESPACE=rhobs-hcp \
  -p TENANT=hcp | oc apply -f -

# Process all domains
for f in *.yaml; do
  oc process -f "$f" \
    -p NAMESPACE=rhobs-hcp \
    -p TENANT=hcp | oc apply -f -
done
```

## Parameters

All templates accept the same parameters:

| Parameter | Required | Description |
|-----------|----------|-------------|
| NAMESPACE | Yes | Namespace to deploy the rules to |
| TENANT | Yes | Tenant identifier for Thanos operator |

## Adding New Rules

1. Identify the appropriate functional domain (or create a new file)
2. Add the PrometheusRule to the corresponding file
3. If creating a new file, add it to the `FILES` array in `scripts/generate-hcp-loki-rules.sh`
4. Run `make hcp-rules` to regenerate `../hcp.yaml`
5. Update this README with the new alert/recording rule
6. Commit both the source file and the generated `hcp.yaml`
7. Start new alerts with `severity: soaking` -- see the [alert graduation process](https://gitlab.cee.redhat.com/service/hypershift-pagerduty-config/-/blob/main/docs/ALERT-GRADUATION.md)
