package lokirules

import (
	"time"

	"github.com/perses/community-mixins/pkg/rules/rule-sdk/alerting"
	"github.com/perses/community-mixins/pkg/rules/rule-sdk/common"
	"github.com/perses/community-mixins/pkg/rules/rule-sdk/promtheusrule"
	"github.com/perses/community-mixins/pkg/rules/rule-sdk/recording"
	"github.com/perses/community-mixins/pkg/rules/rule-sdk/rulegroup"
	promqlbuilder "github.com/perses/promql-builder"
	"github.com/perses/promql-builder/label"
	"github.com/perses/promql-builder/matrix"
	"github.com/perses/promql-builder/vector"
)

func NewLokiRulesBuilder(
	namespace string,
	labels map[string]string,
	annotations map[string]string,
	options ...ConfigOption,
) (promtheusrule.Builder, error) {
	cfg := &RulesConfig{}
	for _, option := range options {
		option(cfg)
	}

	promRule, err := promtheusrule.New(
		"loki-rules",
		namespace,
		promtheusrule.Labels(labels),
		promtheusrule.Annotations(annotations),
		promtheusrule.AddRuleGroup("loki-record", cfg.lokiRecordingRules()...),
		promtheusrule.AddRuleGroup("loki-alerts", cfg.lokiAlerts()...),
	)

	return promRule, err
}

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

func (cfg *RulesConfig) lokiRecordingRules() []rulegroup.Option {
	return []rulegroup.Option{
		rulegroup.AddRule(
			"job_namespace_route_statuscode:loki_request_duration_seconds_count:irate1m",
			recording.Expr(
				promqlbuilder.Sum(
					promqlbuilder.IRate(
						matrix.New(
							vector.New(
								vector.WithMetricName("loki_request_duration_seconds_count"),
							),
							matrix.WithRange(time.Minute),
						),
					),
				).By("job", "namespace", "route", "status_code")),
		),
	}
}

