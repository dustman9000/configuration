package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/bwplotka/mimic"
	"github.com/bwplotka/mimic/encoding"
	kitlog "github.com/go-kit/log"
	lokiv1 "github.com/grafana/loki/operator/api/loki/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"gitlab.cee.redhat.com/rhobs/configuration/clusters"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	lokiStackName = "observatorium-lokistack"
	// lokiRulesInstanceLabelKey is the Loki operator label linking AlertingRule/RecordingRule CRs to this LokiStack (not Thanos PrometheusRule labels).
	lokiRulesInstanceLabelKey = "loki.grafana.com/loki-rule"
)

func (b Build) DefaultLokiStack(config clusters.ClusterConfig) error {
	return generateLogsBundle(config)
}

// NewBundleLokiStack creates a LokiStack with concrete values for bundle deployment (no template parameters).
// rules sets spec.rules (ruler enablement, LokiRule selector, PrometheusRule namespace selection). If nil, ruler is disabled.
func NewBundleLokiStack(namespace string, overrides clusters.TemplateMaps, rules *lokiv1.RulesSpec) *lokiv1.LokiStack {
	templateSpec := &lokiv1.LokiTemplateSpec{
		Distributor: &lokiv1.LokiComponentSpec{
			Replicas: overrides.LokiOverrides[clusters.LokiConfig].Router.Replicas,
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						TopologyKey: "kubernetes.io/hostname",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app.kubernetes.io/component":  "distributor",
								"app.kubernetes.io/created-by": "observatorium-lokistack",
							},
						},
					},
				},
			},
		},
		Ingester: &lokiv1.LokiComponentSpec{
			Replicas: overrides.LokiOverrides[clusters.LokiConfig].Ingest.Replicas,
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						TopologyKey: "kubernetes.io/hostname",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app.kubernetes.io/component":  "ingester",
								"app.kubernetes.io/created-by": "observatorium-lokistack",
							},
						},
					},
					{

						TopologyKey: "kubernetes.io/hostname",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app.kubernetes.io/component": "thanos-receive-ingester",
							},
						},
					},
				},
			},
		},
		Querier: &lokiv1.LokiComponentSpec{
			Replicas: overrides.LokiOverrides[clusters.LokiConfig].Query.Replicas,
		},
		QueryFrontend: &lokiv1.LokiComponentSpec{
			Replicas: overrides.LokiOverrides[clusters.LokiConfig].QueryFrontend.Replicas,
		},
	}
	if rules != nil && rules.Enabled {
		templateSpec.Ruler = &lokiv1.LokiComponentSpec{
			Replicas: overrides.LokiOverrides[clusters.LokiConfig].Ruler.Replicas,
		}
	}

	if rules == nil {
		rules = &lokiv1.RulesSpec{Enabled: false}
	}
	return &lokiv1.LokiStack{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "loki.grafana.com/v1",
			Kind:       "LokiStack",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      lokiStackName,
			Namespace: namespace,
		},
		Spec: lokiv1.LokiStackSpec{
			Limits: &lokiv1.LimitsSpec{
				Global: &lokiv1.LimitsTemplateSpec{
					IngestionLimits: &lokiv1.IngestionLimitSpec{
						IngestionRate:             overrides.LokiOverrides[clusters.LokiConfig].IngestionRateLimitMB,
						IngestionBurstSize:        overrides.LokiOverrides[clusters.LokiConfig].IngestionBurstSizeMB,
						MaxLineSize:               overrides.LokiOverrides[clusters.LokiConfig].MaxLineSize,
						PerStreamRateLimit:        overrides.LokiOverrides[clusters.LokiConfig].PerStreamRateLimitMB,
						PerStreamRateLimitBurst:   overrides.LokiOverrides[clusters.LokiConfig].PerStreamBurstSizeMB,
						MaxGlobalStreamsPerTenant: overrides.LokiOverrides[clusters.LokiConfig].MaxGlobalStreamsPerTenant,
					},
					QueryLimits: &lokiv1.QueryLimitSpec{
						QueryTimeout: overrides.LokiOverrides[clusters.LokiConfig].QueryTimeout,
					},
					OTLP: &lokiv1.OTLPSpec{
						StreamLabels: &lokiv1.OTLPStreamLabelSpec{
							ResourceAttributes: []lokiv1.OTLPAttributeReference{
								{
									Name: "k8s.namespace.name",
								},
								{
									Name: "openshift.label.cluster_name",
								},
								{
									Name: "openshift.log.source",
								},
								{
									Name: "openshift.log.type",
								},
							},
						},
					},
					Retention: &lokiv1.RetentionLimitSpec{
						Days: 90,
					},
				},
			},
			ManagementState: lokiv1.ManagementStateManaged,
			Size:            "1x.extra-small",
			Storage: lokiv1.ObjectStorageSpec{
				Schemas: []lokiv1.ObjectStorageSchema{
					{
						EffectiveDate: "2025-06-06",
						Version:       lokiv1.ObjectStorageSchemaV13,
					},
				},
				Secret: lokiv1.ObjectStorageSecretSpec{
					Name: "loki-default-bucket",
					Type: "s3",
				},
			},
			StorageClassName: "gp3-csi", // Concrete value instead of ${LOKI_STORAGE_CLASS}
			Template:         templateSpec,
			Rules:            rules,
		},
	}
}

