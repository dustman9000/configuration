package syntheticsrules

import (
	"time"

	"github.com/perses/community-mixins/pkg/rules/rule-sdk/alerting"
	"github.com/perses/community-mixins/pkg/rules/rule-sdk/promtheusrule"
	"github.com/perses/community-mixins/pkg/rules/rule-sdk/rulegroup"
	promqlbuilder "github.com/perses/promql-builder"
	"github.com/perses/promql-builder/label"
	"github.com/perses/promql-builder/matrix"
	"github.com/perses/promql-builder/vector"
)

const runbookBase = "https://github.com/openshift/ops-sop/blob/master/hypershift/alerts/rhobs-synthetics"

type RulesConfig struct {
	runbookURL        string
	serviceLabelValue string
}

type ConfigOption func(*RulesConfig)

func WithRunbookURL(url string) ConfigOption {
	return func(config *RulesConfig) {
		config.runbookURL = url
	}
}

func WithServiceLabelValue(labelValue string) ConfigOption {
	return func(config *RulesConfig) {
		config.serviceLabelValue = labelValue
	}
}

func (cfg *RulesConfig) alertLabels(severity string) map[string]string {
	return map[string]string{
		"severity": severity,
		"service":  cfg.serviceLabelValue,
	}
}

func (cfg *RulesConfig) runbook(alert string) map[string]string {
	url := cfg.runbookURL
	if url == "" {
		url = runbookBase + "/" + alert + ".md"
	}
	return map[string]string{
		"runbook": url,
	}
}

func annotations(summary, description string, extra map[string]string) map[string]string {
	result := map[string]string{
		"summary":     summary,
		"description": description,
	}
	for k, v := range extra {
		result[k] = v
	}
	return result
}

// NewSyntheticsAPIRulesBuilder returns a PrometheusRule builder for synthetics-api health alerts.
// synthetics-api runs on RHOBS cells; metrics are scraped locally.
func NewSyntheticsAPIRulesBuilder(
	namespace string,
	labels map[string]string,
	metaAnnotations map[string]string,
	options ...ConfigOption,
) (promtheusrule.Builder, error) {
	cfg := &RulesConfig{
		serviceLabelValue: "rhobs-synthetics",
	}
	for _, option := range options {
		option(cfg)
	}

	return promtheusrule.New(
		"rhobs-synthetics-api",
		namespace,
		promtheusrule.Labels(labels),
		promtheusrule.Annotations(metaAnnotations),
		promtheusrule.AddRuleGroup("rhobs-synthetics-api", cfg.syntheticsAPIAlerts()...),
	)
}

// NewSyntheticsAgentRulesBuilder returns a PrometheusRule builder for synthetics-agent health alerts.
// synthetics-agent runs on RHOBS cells; metrics are scraped locally.
func NewSyntheticsAgentRulesBuilder(
	namespace string,
	labels map[string]string,
	metaAnnotations map[string]string,
	options ...ConfigOption,
) (promtheusrule.Builder, error) {
	cfg := &RulesConfig{
		serviceLabelValue: "rhobs-synthetics",
	}
	for _, option := range options {
		option(cfg)
	}

	return promtheusrule.New(
		"rhobs-synthetics-agent",
		namespace,
		promtheusrule.Labels(labels),
		promtheusrule.Annotations(metaAnnotations),
		promtheusrule.AddRuleGroup("rhobs-synthetics-agent", cfg.syntheticsAgentAlerts()...),
	)
}

