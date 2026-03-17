package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/bwplotka/mimic"
	"github.com/bwplotka/mimic/encoding"
	kitlog "github.com/go-kit/log"
	kghelpers "github.com/observatorium/observatorium/configuration_go/kubegen/helpers"
	routev1 "github.com/openshift/api/route/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"gitlab.cee.redhat.com/rhobs/configuration/clusters"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func (b Build) DefaultThanosStack(config clusters.ClusterConfig) error {
	return generateMetricsBundle(config)
}

func TmpRulerCR(namespace string, templates clusters.TemplateMaps) *v1alpha1.ThanosRuler {
	return &v1alpha1.ThanosRuler{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosRuler",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "telemeter",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosRulerSpec{
			CommonFields: v1alpha1.CommonFields{
				Image:                ptr.To(clusters.TemplateFn("RULER", templates.Images)),
				Version:              ptr.To(clusters.TemplateFn("RULER", templates.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn("RULER", templates.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn("RULER", templates.ResourceRequirements)),
			},
			Replicas: clusters.TemplateFn("RULER", templates.Replicas),
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn("RULER", templates.StorageSize),
			},
			RuleConfigSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"operator.thanos.io/prometheus-rule": "true",
				},
			},
			QueryLabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"operator.thanos.io/query-api": "true",
					"app.kubernetes.io/part-of":    "thanos",
				},
			},
			RuleTenancyConfig: &v1alpha1.RuleTenancyConfig{
				EnforcedTenantIdentifier: ptr.To("tenant_id"),
				TenantSpecifierLabel:     ptr.To("operator.thanos.io/tenant"),
			},
			ObjectStorageConfig: clusters.TemplateFn("TELEMETER", templates.ObjectStorageBucket),
			ExternalLabels: map[string]string{
				"rule_replica": "$(NAME)",
			},
			AlertmanagerURL:    "dnssrv+http://alertmanager-cluster." + namespace + ".svc.cluster.local:9093",
			AlertLabelDrop:     []string{"rule_replica"},
			Retention:          v1alpha1.Duration("2h"),
			EvaluationInterval: v1alpha1.Duration("1m"),
			Additional: v1alpha1.Additional{
				Args: []string{},
			},
		},
	}
}