func (cfg *RulesConfig) lokiAlerts() []rulegroup.Option {
	return []rulegroup.Option{
		rulegroup.AddRule(
			"LokiRequestErrors",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.Mul(
						promqlbuilder.Div(
							promqlbuilder.Sum(
								vector.New(
									vector.WithMetricName("job_namespace_route_statuscode:loki_request_duration_seconds_count:irate1m"),
									vector.WithLabelMatchers(
										label.New("status_code").EqualRegexp("5.."),
									),
								),
							).By("job", "namespace", "route"),
							promqlbuilder.Sum(
								vector.New(
									vector.WithMetricName("job_namespace_route_statuscode:loki_request_duration_seconds_count:irate1m"),
								),
							).By("job", "namespace", "route"),
						),
						promqlbuilder.NewNumber(100),
					),
					promqlbuilder.NewNumber(10),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("critical")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-request-errors",
					`{{ $labels.job }} {{ $labels.route }} is experiencing {{ printf "%.2f" $value }}% errors.`,
					"At least 10% of requests are responded by 5xx server errors.",
				),
			),
		),
		rulegroup.AddRule(
			"LokiRequestPanics",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.Sum(
						promqlbuilder.Increase(
							matrix.New(
								vector.New(
									vector.WithMetricName("loki_panic_total"),
								),
								matrix.WithRange(10*time.Minute),
							),
						),
					).By("job", "namespace"),
					promqlbuilder.NewNumber(0),
				),
			),
			alerting.Labels(cfg.alertLabels("critical")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-request-panics",
					`{{ $labels.job }} is experiencing an increase of {{ $value }} panics.`,
					"A panic was triggered.",
				),
			),
		),
		rulegroup.AddRule(
			"LokiRequestLatency",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.HistogramQuantile(
						0.99,
						promqlbuilder.Sum(
							promqlbuilder.IRate(
								matrix.New(
									vector.New(
										vector.WithMetricName("loki_request_duration_seconds_bucket"),
										vector.WithLabelMatchers(
											label.New("route").NotEqualRegexp("(?i).*tail.*"),
										),
									),
									matrix.WithRange(2*time.Minute),
								),
							),
						).By("job", "namespace", "route", "le"),
					),
					promqlbuilder.NewNumber(5),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("critical")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-request-latency",
					`{{ $labels.job }} {{ $labels.route }} is experiencing {{ printf "%.2f" $value }}s 99th percentile latency.`,
					"The 99th percentile is experiencing high latency (higher than 5 seconds).",
				),
			),
		),
		rulegroup.AddRule(
			"LokiTenantRateLimit",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.Mul(
						promqlbuilder.Div(
							promqlbuilder.Sum(
								vector.New(
									vector.WithMetricName("job_namespace_route_statuscode:loki_request_duration_seconds_count:irate1m"),
									vector.WithLabelMatchers(
										label.New("status_code").Equal("429"),
									),
								),
							).By("job", "namespace", "route"),
							promqlbuilder.Sum(
								vector.New(
									vector.WithMetricName("job_namespace_route_statuscode:loki_request_duration_seconds_count:irate1m"),
								),
							).By("job", "namespace", "route"),
						),
						promqlbuilder.NewNumber(100),
					),
					promqlbuilder.NewNumber(10),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-tenant-rate-limit",
					`{{ $labels.job }} {{ $labels.route }} is experiencing 429 errors.`,
					"At least 10% of requests are responded with the rate limit error code.",
				),
			),
		),
		rulegroup.AddRule(
			"LokiWritePathHighLoad",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.Sum(
						vector.New(
							vector.WithMetricName("loki_ingester_wal_replay_flushing"),
						),
					).By("job", "namespace"),
					promqlbuilder.NewNumber(0),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-write-path-high-load",
					`The write path is experiencing high load.`,
					"The write path is experiencing high load, causing backpressure storage flushing.",
				),
			),
		),
		rulegroup.AddRule(
			"LokiReadPathHighLoad",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.HistogramQuantile(
						0.99,
						promqlbuilder.Sum(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("loki_logql_querystats_latency_seconds_bucket"),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						).By("job", "namespace", "le"),
					),
					promqlbuilder.NewNumber(30),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-read-path-high-load",
					`The read path is experiencing high load.`,
					"The read path has high volume of queries, causing longer response times.",
				),
			),
		),
		rulegroup.AddRule(
			"LokiDiscardedSamplesWarning",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.Sum(
						promqlbuilder.IRate(
							matrix.New(
								vector.New(
									vector.WithMetricName("loki_discarded_samples_total"),
									vector.WithLabelMatchers(
										label.New("reason").NotEqual("rate_limited"),
										label.New("reason").NotEqual("per_stream_rate_limit"),
										label.New("reason").NotEqual("stream_limit"),
									),
								),
								matrix.WithRange(2*time.Minute),
							),
						),
					).By("namespace", "tenant", "reason"),
					promqlbuilder.NewNumber(0),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-discarded-samples-warning",
					`Loki in namespace {{ $labels.namespace }} is discarding samples in the "{{ $labels.tenant }}" tenant during ingestion. Samples are discarded because of "{{ $labels.reason }}" at a rate of {{ .Value | humanize }} samples per second.`,
					"Loki is discarding samples during ingestion because they fail validation.",
				),
			),
		),
		rulegroup.AddRule(
			"LokiIngesterFlushFailureRateCritical",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.Sum(
						promqlbuilder.Div(
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("loki_ingester_chunks_flush_failures_total"),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
							promqlbuilder.Rate(
								matrix.New(
									vector.New(
										vector.WithMetricName("loki_ingester_chunks_flush_requests_total"),
									),
									matrix.WithRange(5*time.Minute),
								),
							),
						),
					).By("namespace", "pod"),
					promqlbuilder.NewNumber(0.2),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("critical")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-ingester-flush-failure-rate-critical",
					`Loki ingester {{ $labels.pod }} in the namespace {{ $labels.namespace }} has a critical flush failure rate of {{ $value | humanizePercentage }} over the last 5 minutes. This requires immediate attention as data is not being flushed to the storage. Validate if the storage configuration is still valid and if the storage is still reachable. Current failure rate: {{ $value | humanizePercentage }} Threshold: 20%`,
					"Loki ingester has critical flush failure rate.",
				),
			),
		),
		rulegroup.AddRule(
			"LokistackComponentsNotReadyWarning",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.Sum(
						promqlbuilder.LabelReplace(
							vector.New(
								vector.WithMetricName("lokistack_status_condition"),
								vector.WithLabelMatchers(
									label.New("reason").Equal("ReadyComponents"),
									label.New("status").Equal("false"),
								),
							),
							"namespace", "$1", "stack_namespace", "(.+)",
						),
					).By("stack_name", "namespace"),
					promqlbuilder.NewNumber(0),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#lokistack-components-not-ready-warning",
					`The LokiStack "{{ $labels.stack_name }}" in namespace "{{ $labels.namespace }}" has components that are not ready.`,
					"One or more LokiStack components are not ready.",
				),
			),
		),
		rulegroup.AddRule(
			"LokiRulerBadConfiguration",
			alerting.Expr(
				promqlbuilder.Eql(
					promqlbuilder.MaxOverTime(
						matrix.New(
							vector.New(
								vector.WithMetricName("loki_ruler_config_last_reload_successful"),
							),
							matrix.WithRange(5*time.Minute),
						),
					),
					promqlbuilder.NewNumber(0),
				),
			),
			alerting.For("10m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-ruler-bad-configuratoin",
					`LokiRuler {{ $labels.pod }} in namespace {{ $labels.namespace }} has failed to reload its configuration.`,
					"Failed LokiRuler configuration reload",
				),
			),
		),
		rulegroup.AddRule(
			"LokiRulerRuleFailures",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.Increase(
						matrix.New(
							vector.New(
								vector.WithMetricName("loki_prometheus_rule_evaluation_failures_total"),
							),
							matrix.WithRange(5*time.Minute),
						),
					),
					promqlbuilder.NewNumber(0),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-ruler-rule-failures",
					`LokiRuler {{ $labels.pod }} in namespace {{ $labels.namespace }} has failed to evaluate {{ printf "%.0f" $value }} rules in the last 5m.`,
					"LokiRuler is failing rule evaluations",
				),
			),
		),
		rulegroup.AddRule(
			"LokiRulerNotConnectedToAlertmanagers",
			alerting.Expr(
				promqlbuilder.Eql(
					promqlbuilder.MaxOverTime(
						matrix.New(
							vector.New(
								vector.WithMetricName("loki_prometheus_notifications_alertmanagers_discovered"),
							),
							matrix.WithRange(5*time.Minute),
						),
					),
					promqlbuilder.NewNumber(0),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-ruler-not-connected-to-alertmanager",
					`LokiRuler {{ $labels.pod }} in namespace {{ $labels.namespace }} is not connected to any Alertmanagers.`,
					"LokiRuler is is not connected to any Alertmanagers.",
				),
			),
		),
		rulegroup.AddRule(
			"LokiRulerNotificationQueueRunningFull",
			alerting.Expr(
				promqlbuilder.Gtr(
					promqlbuilder.PredictLinear(
						matrix.New(
							vector.New(
								vector.WithMetricName("loki_prometheus_notifications_queue_length"),
							),
							matrix.WithRange(5*time.Minute),
						),
						30*60,
					),
					promqlbuilder.MinOverTime(
						matrix.New(
							vector.New(
								vector.WithMetricName("loki_prometheus_notifications_queue_capacity"),
							),
							matrix.WithRange(5*time.Minute),
						),
					),
				),
			),
			alerting.For("15m"),
			alerting.Labels(cfg.alertLabels("warning")),
			alerting.Annotations(
				common.BuildAnnotations(
					"",
					cfg.runbookURL,
					"#loki-ruler-notification-queue-running-full",
					`Alert notification queue for LokiRuler {{ $labels.pod }} in namespace {{ $labels.namespace }} is predicted to run full in less than 30m.`,
					"LokiRuler alert notification queue getting full",
				),
			),
		),
		// NotificationQueueRunningFull
		//
		// TODO(simonpasquier): add alerting rule to detect the absence of
		// LokiRuler once it is deployed to all environments.
	}
}

func (cfg *RulesConfig) alertLabels(severity string) map[string]string {
	return map[string]string{
		"service":  cfg.serviceLabelValue,
		"severity": severity,
	}
}