func (cfg *RulesConfig) syntheticsAPIAlerts() []rulegroup.Option {
	return []rulegroup.Option{
		rulegroup.Interval("1m"),
		rulegroup.AddRule(
			"SyntheticsAPIDown",
			alerting.Expr(
				promqlbuilder.Eqlc(
					vector.New(
						vector.WithMetricName("up"),
						vector.WithLabelMatchers(label.New("job").Equal("synthetics-api")),
					),
					promqlbuilder.NewNumber(0),
				),
			),
			alerting.For("5m"),
			alerting.Labels(cfg.alertLabels("critical")),
			alerting.Annotations(annotations(
				"Synthetics API is down",
				"The synthetics-api service has been unreachable for 5 minutes. Probe lifecycle management is unavailable.",
				cfg.runbook("SyntheticsAPIDown"),
			)),
		),
		rulegroup.AddRule(
			"SyntheticsAPIHighErrorRate",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.Parenthesis(promqlbuilder.Div(
						promqlbuilder.Rate(
							matrix.New(
								vector.New(vector.WithMetricName("rhobs_synthetics_api_probestore_errors_total")),
								matrix.WithRange(5*time.Minute),
							),
						),
						promqlbuilder.Rate(
							matrix.New(
								vector.New(vector.WithMetricName("rhobs_synthetics_api_http_requests_total")),
								matrix.WithRange(5*time.Minute),
							),
						),
					)),
					promqlbuilder.NewNumber(0.1),
				),
			),
			alerting.For("10m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(annotations(
				"Synthetics API has high probestore error rate",
				"The synthetics-api has >10% probestore error rate for 10 minutes.",
				cfg.runbook("SyntheticsAPIHighErrorRate"),
			)),
		),
		rulegroup.AddRule(
			"SyntheticsAPIHighLatency",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.HistogramQuantile(
						0.99,
						promqlbuilder.Rate(
							matrix.New(
								vector.New(vector.WithMetricName("rhobs_synthetics_api_http_request_duration_seconds_bucket")),
								matrix.WithRange(5*time.Minute),
							),
						),
					),
					promqlbuilder.NewNumber(5),
				),
			),
			alerting.For("10m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(annotations(
				"Synthetics API has high request latency",
				"The synthetics-api p99 request latency has been above 5 seconds for 10 minutes.",
				cfg.runbook("SyntheticsAPIHighLatency"),
			)),
		),
		rulegroup.AddRule(
			"SyntheticsAPIMetricsAbsent",
			alerting.Expr(
				promqlbuilder.Absent(
					vector.New(
						vector.WithMetricName("up"),
						vector.WithLabelMatchers(label.New("job").Equal("synthetics-api")),
					),
				),
			),
			alerting.For("10m"),
			alerting.Labels(cfg.alertLabels("critical")),
			alerting.Annotations(annotations(
				"Synthetics API metrics are absent",
				"No metrics are being received from the synthetics-api. The service may be completely down or its metrics endpoint may be unreachable.",
				cfg.runbook("SyntheticsAPIMetricsAbsent"),
			)),
		),
	}
}

// NewSyntheticsBlackboxExporterRulesBuilder returns a PrometheusRule builder for synthetics blackbox-exporter health alerts.
// The blackbox-exporter runs on RHOBS cells; metrics are scraped via the synthetics-bb-exporter ServiceMonitor.
func NewSyntheticsBlackboxExporterRulesBuilder(
	namespace string,
	labels map[string]string,
	metaAnnotations map[string]string,
	options ...ConfigOption,
) (promtheusrule.Builder, error) {
	cfg := &RulesConfig{
		serviceLabelValue: "rhobs-synthetics",
	}
	for _, option := range options {
		option(cfg)
	}

	return promtheusrule.New(
		"rhobs-synthetics-blackbox-exporter",
		namespace,
		promtheusrule.Labels(labels),
		promtheusrule.Annotations(metaAnnotations),
		promtheusrule.AddRuleGroup("rhobs-synthetics-blackbox-exporter", cfg.syntheticsBlackboxExporterAlerts()...),
	)
}