func defaultQueryCR(namespace string, templates clusters.TemplateMaps, oauth bool, withAdditionalArgs ...string) []runtime.Object {
	var objs []runtime.Object

	query := &v1alpha1.ThanosQuery{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosQuery",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rhobs",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosQuerySpec{
			Additional: v1alpha1.Additional{
				Args: withAdditionalArgs,
			},
			CommonFields: v1alpha1.CommonFields{
				Image:                ptr.To(clusters.TemplateFn(clusters.Query, templates.Images)),
				Version:              ptr.To(clusters.TemplateFn(clusters.Query, templates.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn(clusters.Query, templates.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn(clusters.Query, templates.ResourceRequirements)),
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
				Affinity: &corev1.Affinity{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
							{
								Weight: 100,
								PodAffinityTerm: corev1.PodAffinityTerm{
									TopologyKey: "kubernetes.io/hostname",
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"app.kubernetes.io/component": "query-layer",
											"app.kubernetes.io/instance":  "thanos-query-rhobs",
										},
									},
								},
							},
						},
					},
				},
				PodDisruptionBudgetConfig: &v1alpha1.PodDisruptionBudgetConfig{
					Enable: ptr.To(false),
				},
			},
			StoreLabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"operator.thanos.io/store-api": "true",
					"app.kubernetes.io/part-of":    "thanos",
				},
			},
			Replicas: clusters.TemplateFn(clusters.Query, templates.Replicas),
			ReplicaLabels: []string{
				"prometheus_replica",
				"replica",
				"rule_replica",
			},
			WebConfig: &v1alpha1.WebConfig{
				PrefixHeader: ptr.To("X-Forwarded-Prefix"),
			},
			GRPCProxyStrategy: "lazy",
			TelemetryQuantiles: &v1alpha1.TelemetryQuantiles{
				Duration: []string{
					"0.1", "0.25", "0.75", "1.25", "1.75", "2.5", "3", "5", "10", "15", "30", "60", "120",
				},
			},
			QueryFrontend: &v1alpha1.QueryFrontendSpec{
				CommonFields: v1alpha1.CommonFields{
					Image:                ptr.To(clusters.TemplateFn("QUERY_FRONTEND", templates.Images)),
					Version:              ptr.To(clusters.TemplateFn("QUERY_FRONTEND", templates.Versions)),
					ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
					LogLevel:             ptr.To(clusters.TemplateFn("QUERY_FRONTEND", templates.LogLevels)),
					LogFormat:            ptr.To("logfmt"),
					ResourceRequirements: ptr.To(clusters.TemplateFn("QUERY_FRONTEND", templates.ResourceRequirements)),
					SecurityContext: &corev1.PodSecurityContext{
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: 100,
									PodAffinityTerm: corev1.PodAffinityTerm{
										TopologyKey: "kubernetes.io/hostname",
										LabelSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"app.kubernetes.io/component": "query-frontend",
												"app.kubernetes.io/instance":  "thanos-query-frontend-rhobs",
											},
										},
									},
								},
							},
						},
					},
				},
				Replicas:             clusters.TemplateFn("QUERY_FRONTEND", templates.Replicas),
				CompressResponses:    true,
				LogQueriesLongerThan: ptr.To(v1alpha1.Duration("10s")),
				LabelsMaxRetries:     3,
				QueryRangeMaxRetries: 3,
				QueryLabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"operator.thanos.io/query-api": "true",
					},
				},
				QueryRangeSplitInterval: ptr.To(v1alpha1.Duration("48h")),
				LabelsSplitInterval:     ptr.To(v1alpha1.Duration("48h")),
				LabelsDefaultTimeRange:  ptr.To(v1alpha1.Duration("336h")),
				QueryRangeResponseCacheConfig: &v1alpha1.CacheConfig{
					ExternalCacheConfig: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "thanos-query-range-cache-memcached",
						},
						Key: "thanos.yaml",
					},
				},
			},
		},
	}
	if oauth {
		route := &routev1.Route{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "route.openshift.io/v1",
				Kind:       "Route",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "thanos-query-frontend-rhobs",
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "thanos",
				},
			},
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{
					Kind:   "Service",
					Name:   "thanos-query-frontend-rhobs",
					Weight: ptr.To(int32(100)),
				},
				Port: &routev1.RoutePort{
					TargetPort: intstr.FromString("https"), // Assuming the oauth-proxy is exposing on https port
				},
				TLS: &routev1.TLSConfig{
					Termination:                   routev1.TLSTerminationReencrypt,
					InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
				},
			},
		}
		objs = append(objs, route)
		query.Spec.QueryFrontend.Annotations = map[string]string{
			"service.beta.openshift.io/serving-cert-secret-name":               "query-frontend-tls",
			"serviceaccounts.openshift.io/oauth-redirectreference.application": `{"kind":"OAuthRedirectReference","apiVersion":"v1","reference":{"kind":"Route","name":"thanos-query-frontend-rhobs"}}`,
		}
		query.Spec.QueryFrontend.ServicePorts = append(query.Spec.QueryFrontend.ServicePorts, corev1.ServicePort{
			Name: "https",
			Port: 8443,
			TargetPort: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: 8443,
			},
		})
		query.Spec.QueryFrontend.Containers = append(query.Spec.QueryFrontend.Containers, makeOauthProxyContainer(9090, namespace, "thanos-query-frontend-rhobs", "query-frontend-tls"))
		query.Spec.QueryFrontend.Volumes = append(query.Spec.QueryFrontend.Volumes, kghelpers.NewPodVolumeFromSecret("tls", "query-frontend-tls"))
		query.Spec.QueryFrontend.Volumes = append(query.Spec.QueryFrontend.Volumes, kghelpers.NewPodVolumeFromSecret("oauth-cookie", "oauth-cookie"))
	}

	objs = append(objs, query)
	return objs
}

