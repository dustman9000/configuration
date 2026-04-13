package main

import (
	"strings"

	"github.com/bwplotka/mimic"
	"github.com/bwplotka/mimic/encoding"
	alertmanagerrules "github.com/perses/community-mixins/pkg/rules/alertmanager"
	thanosrules "github.com/perses/community-mixins/pkg/rules/thanos"
	thanosoperatorrules "github.com/perses/community-mixins/pkg/rules/thanos-operator"
	v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"gitlab.cee.redhat.com/rhobs/configuration/internal/lokirules"
	"gitlab.cee.redhat.com/rhobs/configuration/internal/syntheticsrules"
	customthanosrules "gitlab.cee.redhat.com/rhobs/configuration/internal/thanosrules"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Dashboard URLs
const (
	dashboardThanosReceive  = "https://grafana.app-sre.devshift.net/d/thanos-receive-overview/thanos-receive-overview?orgId=1&from=now-1h&to=now&timezone=UTC&var-datasource={{$externalLabels.cluster}}-prometheus&var-router_job=thanos-receive-router-rhobs&var-ingester_job=thanos-receive-ingester-rhobs-default&var-namespace={{$labels.namespace}}&var-tenant=EFD08939-FE1D-41A1-A28A-BE9A9BC68003"
	dashboardThanosCompact  = "https://grafana.app-sre.devshift.net/d/thanos-compact-overview/thanos-compact-overview?orgId=1&from=now-1h&to=now&timezone=UTC&var-datasource={{$externalLabels.cluster}}-prometheus&var-namespace={{$labels.namespace}}&var-job=thanos-compact-rhobs"
	dashboardThanosQuery    = "https://grafana.app-sre.devshift.net/d/thanos-query-overview/thanos-query-overview?orgId=1&from=now-1h&to=now&timezone=UTC&var-datasource={{$externalLabels.cluster}}-prometheus&var-namespace={{$labels.namespace}}&var-job=thanos-query-rhobs"
	dashboardThanosStore    = "https://grafana.app-sre.devshift.net/d/thanos-store-overview/thanos-store-gateway-overview?orgId=1&from=now-1h&to=now&timezone=UTC&var-datasource={{$externalLabels.cluster}}-prometheus&var-job=thanos-store-default&var-namespace={{$labels.namespace}}"
	dashboardThanosRule     = "https://grafana.app-sre.devshift.net/d/thanos-ruler-overview/thanos-ruler-overview?orgId=1&from=now-1h&to=now&timezone=UTC&var-datasource={{$externalLabels.cluster}}-prometheus&var-job=thanos-ruler-rhobs&var-namespace={{$labels.namespace}}"
	dashboardThanosOperator = "https://grafana.app-sre.devshift.net/d/72e0e05bef5099e5f049b05fdc429ed4/thanos-operator-controller-manager?orgId=1&from=now-1h&to=now&timezone=UTC&var-datasource={{$externalLabels.cluster}}-prometheus&refresh=30s"

	dashboardAlertmanager = "https://grafana.app-sre.devshift.net/d/50b36e28785705570854022296f14821/alertmanager?orgId=1&refresh=10s&var-datasource={{$externalLabels.cluster}}-prometheus&var-namespace={{$labels.namespace}}&var-job=All&var-pod=All&var-interval=5m"

	dashboardLokiWrites = "https://grafana.app-sre.devshift.net/d/rhobs-lokistack-writes/lokistack-writes?orgId=1&var-datasource={{$externalLabels.cluster}}-prometheus&var-namespace={{$labels.namespace}}"
)

func (b Build) Rules() error {
	b.ThanosRules()
	b.ThanosOperatorRules()
	b.AlertmanagerRules()
	b.LokiRules()
	b.SLORules()
	b.SyntheticsRules()
	return nil
}

func (b Build) ThanosRules() {
	gen := b.o11yGenerator("thanos-rules")
	thanosRules(gen)
}

func thanosRules(gen *mimic.Generator) {
	gen.Add("thanos-rules.yaml", encoding.GhodssYAML("", ThanosPrometheusRule(false)))
	gen.Add("thanos-rules-non-critical.yaml", encoding.GhodssYAML("", ThanosPrometheusRule(true)))
	gen.Generate()
}

func (b Build) ThanosOperatorRules() {
	gen := b.o11yGenerator("thanos-operator-rules")
	thanosOperatorRules(gen)
}

func thanosOperatorRules(gen *mimic.Generator) {
	gen.Add("thanos-operator-rules.yaml", encoding.GhodssYAML("", ThanosOperatorPrometheusRule(false)))
	gen.Add("thanos-operator-rules-non-critical.yaml", encoding.GhodssYAML("", ThanosOperatorPrometheusRule(true)))
	gen.Generate()
}

func (b Build) AlertmanagerRules() {
	gen := b.o11yGenerator("alertmanager-rules")
	alertmanagerRules(gen)
}

func alertmanagerRules(gen *mimic.Generator) {
	gen.Add("alertmanager-rules.yaml", encoding.GhodssYAML("", AlertmanagerPrometheusRule(false)))
	gen.Add("alertmanager-rules-non-critical.yaml", encoding.GhodssYAML("", AlertmanagerPrometheusRule(true)))
	gen.Generate()
}

// LokiRules generates recording rules and alerting rules for Loki
func (b Build) LokiRules() {
	gen := b.o11yGenerator("loki-rules")
	lokiRules(gen)
}

func lokiRules(gen *mimic.Generator) {
	gen.Add("loki-rules.yaml", encoding.GhodssYAML("", LokiPrometheusRule(false)))
	gen.Add("loki-rules-non-critical.yaml", encoding.GhodssYAML("", LokiPrometheusRule(true)))
	gen.Generate()
}

func ThanosPrometheusRule(nonCriticalPostProcessing bool) *appInterfacePrometheusRule {
	builder, err := thanosrules.NewThanosRulesBuilder(
		"",
		map[string]string{
			"app.kubernetes.io/component": "thanos",
			"app.kubernetes.io/name":      "thanos-rules",
			"app.kubernetes.io/part-of":   rhobsNextServiceLabel,
			"app.kubernetes.io/version":   "main",
			"prometheus":                  "app-sre",
			"role":                        "alert-rules",
		},
		map[string]string{},
		thanosrules.WithRunbookURL("https://gitlab.cee.redhat.com/rhobs/configuration/-/tree/main/runbooks/thanos.md"),
		thanosrules.WithServiceLabelValue("thanos"),
		thanosrules.WithCompactDashboardURL(dashboardThanosCompact),
		thanosrules.WithQueryDashboardURL(dashboardThanosQuery),
		thanosrules.WithReceiveDashboardURL(dashboardThanosReceive),
		thanosrules.WithStoreDashboardURL(dashboardThanosStore),
		thanosrules.WithRuleDashboardURL(dashboardThanosRule),
		thanosrules.WithServiceLabelValue(rhobsNextServiceLabel),
		thanosrules.WithServiceSelectorSuffix("-rhobs"),
	)

	if err != nil {
		return nil
	}

	if nonCriticalPostProcessing {
		builder.PrometheusRule = RuleNonCriticalPostProcessing(builder.PrometheusRule)
	} else {
		builder.PrometheusRule = RuleCriticalPostProcessing(builder.PrometheusRule)
	}
	builder.Spec.Groups = ReplaceSummaryWithMessage(builder.Spec.Groups)
	builder.Spec.Groups = ReplaceStoreRhobsWithDefault(builder.Spec.Groups)

	// Append custom per-rule-group evaluation failure alert.
	// The mixin's ThanosRuleHighRuleEvaluationFailures uses a 5% ratio threshold
	// which is too high to catch a single failing rule out of 64. This alert fires
	// per rule_group when sustained failures are detected.
	for i, g := range builder.Spec.Groups {
		if g.Name == "thanos-rule" {
			builder.Spec.Groups[i].Rules = append(builder.Spec.Groups[i].Rules,
				customthanosrules.ThanosRuleGroupEvaluationFailuresRule(
					rhobsNextServiceLabel,
					dashboardThanosRule,
					"https://gitlab.cee.redhat.com/rhobs/configuration/-/blob/main/runbooks/thanos.md#thanosrulehighruleevaluationfailures",
				),
			)
			break
		}
	}

	return &appInterfacePrometheusRule{
		Schema:         schemaPath,
		PrometheusRule: builder.PrometheusRule,
	}
}

func ThanosOperatorPrometheusRule(nonCriticalPostProcessing bool) *appInterfacePrometheusRule {
	builder, err := thanosoperatorrules.NewThanosOperatorRulesBuilder(
		"",
		map[string]string{
			"app.kubernetes.io/component": "thanos-operator",
			"app.kubernetes.io/name":      "thanos-operator-rules",
			"app.kubernetes.io/part-of":   rhobsNextServiceLabel,
			"app.kubernetes.io/version":   "main",
			"prometheus":                  "app-sre",
			"role":                        "alert-rules",
		},
		map[string]string{},
		thanosoperatorrules.WithRunbookURL("https://gitlab.cee.redhat.com/rhobs/configuration/-/tree/main/runbooks/thanos-operator.md"),
		thanosoperatorrules.WithServiceLabelValue(rhobsNextServiceLabel),
		thanosoperatorrules.WithDashboardURL(dashboardThanosOperator),
	)

	if err != nil {
		return nil
	}

	if nonCriticalPostProcessing {
		builder.PrometheusRule = RuleNonCriticalPostProcessing(builder.PrometheusRule)
	} else {
		builder.PrometheusRule = RuleCriticalPostProcessing(builder.PrometheusRule)
	}
	builder.Spec.Groups = ReplaceSummaryWithMessage(builder.Spec.Groups)

	return &appInterfacePrometheusRule{
		Schema:         schemaPath,
		PrometheusRule: builder.PrometheusRule,
	}
}

func AlertmanagerPrometheusRule(nonCriticalPostProcessing bool) *appInterfacePrometheusRule {
	builder, err := alertmanagerrules.NewAlertmanagerRulesBuilder(
		"",
		map[string]string{
			"app.kubernetes.io/component": "alertmanager",
			"app.kubernetes.io/name":      "alertmanager-rules",
			"app.kubernetes.io/part-of":   rhobsNextServiceLabel,
			"app.kubernetes.io/version":   "main",
			"prometheus":                  "app-sre",
			"role":                        "alert-rules",
		},
		map[string]string{},
		alertmanagerrules.WithRunbookURL("https://gitlab.cee.redhat.com/rhobs/configuration/-/tree/main/runbooks/alertmanager.md"),
		alertmanagerrules.WithServiceLabelValue(rhobsNextServiceLabel),
		alertmanagerrules.WithCriticalIntegrationSelectorRegexp("slack|pagerduty|email|webhook"),
		alertmanagerrules.WithNonCriticalIntegrationSelectorRegexp("slack|pagerduty|email|webhook"),
		alertmanagerrules.WithDashboardURL(dashboardAlertmanager),
	)

	if err != nil {
		return nil
	}

	if nonCriticalPostProcessing {
		builder.PrometheusRule = RuleNonCriticalPostProcessing(builder.PrometheusRule)
	} else {
		builder.PrometheusRule = RuleCriticalPostProcessing(builder.PrometheusRule)
	}
	builder.Spec.Groups = ReplaceSummaryWithMessage(builder.Spec.Groups)

	return &appInterfacePrometheusRule{
		Schema:         schemaPath,
		PrometheusRule: builder.PrometheusRule,
	}
}

func LokiPrometheusRule(nonCriticalPostProcessing bool) *appInterfacePrometheusRule {
	builder, err := lokirules.NewLokiRulesBuilder(
		"",
		map[string]string{
			"app.kubernetes.io/component": "loki",
			"app.kubernetes.io/name":      "loki-rules",
			"app.kubernetes.io/part-of":   rhobsNextServiceLabel,
			"app.kubernetes.io/version":   "main",
			"prometheus":                  "app-sre",
			"role":                        "alert-rules",
		},
		map[string]string{},
		lokirules.WithRunbookURL("https://gitlab.cee.redhat.com/rhobs/configuration/-/tree/main/runbooks/loki.md"),
		lokirules.WithServiceLabelValue(rhobsNextServiceLabel),
	)
	if err != nil {
		return nil
	}

	if nonCriticalPostProcessing {
		builder.PrometheusRule = RuleNonCriticalPostProcessing(builder.PrometheusRule)
	} else {
		builder.PrometheusRule = RuleCriticalPostProcessing(builder.PrometheusRule)
	}
	builder.Spec.Groups = ReplaceSummaryWithMessage(builder.Spec.Groups)

	return &appInterfacePrometheusRule{
		Schema:         schemaPath,
		PrometheusRule: builder.PrometheusRule,
	}
}

func RuleCriticalPostProcessing(rule v1.PrometheusRule) v1.PrometheusRule {
	keepCriticalAlerts := map[string]bool{
		"LokiIngesterFlushFailureRateCritical": true,
	}
	for i := range rule.Spec.Groups {
		for j := range rule.Spec.Groups[i].Rules {
			if v, ok := rule.Spec.Groups[i].Rules[j].Labels["severity"]; ok {
				// Downgrade critical to warning for critical rule path
				if v == "critical" {
					if !keepCriticalAlerts[rule.Spec.Groups[i].Rules[j].Alert] {
						rule.Spec.Groups[i].Rules[j].Labels["severity"] = "warning"
					}
				}
			}
		}
	}
	return rule
}

func RuleNonCriticalPostProcessing(rule v1.PrometheusRule) v1.PrometheusRule {
	for i := range rule.Spec.Groups {
		for j := range rule.Spec.Groups[i].Rules {
			if v, ok := rule.Spec.Groups[i].Rules[j].Labels["severity"]; ok {
				switch v {
				case "critical":
					rule.Spec.Groups[i].Rules[j].Labels["severity"] = "high"
				case "warning":
					rule.Spec.Groups[i].Rules[j].Labels["severity"] = "medium"
				}
			}
		}
	}
	return rule
}

// Usually summary + description is the best practice, but this is so that we are compatible with app-interface schema.
// To be removed once https://issues.redhat.com/browse/APPSRE-7834 is closed
func ReplaceSummaryWithMessage(groups []v1.RuleGroup) []v1.RuleGroup {
	for i := range groups {
		for j := range groups[i].Rules {
			if groups[i].Rules[j].Annotations["summary"] != "" {
				groups[i].Rules[j].Annotations["message"] = groups[i].Rules[j].Annotations["summary"]
				delete(groups[i].Rules[j].Annotations, "summary")
			}
		}
	}
	return groups
}

// ReplaceStoreRhobsWithDefault replaces "thanos-store-rhobs" with "thanos-storet"
// in rule expressions to use the correct store matcher.
func ReplaceStoreRhobsWithDefault(groups []v1.RuleGroup) []v1.RuleGroup {
	for i := range groups {
		for j := range groups[i].Rules {
			expr := groups[i].Rules[j].Expr.String()
			if strings.Contains(expr, "thanos-store-rhobs") {
				newExpr := strings.ReplaceAll(expr, "thanos-store-rhobs", "thanos-store-")
				groups[i].Rules[j].Expr = intstr.FromString(newExpr)
			}
		}
	}
	return groups
}

func (b Build) SyntheticsRules() {
	gen := b.o11yGenerator("synthetics-rules")
	syntheticsRules(gen)
}

func syntheticsRules(gen *mimic.Generator) {
	gen.Add("synthetics-api-rules.yaml", encoding.GhodssYAML("", SyntheticsAPIPrometheusRule(false)))
	gen.Add("synthetics-api-rules-non-critical.yaml", encoding.GhodssYAML("", SyntheticsAPIPrometheusRule(true)))
	gen.Add("synthetics-agent-rules.yaml", encoding.GhodssYAML("", SyntheticsAgentPrometheusRule(false)))
	gen.Add("synthetics-agent-rules-non-critical.yaml", encoding.GhodssYAML("", SyntheticsAgentPrometheusRule(true)))
	gen.Add("synthetics-blackbox-exporter-rules.yaml", encoding.GhodssYAML("", SyntheticsBlackboxExporterPrometheusRule(false)))
	gen.Add("synthetics-blackbox-exporter-rules-non-critical.yaml", encoding.GhodssYAML("", SyntheticsBlackboxExporterPrometheusRule(true)))
	gen.Generate()
}

func SyntheticsAPIPrometheusRule(nonCriticalPostProcessing bool) *appInterfacePrometheusRule {
	builder, err := syntheticsrules.NewSyntheticsAPIRulesBuilder(
		"",
		map[string]string{
			"app.kubernetes.io/component": "synthetics-api",
			"app.kubernetes.io/name":      "synthetics-api-rules",
			"app.kubernetes.io/part-of":   rhobsNextServiceLabel,
			"app.kubernetes.io/version":   "main",
			"prometheus":                  "app-sre",
			"role":                        "alert-rules",
		},
		map[string]string{},
		syntheticsrules.WithServiceLabelValue(rhobsNextServiceLabel),
	)
	if err != nil {
		return nil
	}

	if nonCriticalPostProcessing {
		builder.PrometheusRule = RuleNonCriticalPostProcessing(builder.PrometheusRule)
	} else {
		builder.PrometheusRule = RuleCriticalPostProcessing(builder.PrometheusRule)
	}
	builder.Spec.Groups = ReplaceSummaryWithMessage(builder.Spec.Groups)

	return &appInterfacePrometheusRule{
		Schema:         schemaPath,
		PrometheusRule: builder.PrometheusRule,
	}
}

func SyntheticsBlackboxExporterPrometheusRule(nonCriticalPostProcessing bool) *appInterfacePrometheusRule {
	builder, err := syntheticsrules.NewSyntheticsBlackboxExporterRulesBuilder(
		"",
		map[string]string{
			"app.kubernetes.io/component": "synthetics-blackbox-exporter",
			"app.kubernetes.io/name":      "synthetics-blackbox-exporter-rules",
			"app.kubernetes.io/part-of":   rhobsNextServiceLabel,
			"app.kubernetes.io/version":   "main",
			"prometheus":                  "app-sre",
			"role":                        "alert-rules",
		},
		map[string]string{},
		syntheticsrules.WithServiceLabelValue(rhobsNextServiceLabel),
	)
	if err != nil {
		return nil
	}

	if nonCriticalPostProcessing {
		builder.PrometheusRule = RuleNonCriticalPostProcessing(builder.PrometheusRule)
	} else {
		builder.PrometheusRule = RuleCriticalPostProcessing(builder.PrometheusRule)
	}
	builder.Spec.Groups = ReplaceSummaryWithMessage(builder.Spec.Groups)

	return &appInterfacePrometheusRule{
		Schema:         schemaPath,
		PrometheusRule: builder.PrometheusRule,
	}
}

func SyntheticsAgentPrometheusRule(nonCriticalPostProcessing bool) *appInterfacePrometheusRule {
	builder, err := syntheticsrules.NewSyntheticsAgentRulesBuilder(
		"",
		map[string]string{
			"app.kubernetes.io/component": "synthetics-agent",
			"app.kubernetes.io/name":      "synthetics-agent-rules",
			"app.kubernetes.io/part-of":   rhobsNextServiceLabel,
			"app.kubernetes.io/version":   "main",
			"prometheus":                  "app-sre",
			"role":                        "alert-rules",
		},
		map[string]string{},
		syntheticsrules.WithServiceLabelValue(rhobsNextServiceLabel),
	)
	if err != nil {
		return nil
	}

	if nonCriticalPostProcessing {
		builder.PrometheusRule = RuleNonCriticalPostProcessing(builder.PrometheusRule)
	} else {
		builder.PrometheusRule = RuleCriticalPostProcessing(builder.PrometheusRule)
	}
	builder.Spec.Groups = ReplaceSummaryWithMessage(builder.Spec.Groups)

	return &appInterfacePrometheusRule{
		Schema:         schemaPath,
		PrometheusRule: builder.PrometheusRule,
	}
}