func (cfg *RulesConfig) syntheticsBlackboxExporterAlerts() []rulegroup.Option {
	return []rulegroup.Option{
		rulegroup.Interval("1m"),
		rulegroup.AddRule(
			"SyntheticsBlackboxExporterConfigReloadFailed",
			alerting.Expr(
				promqlbuilder.Eqlc(
					vector.New(vector.WithMetricName("blackbox_exporter_config_last_reload_successful")),
					promqlbuilder.NewNumber(0),
				),
			),
			alerting.For("5m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(annotations(
				"Synthetics blackbox-exporter config reload failed",
				"The synthetics blackbox-exporter failed to reload its configuration. Probe module changes will not take effect until this is resolved.",
				map[string]string{"runbook": "https://gitlab.cee.redhat.com/rhobs/configuration/-/tree/main/runbooks/synthetics-blackbox-exporter.md#syntheticsblackboxexporterconfigreloadfailed"},
			)),
		),
		rulegroup.AddRule(
			"SyntheticsBlackboxExporterUnknownModule",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.Rate(
						matrix.New(
							vector.New(vector.WithMetricName("blackbox_module_unknown_total")),
							matrix.WithRange(5*time.Minute),
						),
					),
					promqlbuilder.NewNumber(0),
				),
			),
			alerting.For("5m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(annotations(
				"Synthetics blackbox-exporter receiving requests for unknown module",
				"The synthetics blackbox-exporter is receiving probe requests for a module not defined in its configuration. Affected probes will silently fail.",
				map[string]string{"runbook": "https://gitlab.cee.redhat.com/rhobs/configuration/-/tree/main/runbooks/synthetics-blackbox-exporter.md#syntheticsblackboxexporterunknownmodule"},
			)),
		),
		rulegroup.AddRule(
			"SyntheticsBlackboxExporterDown",
			alerting.Expr(
				promqlbuilder.Or(
					promqlbuilder.Absent(
						vector.New(
							vector.WithMetricName("up"),
							vector.WithLabelMatchers(label.New("job").Equal("synthetics-blackbox-prober-default-service")),
						),
					),
					promqlbuilder.Eqlc(
						vector.New(
							vector.WithMetricName("up"),
							vector.WithLabelMatchers(label.New("job").Equal("synthetics-blackbox-prober-default-service")),
						),
						promqlbuilder.NewNumber(0),
					),
				),
			),
			alerting.For("5m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(annotations(
				"Synthetics blackbox-exporter is down",
				"The synthetics blackbox-exporter has been unreachable for 5 minutes. The pod may not exist or the scrape is failing. Probe results will not be collected.",
				map[string]string{"runbook": "https://gitlab.cee.redhat.com/rhobs/configuration/-/tree/main/runbooks/synthetics-blackbox-exporter.md#syntheticsblackboxexporterdown"},
			)),
		),
	}
}