func defaultStoreCR(namespace string, templates clusters.TemplateMaps) runtime.Object {
	return &v1alpha1.ThanosStore{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosStore",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosStoreSpec{
			CommonFields: v1alpha1.CommonFields{
				Image:                ptr.To(clusters.TemplateFn(clusters.StoreDefault, templates.Images)),
				Version:              ptr.To(clusters.TemplateFn(clusters.StoreDefault, templates.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn(clusters.StoreDefault, templates.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn(clusters.StoreDefault, templates.ResourceRequirements)),
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			},
			Replicas:            clusters.TemplateFn(clusters.StoreDefault, templates.Replicas),
			ObjectStorageConfig: clusters.TemplateFn(clusters.DefaultBucket, templates.ObjectStorageBucket),
			IndexCacheConfig: &v1alpha1.CacheConfig{
				ExternalCacheConfig: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "thanos-index-cache-memcached",
					},
					Key: "thanos.yaml",
				},
			},
			CachingBucketConfig: &v1alpha1.CacheConfig{
				ExternalCacheConfig: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "thanos-bucket-cache-memcached",
					},
					Key: "thanos.yaml",
				},
			},
			ShardingStrategy: v1alpha1.ShardingStrategy{
				Type:   v1alpha1.Block,
				Shards: 3,
			},
			IndexHeaderConfig: &v1alpha1.IndexHeaderConfig{
				EnableLazyReader:      ptr.To(true),
				LazyDownloadStrategy:  ptr.To("lazy"),
				LazyReaderIdleTimeout: ptr.To(v1alpha1.Duration("5m")),
			},
			StoreLimitsOptions: &v1alpha1.StoreLimitsOptions{
				StoreLimitsRequestSamples: 0,
				StoreLimitsRequestSeries:  0,
			},
			BlockConfig: &v1alpha1.BlockConfig{
				BlockDiscoveryStrategy:    v1alpha1.BlockDiscoveryStrategy("concurrent"),
				BlockFilesConcurrency:     ptr.To(int32(1)),
				BlockMetaFetchConcurrency: ptr.To(int32(32)),
			},
			IgnoreDeletionMarksDelay: v1alpha1.Duration("24h"),
			TimeRangeConfig: &v1alpha1.TimeRangeConfig{
				MaxTime: ptr.To(v1alpha1.Duration("-22h")),
			},
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn(clusters.StoreDefault, templates.StorageSize),
			},
			Additional: v1alpha1.Additional{},
		},
	}
}

