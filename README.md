# RHOBS Configuration

This project holds the configuration files for our internal Red Hat Observability Service based on [Observatorium](https://github.com/observatorium/observatorium).

## Requirements

* Go
* [Mage](https://magefile.org/) (`go install github.com/magefile/mage@latest`)

## RHOS cells

A RHOBS cell (or RHOBS cluster) exposes APIs to ingest, alert on and query metrics and logs. Deployment and configuration of a cell is managed by app-interface which applies the YAML manifests (bundle) from the `resources/clusters/<env>/<cluster>` directories.

This repository leans heavily on [Mage](https://magefile.org/) to generate the RHOBS manifests. You can find the available Mage targets by running:

```bash
mage -l
```

### Manifests generation

The configuration for each RHOBS cell is defined in the `clusters/` directory.

To regenerate/update all YAML manifests, run

```bash
mage build:clusters
```

To regenerate manifests for all clusters in a given environment, run

```bash
mage build:environmnet <environment name>
```

To regenerate manifests for a single cluster, run

```bash
mage build:cluster <cluster name>
```

### Components update

Use the `mage sync:konflux` command to update the component images (and Custom Resource Definitions for Thanos and Loki).

```bash
mage sync:konflux loki-operator latest
mage sync:konflux thanos-operator latest
mage sync:konflux observatorium-api latest
```

The commit SHAs for container images are based on the Konflux downstream repositories:
* [Observatorium API](https://github.com/rhobs/rhobs-konflux-obs-api)
* [Thanos operator](https://github.com/rhobs/rhobs-konflux-thanos-operator)
* [Loki operator](https://github.com/rhobs/rhobs-konflux-loki-operator)

More details in https://docs.google.com/document/d/1wSS3H_6irgRHCoglaIVEZyWt-s8vPAthjcEwN6cfY7s/edit?usp=sharing

## Tenant rules

Tenant rules can be based on metrics or logs. They are defined under
* `resources/tenant-rules/hcp` for rules based on HCP metrics.
* `resources/tenant-rules/sc` for rules based on Service Cluster (SC) metrics.

After update, they should be regenerated:

```bash
make all
```

## Collection stacks

The repository also contains manifests under `resources/collection` for the components collecting and sending metrics and logs to the RHOBS cells. These manifests can be edited manually.

## See also

[hcp_configuration_promotion.md](./docs/sop/hcp_configuration_promotion.md) for the details about the app-interface's integration.