// generateLogsBundle generates individual Loki component resources for bundle deployment
func generateLogsBundle(config clusters.ClusterConfig) error {
	ns := config.Namespace

	// Create bundle generator for individual resource files
	bundleGen := &mimic.Generator{}
	bundleGen = bundleGen.With("resources", "clusters", string(config.Environment), string(config.Name), "logs", "bundle")
	bundleGen.Logger = kitlog.NewLogfmtLogger(kitlog.NewSyncWriter(os.Stdout))

	// 1. CRDs (prefix: 01-*)
	crdObjs := getLokiCRDObjects()
	crdNames := []string{"projectconfigs", "alertingrules", "lokistacks", "recordingrules", "rulerconfigs"}
	for i, crd := range crdObjs {
		crdName := "unknown"
		if i < len(crdNames) {
			crdName = crdNames[i]
		}
		filename := fmt.Sprintf("01-crd-%s.yaml", crdName)
		bundleGen.Add(filename, encoding.GhodssYAML(crd))
	}

	// 2. OPERATOR (prefix: 02-*)
	operatorObjs := lokiOperatorResources(ns)
	for i, obj := range operatorObjs {
		resourceKind := getResourceKind(obj)
		resourceName := getLokiResourceName(obj)
		filename := fmt.Sprintf("02-operator-%02d-%s-%s.yaml", i+1, resourceName, resourceKind)
		bundleGen.Add(filename, encoding.GhodssYAML(obj))
	}

	rulesSpec, rulerConfig, err := newRulerResources(ns, config.LoggingRulerMode())
	if err != nil {
		return err
	}

	// 3. LOKISTACK RESOURCES (prefix: 03-*)
	lokiStackObjs := make([]runtime.Object, 0, 2)
	lokiStackObjs = append(lokiStackObjs, NewBundleLokiStack(ns, config.Templates, rulesSpec))
	if rulerConfig != nil {
		lokiStackObjs = append(lokiStackObjs, rulerConfig)
	}

	for _, obj := range lokiStackObjs {
		resourceKind := getResourceKind(obj)
		resourceName := getLokiResourceName(obj)
		// Clean up names and remove redundant prefixes
		resourceName = strings.TrimPrefix(resourceName, "observatorium-")
		filename := fmt.Sprintf("03-%s-%s.yaml", resourceName, resourceKind)
		bundleGen.Add(filename, encoding.GhodssYAML(obj))
	}

	// Generate the bundle files
	bundleGen.Generate()

	// Add consolidated ServiceMonitors to monitoring bundle
	monBundle := GetMonitoringBundle(config)
	lokiServiceMonitors := createConsolidatedLokiServiceMonitors()

	for _, sm := range lokiServiceMonitors {
		if smObj, ok := sm.(*monitoringv1.ServiceMonitor); ok && smObj != nil {
			monBundle.AddServiceMonitor(smObj)
		}
	}

	return nil
}

// getLokiCRDObjects retrieves Loki operator CRDs
func getLokiCRDObjects() []runtime.Object {
	const (
		projectconfigs = "config.grafana.com_projectconfigs.yaml"
		alertingrules  = "loki.grafana.com_alertingrules.yaml"
		lokistacks     = "loki.grafana.com_lokistacks.yaml"
		recordingrules = "loki.grafana.com_recordingrules.yaml"
		rulerconfigs   = "loki.grafana.com_rulerconfigs.yaml"
		base           = "https://raw.githubusercontent.com/openshift/loki/" + lokiOperatorCRDRef + "/operator/config/crd/bases/"
	)

	var objs []runtime.Object
	for _, component := range []string{projectconfigs, alertingrules, lokistacks, recordingrules, rulerconfigs} {
		crd, err := getCustomResourceDefinition(base + component)
		if err != nil {
			log.Printf("Error fetching CRD %s: %v", component, err)
			continue
		}
		objs = append(objs, crd)
	}
	return objs
}