func defaultReceiveCR(namespace string, templates clusters.TemplateMaps) runtime.Object {
	grpcDisableEndlessRetry := `{
  "loadBalancingPolicy":"round_robin",
  "retryPolicy": {
    "maxAttempts": 0,
    "initialBackoff": "0.1s",
    "backoffMultiplier": 1,
    "retryableStatusCodes": [
  	  "UNAVAILABLE"
    ]
  }
}`

	hashrings := []v1alpha1.IngesterHashringSpec{
		{
			Name: "active-default",
			CommonFields: v1alpha1.CommonFields{
				Image:                ptr.To(clusters.TemplateFn(clusters.ReceiveIngestorDefault, templates.Images)),
				Version:              ptr.To(clusters.TemplateFn(clusters.ReceiveIngestorDefault, templates.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn(clusters.ReceiveIngestorDefault, templates.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn(clusters.ReceiveIngestorActiveDefault, templates.ResourceRequirements)),
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
				Affinity: &corev1.Affinity{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
							{
								Weight: 100,
								PodAffinityTerm: corev1.PodAffinityTerm{
									TopologyKey: "kubernetes.io/hostname",
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"app.kubernetes.io/component": "thanos-receive-ingester",
											"app.kubernetes.io/instance":  "thanos-receive-ingester-rhobs-active-default",
										},
									},
								},
							},
						},
					},
				},
			},
			ExternalLabels: map[string]string{
				"replica": "$(POD_NAME)",
			},
			Replicas: clusters.TemplateFn(clusters.ReceiveIngestorDefault, templates.Replicas),
			TSDBConfig: v1alpha1.TSDBConfig{
				Retention: v1alpha1.Duration("2h"),
			},
			AsyncForwardWorkerCount:  ptr.To(uint64(50)),
			TooFarInFutureTimeWindow: ptr.To(v1alpha1.Duration("5m")),
			StoreLimitsOptions: &v1alpha1.StoreLimitsOptions{
				StoreLimitsRequestSamples: 0,
				StoreLimitsRequestSeries:  0,
			},
			TenancyConfig: &v1alpha1.TenancyConfig{
				TenantMatcherType: "exact",
				DefaultTenantID:   "EFD08939-FE1D-41A1-A28A-BE9A9BC68003",
				TenantHeader:      "THANOS-TENANT",
				TenantLabelName:   "tenant_id",
			},
			ObjectStorageConfig: ptr.To(clusters.TemplateFn(clusters.DefaultBucket, templates.ObjectStorageBucket)),
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn(clusters.ReceiveIngestorDefault, templates.StorageSize),
			},
			HashingAlgorithm: ptr.To("ketama"),
		},
		{
			Name: "default",
			CommonFields: v1alpha1.CommonFields{
				Image:                ptr.To(clusters.TemplateFn(clusters.ReceiveIngestorDefault, templates.Images)),
				Version:              ptr.To(clusters.TemplateFn(clusters.ReceiveIngestorDefault, templates.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn(clusters.ReceiveIngestorDefault, templates.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn(clusters.ReceiveIngestorDefault, templates.ResourceRequirements)),
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
				Affinity: &corev1.Affinity{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
							{
								Weight: 100,
								PodAffinityTerm: corev1.PodAffinityTerm{
									TopologyKey: "kubernetes.io/hostname",
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"app.kubernetes.io/component": "thanos-receive-ingester",
											"app.kubernetes.io/instance":  "thanos-receive-ingester-rhobs-default",
										},
									},
								},
							},
						},
					},
				},
			},
			ExternalLabels: map[string]string{
				"replica": "$(POD_NAME)",
			},
			Replicas: clusters.TemplateFn(clusters.ReceiveIngestorDefault, templates.Replicas),
			TSDBConfig: v1alpha1.TSDBConfig{
				Retention: v1alpha1.Duration("2h"),
			},
			AsyncForwardWorkerCount:  ptr.To(uint64(50)),
			TooFarInFutureTimeWindow: ptr.To(v1alpha1.Duration("5m")),
			StoreLimitsOptions: &v1alpha1.StoreLimitsOptions{
				StoreLimitsRequestSamples: 0,
				StoreLimitsRequestSeries:  0,
			},
			TenancyConfig: &v1alpha1.TenancyConfig{
				TenantMatcherType: "exact",
				DefaultTenantID:   "FB870BF3-9F3A-44FF-9BF7-D7A047A52F43",
				TenantHeader:      "THANOS-TENANT",
				TenantLabelName:   "tenant_id",
			},
			ObjectStorageConfig: ptr.To(clusters.TemplateFn(clusters.DefaultBucket, templates.ObjectStorageBucket)),
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn(clusters.ReceiveIngestorDefault, templates.StorageSize),
			},
			HashingAlgorithm: ptr.To("hashmod"),
		},
	}

	if namespace != "rhobs-int" {
		hashrings = hashrings[1:] // only "default", not "active-default"
	}

	return &v1alpha1.ThanosReceive{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosReceive",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rhobs",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosReceiveSpec{
			Router: v1alpha1.RouterSpec{
				CommonFields: v1alpha1.CommonFields{
					Image:                ptr.To(clusters.TemplateFn(clusters.ReceiveRouter, templates.Images)),
					Version:              ptr.To(clusters.TemplateFn(clusters.ReceiveRouter, templates.Versions)),
					ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
					LogLevel:             ptr.To(clusters.TemplateFn(clusters.ReceiveRouter, templates.LogLevels)),
					LogFormat:            ptr.To("logfmt"),
					ResourceRequirements: ptr.To(clusters.TemplateFn(clusters.ReceiveRouter, templates.ResourceRequirements)),
					SecurityContext: &corev1.PodSecurityContext{
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					PodDisruptionBudgetConfig: &v1alpha1.PodDisruptionBudgetConfig{
						Enable: ptr.To(true),
					},
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: 100,
									PodAffinityTerm: corev1.PodAffinityTerm{
										TopologyKey: "kubernetes.io/hostname",
										LabelSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"app.kubernetes.io/component": "thanos-receive-router",
											},
										},
									},
								},
							},
						},
					},
				},
				Replicas:          clusters.TemplateFn(clusters.ReceiveRouter, templates.Replicas),
				ReplicationFactor: 3,
				ExternalLabels: map[string]string{
					"receive": "true",
				},
				Additional: v1alpha1.Additional{
					Args: []string{
						fmt.Sprintf("--receive.grpc-service-config=%s", grpcDisableEndlessRetry),
					},
				},
			},
			Ingester: v1alpha1.IngesterSpec{
				DefaultObjectStorageConfig: clusters.TemplateFn(clusters.DefaultBucket, templates.ObjectStorageBucket),
				Additional:                 v1alpha1.Additional{},
				Hashrings:                  hashrings,
			},
		},
	}
}

