package thanosrules

import (
	"time"

	promqlbuilder "github.com/perses/promql-builder"
	"github.com/perses/promql-builder/label"
	"github.com/perses/promql-builder/matrix"
	"github.com/perses/promql-builder/vector"
	v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ThanosRuleGroupEvaluationFailuresRule returns the ThanosRuleGroupEvaluationFailures alert rule.
// The upstream mixin's ThanosRuleHighRuleEvaluationFailures uses a 5% ratio threshold which is too
// high to catch a single failing rule out of many. This alert fires per rule_group when sustained
// failures are detected.
func ThanosRuleGroupEvaluationFailuresRule(serviceLabelValue, dashboardURL, runbookURL string) v1.Rule {
	forDuration := v1.Duration("30m")

	expr := promqlbuilder.Gtr(
		promqlbuilder.Mul(
			promqlbuilder.Parenthesis(
				promqlbuilder.Div(
					promqlbuilder.Sum(
						promqlbuilder.Rate(
							matrix.New(
								vector.New(
									vector.WithMetricName("prometheus_rule_evaluation_failures_total"),
									vector.WithLabelMatchers(label.New("job").EqualRegexp("thanos-ruler-rhobs.*")),
								),
								matrix.WithRange(5*time.Minute),
							),
						),
					).By("namespace", "job", "instance", "rule_group"),
					promqlbuilder.Sum(
						promqlbuilder.Rate(
							matrix.New(
								vector.New(
									vector.WithMetricName("prometheus_rule_evaluations_total"),
									vector.WithLabelMatchers(label.New("job").EqualRegexp("thanos-ruler-rhobs.*")),
								),
								matrix.WithRange(5*time.Minute),
							),
						),
					).By("namespace", "job", "instance", "rule_group"),
				),
			),
			promqlbuilder.NewNumber(100),
		),
		promqlbuilder.NewNumber(5),
	)

	return v1.Rule{
		Alert: "ThanosRuleGroupEvaluationFailures",
		Expr:  intstr.FromString(expr.Pretty(0)),
		For:   &forDuration,
		Labels: map[string]string{
			"service":    serviceLabelValue,
			"severity":   "high",
			"rule_group": "{{ $labels.rule_group }}",
		},
		Annotations: map[string]string{
			"dashboard":   dashboardURL,
			"description": "Thanos Rule group {{ $labels.rule_group }} on {{ $labels.instance }} in {{ $labels.namespace }} has sustained evaluation failures. A rule in this group is consistently failing to evaluate, which means alerts from this group will never fire.",
			"message":     "Thanos Rule group has sustained evaluation failures.",
			"runbook":     runbookURL,
		},
	}
}