// getLokiResourceName extracts a meaningful name from a Loki Kubernetes object
func getLokiResourceName(obj runtime.Object) string {
	if obj == nil {
		return "unknown"
	}

	switch o := obj.(type) {
	case metav1.Object:
		name := o.GetName()
		if name != "" {
			// Remove redundant loki prefix since it's implied by being in the logs bundle
			name = strings.TrimPrefix(name, "loki-")
			return name
		}
	}

	// Fallback to the object type
	return "unnamed"
}

// newRulerResources builds spec.rules for the LokiStack and, when the ruler is enabled, a RulerConfig.
// AlertingRule / RecordingRule / LokiRule CRs must include label lokiRulesInstanceLabelKey=lokiStackName.
func newRulerResources(namespace string, rulerMode clusters.LokiRulerMode) (*lokiv1.RulesSpec, *lokiv1.RulerConfig, error) {
	if !rulerMode.Enabled() {
		return &lokiv1.RulesSpec{Enabled: false}, nil, nil
	}
	rules := &lokiv1.RulesSpec{
		Enabled: true,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				lokiRulesInstanceLabelKey: "true",
			},
		},
	}

	return rules, newBundleLokiRulerConfig(namespace, rulerMode), nil
}

// newBundleLokiRulerConfig builds RulerConfig. When recordingRemoteWrite is true, tenantID must be the HCP
// gateway tenant for THANOS-TENANT; when false, only Alertmanager is configured (log-based alerts only).
func newBundleLokiRulerConfig(namespace string, rulerMode clusters.LokiRulerMode) *lokiv1.RulerConfig {
	spec := lokiv1.RulerConfigSpec{
		AlertManagerSpec: &lokiv1.AlertManagerSpec{
			EnableV2: true,
			Endpoints: []string{
				fmt.Sprintf("http://alertmanager-0.alertmanager-cluster.%s.svc.cluster.local:9093", namespace),
				fmt.Sprintf("http://alertmanager-1.alertmanager-cluster.%s.svc.cluster.local:9093", namespace),
			},
			// Rename the built-in "tenantId" label to "tenant_id" until we
			// have the possibility to customize the tenant ID label in the
			// Ruler definition.
			RelabelConfigs: []lokiv1.RelabelConfig{
				{
					SourceLabels: []string{"tenantId"},
					TargetLabel:  "tenant_id",
				},
				{
					SourceLabels: []string{},
					Regex:        "tenantId",
					Action:       "labeldrop",
				},
			},
		},
	}

	return &lokiv1.RulerConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "loki.grafana.com/v1",
			Kind:       "RulerConfig",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      lokiStackName,
			Namespace: namespace,
		},
		Spec: spec,
	}
}