func defaultCompactCR(namespace string, templates clusters.TemplateMaps, oauth bool) []runtime.Object {
	var objs []runtime.Object
	defaultCompact := &v1alpha1.ThanosCompact{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosCompact",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rhobs",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosCompactSpec{
			CommonFields: v1alpha1.CommonFields{
				Image:                ptr.To(clusters.TemplateFn(clusters.CompactDefault, templates.Images)),
				Version:              ptr.To(clusters.TemplateFn(clusters.CompactDefault, templates.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn(clusters.CompactDefault, templates.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn(clusters.CompactDefault, templates.ResourceRequirements)),
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			},
			ObjectStorageConfig: clusters.TemplateFn(clusters.DefaultBucket, templates.ObjectStorageBucket),
			RetentionConfig: v1alpha1.RetentionResolutionConfig{
				Raw:         v1alpha1.Duration("365d"),
				FiveMinutes: v1alpha1.Duration("365d"),
				OneHour:     v1alpha1.Duration("365d"),
			},
			DownsamplingConfig: &v1alpha1.DownsamplingConfig{
				Concurrency: ptr.To(int32(1)),
				Disable:     ptr.To(false),
			},
			CompactConfig: &v1alpha1.CompactConfig{
				CompactConcurrency: ptr.To(int32(1)),
			},
			DebugConfig: &v1alpha1.DebugConfig{
				AcceptMalformedIndex: ptr.To(true),
				HaltOnError:          ptr.To(true),
				MaxCompactionLevel:   ptr.To(int32(3)),
			},
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn(clusters.CompactDefault, templates.StorageSize),
			},
			Additional: v1alpha1.Additional{
				Args: []string{
					`--deduplication.replica-label=replica`,
				},
			},
		},
	}

	if oauth {
		route := &routev1.Route{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "route.openshift.io/v1",
				Kind:       "Route",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "thanos-compact-rhobs",
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "thanos",
				},
			},
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{
					Kind:   "Service",
					Name:   "thanos-compact-rhobs",
					Weight: ptr.To(int32(100)),
				},
				Port: &routev1.RoutePort{
					TargetPort: intstr.FromString("https"), // Assuming the oauth-proxy is exposing on https port
				},
				TLS: &routev1.TLSConfig{
					Termination:                   routev1.TLSTerminationReencrypt,
					InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
				},
			},
		}
		objs = append(objs, route)
		defaultCompact.Annotations = map[string]string{
			"service.beta.openshift.io/serving-cert-secret-name":               "compact-tls",
			"serviceaccounts.openshift.io/oauth-redirectreference.application": `{"kind":"OAuthRedirectReference","apiVersion":"v1","reference":{"kind":"Route","name":"thanos-compact-rhobs"}}`,
		}
		defaultCompact.Spec.ServicePorts = append(defaultCompact.Spec.ServicePorts, corev1.ServicePort{
			Name: "https",
			Port: 8443,
			TargetPort: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: 8443,
			},
		})
		defaultCompact.Spec.Containers = append(defaultCompact.Spec.Containers, makeOauthProxyContainer(10902, namespace, "thanos-compact-rhobs", "compact-tls"))
		defaultCompact.Spec.Volumes = append(defaultCompact.Spec.Volumes, kghelpers.NewPodVolumeFromSecret("tls", "compact-tls"))
		defaultCompact.Spec.Volumes = append(defaultCompact.Spec.Volumes, kghelpers.NewPodVolumeFromSecret("oauth-cookie", "oauth-cookie"))
	}

	objs = append(objs, defaultCompact)
	return objs
}