func (cfg *RulesConfig) syntheticsAgentAlerts() []rulegroup.Option {
	return []rulegroup.Option{
		rulegroup.Interval("1m"),
		rulegroup.AddRule(
			"SyntheticsAgentDown",
			alerting.Expr(
				promqlbuilder.Eqlc(
					vector.New(
						vector.WithMetricName("up"),
						vector.WithLabelMatchers(label.New("job").Equal("synthetics-agent")),
					),
					promqlbuilder.NewNumber(0),
				),
			),
			alerting.For("5m"),
			alerting.Labels(cfg.alertLabels("critical")),
			alerting.Annotations(annotations(
				"Synthetics Agent is down",
				"The synthetics-agent has been unreachable for 5 minutes. Probe reconciliation is not running.",
				cfg.runbook("SyntheticsAgentDown"),
			)),
		),
		rulegroup.AddRule(
			"SyntheticsAgentReconcileFailures",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.Parenthesis(promqlbuilder.Div(
						promqlbuilder.Rate(
							matrix.New(
								vector.New(
									vector.WithMetricName("rhobs_synthetics_agent_reconciliation_total"),
									vector.WithLabelMatchers(label.New("status").Equal("error")),
								),
								matrix.WithRange(10*time.Minute),
							),
						),
						promqlbuilder.Rate(
							matrix.New(
								vector.New(vector.WithMetricName("rhobs_synthetics_agent_reconciliation_total")),
								matrix.WithRange(10*time.Minute),
							),
						),
					)),
					promqlbuilder.NewNumber(0.5),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(annotations(
				"Synthetics Agent has high reconciliation failure rate",
				"The synthetics-agent has >50% reconciliation failure rate for 15 minutes. Probe CRs may not be getting created or updated.",
				cfg.runbook("SyntheticsAgentReconcileFailures"),
			)),
		),
		rulegroup.AddRule(
			"SyntheticsAgentFetchFailures",
			alerting.Expr(
				promqlbuilder.Unless(
					promqlbuilder.Gtr(
						promqlbuilder.Rate(
							matrix.New(
								vector.New(
									vector.WithMetricName("rhobs_synthetics_agent_probe_list_fetch_total"),
									vector.WithLabelMatchers(label.New("status").Equal("error")),
								),
								matrix.WithRange(10*time.Minute),
							),
						),
						promqlbuilder.NewNumber(0),
					),
					promqlbuilder.Gtr(
						promqlbuilder.Rate(
							matrix.New(
								vector.New(
									vector.WithMetricName("rhobs_synthetics_agent_probe_list_fetch_total"),
									vector.WithLabelMatchers(label.New("status").Equal("success")),
								),
								matrix.WithRange(10*time.Minute),
							),
						),
						promqlbuilder.NewNumber(0),
					),
				),
			),
			alerting.For("10m"),
			alerting.Labels(cfg.alertLabels("critical")),
			alerting.Annotations(annotations(
				"Synthetics Agent cannot fetch probe list from API",
				"The synthetics-agent has had no successful API fetches for 10 minutes. New probes will not be created and existing probes will not be reconciled.",
				cfg.runbook("SyntheticsAgentFetchFailures"),
			)),
		),
		rulegroup.AddRule(
			"SyntheticsAgentNoProbesManaged",
			alerting.Expr(
				promqlbuilder.Eqlc(
					vector.New(vector.WithMetricName("rhobs_synthetics_agent_probe_resources_managed")),
					promqlbuilder.NewNumber(0),
				),
			),
			alerting.For("30m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(annotations(
				"Synthetics Agent is managing zero probes",
				"The synthetics-agent has been managing zero probe CRs for 30 minutes. This may indicate a configuration issue or API connectivity problem.",
				cfg.runbook("SyntheticsAgentNoProbesManaged"),
			)),
		),
		rulegroup.AddRule(
			"SyntheticsAgentReconcileStale",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.Sub(
						promqlbuilder.Time(),
						vector.New(vector.WithMetricName("rhobs_synthetics_agent_last_reconciliation_timestamp_seconds")),
					),
					promqlbuilder.NewNumber(600),
				),
			),
			alerting.For("10m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(annotations(
				"Synthetics Agent reconciliation is stale",
				"The synthetics-agent has not completed a reconciliation cycle in over 5 minutes (it normally runs every 30 seconds). The agent may be hung or crashed without the process exiting.",
				cfg.runbook("SyntheticsAgentReconcileStale"),
			)),
		),
		rulegroup.AddRule(
			"SyntheticsAgentMetricsAbsent",
			alerting.Expr(
				promqlbuilder.Absent(
					vector.New(
						vector.WithMetricName("up"),
						vector.WithLabelMatchers(label.New("job").Equal("synthetics-agent")),
					),
				),
			),
			alerting.For("10m"),
			alerting.Labels(cfg.alertLabels("critical")),
			alerting.Annotations(annotations(
				"Synthetics Agent metrics are absent",
				"No metrics are being received from the synthetics-agent. The service may be completely down or its metrics endpoint may be unreachable.",
				cfg.runbook("SyntheticsAgentMetricsAbsent"),
			)),
		),
	}
}