// createConsolidatedLokiServiceMonitors creates ServiceMonitors for Loki components
func createConsolidatedLokiServiceMonitors() []runtime.Object {
	return []runtime.Object{
		// Loki Operator Controller Manager
		&monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "ServiceMonitor",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "loki-operator-controller-manager-metrics",
				Labels: map[string]string{
					"app.kubernetes.io/component":  "monitoring",
					"app.kubernetes.io/created-by": "loki-operator",
					"app.kubernetes.io/instance":   "controller-manager-metrics",
					"app.kubernetes.io/managed-by": "rhobs",
					"app.kubernetes.io/name":       "servicemonitor",
					"app.kubernetes.io/part-of":    "loki-operator",
				},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{
					{
						Port: "metrics",
					},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/component": "metrics",
					},
				},
			},
		},
		// Loki Compactor
		&monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "ServiceMonitor",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "loki-compactor-http",
				Labels: map[string]string{
					"app.kubernetes.io/component":  "compactor",
					"app.kubernetes.io/created-by": "lokistack-controller",
					"app.kubernetes.io/instance":   "observatorium-lokistack",
					"app.kubernetes.io/managed-by": "lokistack-controller",
					"app.kubernetes.io/name":       "lokistack",
				},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{
					{
						Interval: "30s",
						Port:     "metrics",
					},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/component":  "compactor",
						"app.kubernetes.io/created-by": "lokistack-controller",
						"app.kubernetes.io/instance":   "observatorium-lokistack",
						"app.kubernetes.io/managed-by": "lokistack-controller",
						"app.kubernetes.io/name":       "lokistack",
					},
				},
			},
		},
		// Loki Distributor
		&monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "ServiceMonitor",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "loki-distributor-http",
				Labels: map[string]string{
					"app.kubernetes.io/component":  "distributor",
					"app.kubernetes.io/created-by": "lokistack-controller",
					"app.kubernetes.io/instance":   "observatorium-lokistack",
					"app.kubernetes.io/managed-by": "lokistack-controller",
					"app.kubernetes.io/name":       "lokistack",
				},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{
					{
						Interval: "30s",
						Port:     "metrics",
					},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/component":  "distributor",
						"app.kubernetes.io/created-by": "lokistack-controller",
						"app.kubernetes.io/instance":   "observatorium-lokistack",
						"app.kubernetes.io/managed-by": "lokistack-controller",
						"app.kubernetes.io/name":       "lokistack",
					},
				},
			},
		},
		// Loki Index Gateway
		&monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "ServiceMonitor",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "loki-index-gateway-http",
				Labels: map[string]string{
					"app.kubernetes.io/component":  "index-gateway",
					"app.kubernetes.io/created-by": "lokistack-controller",
					"app.kubernetes.io/instance":   "observatorium-lokistack",
					"app.kubernetes.io/managed-by": "lokistack-controller",
					"app.kubernetes.io/name":       "lokistack",
				},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{
					{
						Interval: "30s",
						Port:     "metrics",
					},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/component":  "index-gateway",
						"app.kubernetes.io/created-by": "lokistack-controller",
						"app.kubernetes.io/instance":   "observatorium-lokistack",
						"app.kubernetes.io/managed-by": "lokistack-controller",
						"app.kubernetes.io/name":       "lokistack",
					},
				},
			},
		},
		// Loki Ingester
		&monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "ServiceMonitor",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "loki-ingester-http",
				Labels: map[string]string{
					"app.kubernetes.io/component":  "ingester",
					"app.kubernetes.io/created-by": "lokistack-controller",
					"app.kubernetes.io/instance":   "observatorium-lokistack",
					"app.kubernetes.io/managed-by": "lokistack-controller",
					"app.kubernetes.io/name":       "lokistack",
				},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{
					{
						Interval: "30s",
						Port:     "metrics",
					},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/component":  "ingester",
						"app.kubernetes.io/created-by": "lokistack-controller",
						"app.kubernetes.io/instance":   "observatorium-lokistack",
						"app.kubernetes.io/managed-by": "lokistack-controller",
						"app.kubernetes.io/name":       "lokistack",
					},
				},
			},
		},
		// Loki Querier
		&monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "ServiceMonitor",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "loki-querier-http",
				Labels: map[string]string{
					"app.kubernetes.io/component":  "querier",
					"app.kubernetes.io/created-by": "lokistack-controller",
					"app.kubernetes.io/instance":   "observatorium-lokistack",
					"app.kubernetes.io/managed-by": "lokistack-controller",
					"app.kubernetes.io/name":       "lokistack",
				},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{
					{
						Interval: "30s",
						Port:     "metrics",
					},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/component":  "querier",
						"app.kubernetes.io/created-by": "lokistack-controller",
						"app.kubernetes.io/instance":   "observatorium-lokistack",
						"app.kubernetes.io/managed-by": "lokistack-controller",
						"app.kubernetes.io/name":       "lokistack",
					},
				},
			},
		},
		// Loki Query Frontend
		&monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "ServiceMonitor",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "loki-query-frontend-http",
				Labels: map[string]string{
					"app.kubernetes.io/component":  "query-frontend",
					"app.kubernetes.io/created-by": "lokistack-controller",
					"app.kubernetes.io/instance":   "observatorium-lokistack",
					"app.kubernetes.io/managed-by": "lokistack-controller",
					"app.kubernetes.io/name":       "lokistack",
				},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{
					{
						Interval: "30s",
						Port:     "metrics",
					},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/component":  "query-frontend",
						"app.kubernetes.io/created-by": "lokistack-controller",
						"app.kubernetes.io/instance":   "observatorium-lokistack",
						"app.kubernetes.io/managed-by": "lokistack-controller",
						"app.kubernetes.io/name":       "lokistack",
					},
				},
			},
		},
		// Loki Ruler
		&monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "monitoring.coreos.com/v1",
				Kind:       "ServiceMonitor",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "loki-ruler-http",
				Labels: map[string]string{
					"app.kubernetes.io/component":  "ruler",
					"app.kubernetes.io/created-by": "lokistack-controller",
					"app.kubernetes.io/instance":   "observatorium-lokistack",
					"app.kubernetes.io/managed-by": "lokistack-controller",
					"app.kubernetes.io/name":       "lokistack",
				},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{
					{
						Interval: "30s",
						Port:     "metrics",
					},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/component":  "ruler",
						"app.kubernetes.io/created-by": "lokistack-controller",
						"app.kubernetes.io/instance":   "observatorium-lokistack",
						"app.kubernetes.io/managed-by": "lokistack-controller",
						"app.kubernetes.io/name":       "lokistack",
					},
				},
			},
		},
	}
}