func defaultRulerCR(namespace string, templates clusters.TemplateMaps) runtime.Object {
	return &v1alpha1.ThanosRuler{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.thanos.io/v1alpha1",
			Kind:       "ThanosRuler",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rhobs",
			Namespace: namespace,
		},
		Spec: v1alpha1.ThanosRulerSpec{
			CommonFields: v1alpha1.CommonFields{
				Image:                ptr.To(clusters.TemplateFn(clusters.Ruler, templates.Images)),
				Version:              ptr.To(clusters.TemplateFn(clusters.Ruler, templates.Versions)),
				ImagePullPolicy:      ptr.To(corev1.PullIfNotPresent),
				LogLevel:             ptr.To(clusters.TemplateFn(clusters.Ruler, templates.LogLevels)),
				LogFormat:            ptr.To("logfmt"),
				ResourceRequirements: ptr.To(clusters.TemplateFn(clusters.Ruler, templates.ResourceRequirements)),
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			},
			Replicas: clusters.TemplateFn(clusters.Ruler, templates.Replicas),
			RuleConfigSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"operator.thanos.io/prometheus-rule": "true",
				},
			},
			QueryLabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"operator.thanos.io/query-api": "true",
					"app.kubernetes.io/part-of":    "thanos",
				},
			},
			RuleTenancyConfig: &v1alpha1.RuleTenancyConfig{
				EnforcedTenantIdentifier: ptr.To("tenant_id"),
				TenantSpecifierLabel:     ptr.To("operator.thanos.io/tenant"),
			},
			ExternalLabels: map[string]string{
				"rule_replica": "$(NAME)",
			},
			ObjectStorageConfig: clusters.TemplateFn(clusters.DefaultBucket, templates.ObjectStorageBucket),
			AlertmanagerURL:     "dnssrv+http://alertmanager-cluster." + namespace + ".svc.cluster.local:9093",
			AlertLabelDrop:      []string{"rule_replica"},
			Retention:           v1alpha1.Duration("48h"),
			EvaluationInterval:  v1alpha1.Duration("1m"),
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: clusters.TemplateFn(clusters.Ruler, templates.StorageSize),
			},
			Additional: v1alpha1.Additional{},
		},
	}
}

// generateMetricsBundle generates individual metrics bundle resources for Thanos components
// Ordering: CRDs, operator, cache, then Thanos components
func generateMetricsBundle(config clusters.ClusterConfig) error {
	ns := config.Namespace

	// Create bundle generator for individual resource files
	bundleGen := &mimic.Generator{}
	bundleGen = bundleGen.With(templatePath, templateClustersPath, string(config.Environment), string(config.Name), "metrics", "bundle")
	bundleGen.Logger = kitlog.NewLogfmtLogger(kitlog.NewSyncWriter(os.Stdout))

	// 1. CRDs (prefix: 01-*)
	crdObjs := getCRDObjects()
	crdNames := []string{"compacts", "queries", "receives", "rulers", "stores"}
	for i, crd := range crdObjs {
		crdName := "unknown"
		if i < len(crdNames) {
			crdName = crdNames[i]
		}
		filename := fmt.Sprintf("01-crd-%s.yaml", crdName)
		bundleGen.Add(filename, encoding.GhodssYAML(crd))
	}

	// 2. OPERATOR (prefix: 02-*)
	operatorObjs, err := operatorResources(ns, config.Templates)
	if err != nil {
		return fmt.Errorf("failed to generate operator resources: %w", err)
	}
	for _, obj := range operatorObjs {
		resourceKind := getResourceKind(obj)
		resourceName := getSmartResourceName(obj)
		filename := fmt.Sprintf("02-operator-%s-%s.yaml", resourceName, resourceKind)
		bundleGen.Add(filename, encoding.GhodssYAML(obj))
	}

	// 3. CACHE (prefix: 03-*)
	cacheObjs := getThanosCacheObjects(ns, config.Templates)
	for _, obj := range cacheObjs {
		cacheKind := getResourceKind(obj)
		cacheName := getResourceName(obj)
		// Remove thanos- prefix and simplify cache names
		cleanCacheName := strings.TrimPrefix(cacheName, "thanos-")
		filename := fmt.Sprintf("03-cache-%s-%s.yaml", cleanCacheName, cacheKind)
		bundleGen.Add(filename, encoding.GhodssYAML(obj))
	}

	// 4. CUSTOM RESOURCES (prefix: 04-*)
	thanosObjs := make([]runtime.Object, 0, 7) // Pre-allocate for expected ~7 resources (query+route, receive, compact+route, ruler, store)
	thanosObjs = append(thanosObjs, defaultQueryCR(ns, config.Templates, true)...)
	thanosObjs = append(thanosObjs, defaultReceiveCR(ns, config.Templates))
	thanosObjs = append(thanosObjs, defaultCompactCR(ns, config.Templates, true)...)
	thanosObjs = append(thanosObjs, defaultRulerCR(ns, config.Templates))
	thanosObjs = append(thanosObjs, defaultStoreCR(ns, config.Templates))

	for i, obj := range thanosObjs {
		resourceKind := getResourceKind(obj)
		resourceName := getResourceName(obj)
		// Clean up names and remove redundant prefixes
		cleanName := strings.TrimPrefix(resourceName, "thanos-")
		cleanName = strings.TrimPrefix(cleanName, "rhobs-")
		if cleanName == "Unknown" || cleanName == resourceKind || cleanName == "" {
			filename := fmt.Sprintf("04-%s-%d.yaml", strings.ToLower(resourceKind), i+1)
			bundleGen.Add(filename, encoding.GhodssYAML(obj))
		} else {
			filename := fmt.Sprintf("04-%s-%s.yaml", cleanName, resourceKind)
			bundleGen.Add(filename, encoding.GhodssYAML(obj))
		}
	}

	// Generate the bundle files
	bundleGen.Generate()

	// Add consolidated ServiceMonitors to monitoring bundle
	monBundle := GetMonitoringBundle(config)
	thanosServiceMonitors := createConsolidatedThanosServiceMonitors(ns)
	operatorServiceMonitors := thanosOperatorServiceMonitor(ns)

	for _, sm := range thanosServiceMonitors {
		if smObj, ok := sm.(*monitoringv1.ServiceMonitor); ok && smObj != nil {
			monBundle.AddServiceMonitor(smObj)
		}
	}
	for _, sm := range operatorServiceMonitors {
		if smObj, ok := sm.(*monitoringv1.ServiceMonitor); ok && smObj != nil {
			monBundle.AddServiceMonitor(smObj)
		}
	}

	// Add cache ServiceMonitors to monitoring bundle
	cacheServiceMonitors := createThanosCacheServiceMonitors(config)
	for _, sm := range cacheServiceMonitors {
		monBundle.AddServiceMonitor(sm)
	}

	// Generate the individual ServiceMonitor files
	if err := monBundle.Generate(); err != nil {
		return fmt.Errorf("failed to generate monitoring bundle: %w", err)
	}

	return nil
}

// getCRDObjects retrieves Thanos operator CRDs
func getCRDObjects() []runtime.Object {
	const (
		compact   = "thanoscompacts.yaml"
		queries   = "thanosqueries.yaml"
		receivers = "thanosreceives.yaml"
		rulers    = "thanosrulers.yaml"
		stores    = "thanosstores.yaml"
		base      = "https://raw.githubusercontent.com/thanos-community/thanos-operator/" + thanosOperatorCRDRef + "/config/crd/bases/monitoring.thanos.io_"
	)

	var objs []runtime.Object
	for _, component := range []string{compact, queries, receivers, rulers, stores} {
		crd, err := getCustomResourceDefinition(base + component)
		if err != nil {
			log.Printf("Error fetching CRD %s: %v", component, err)
			continue
		}
		objs = append(objs, crd)
	}
	return objs
}

// getThanosCacheObjects returns cache objects for Thanos components
func getThanosCacheObjects(namespace string, templates clusters.TemplateMaps) []runtime.Object {
	var objs []runtime.Object

	// Index cache
	indexCacheConfig := indexCache(templates, namespace)
	objs = append(objs, memcachedStatefulSet(indexCacheConfig, templates))
	objs = append(objs, createServiceAccount(indexCacheConfig.Name, indexCacheConfig.Namespace, indexCacheConfig.Labels))
	objs = append(objs, createCacheHeadlessService(indexCacheConfig))

	// Bucket cache
	bucketCacheConfig := bucketCache(templates, namespace)
	objs = append(objs, memcachedStatefulSet(bucketCacheConfig, templates))
	objs = append(objs, createServiceAccount(bucketCacheConfig.Name, bucketCacheConfig.Namespace, bucketCacheConfig.Labels))
	objs = append(objs, createCacheHeadlessService(bucketCacheConfig))

	// Query range cache
	queryRangeCacheConfig := queryRangeCache(templates, namespace)
	objs = append(objs, memcachedStatefulSet(queryRangeCacheConfig, templates))
	objs = append(objs, createServiceAccount(queryRangeCacheConfig.Name, queryRangeCacheConfig.Namespace, queryRangeCacheConfig.Labels))
	objs = append(objs, createCacheHeadlessService(queryRangeCacheConfig))

	// Cache secrets
	cacheSecrets := memcachedCacheSecrets(namespace)
	for _, secret := range cacheSecrets {
		objs = append(objs, secret)
	}

	return objs
}

// createThanosCacheServiceMonitors creates ServiceMonitors for Thanos cache components
func createThanosCacheServiceMonitors(config clusters.ClusterConfig) []*monitoringv1.ServiceMonitor {
	ns := config.Namespace
	templates := config.Templates

	var serviceMonitors []*monitoringv1.ServiceMonitor

	// Create cache configurations
	cacheConfigs := []*memcachedConfig{
		indexCache(templates, ns),
		bucketCache(templates, ns),
		queryRangeCache(templates, ns),
	}

	// Generate ServiceMonitor for each cache
	for _, cacheConfig := range cacheConfigs {
		sm := createCacheServiceMonitor(cacheConfig)
		serviceMonitors = append(serviceMonitors, sm)
	}

	return serviceMonitors
}

// getResourceName extracts a meaningful name from a Kubernetes object
func getResourceName(obj runtime.Object) string {
	if obj == nil {
		return "unknown"
	}

	switch o := obj.(type) {
	case metav1.Object:
		name := o.GetName()
		if name != "" {
			return name
		}
	}

	// Fallback to the object type
	return "unnamed"
}

// getSmartResourceName generates meaningful names for operator resources
func getSmartResourceName(obj runtime.Object) string {
	if obj == nil {
		return "unknown"
	}

	// Get the basic name first
	basicName := getResourceName(obj)
	if basicName == "unnamed" || basicName == "unknown" {
		return basicName
	}

	// For operator resources that have thanos-operator prefix, remove it since it's implied
	basicName = strings.TrimPrefix(basicName, "thanos-operator-")

	return basicName
}
