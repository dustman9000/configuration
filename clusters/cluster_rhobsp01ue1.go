package clusters

import (
	"github.com/observatorium/api/rbac"
	observatoriumapi "github.com/observatorium/observatorium/configuration_go/abstr/kubernetes/observatorium/api"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	ClusterRHOBSUSEastOneProduction ClusterName = "rhobsp01ue1"
)

func init() {
	RegisterCluster(ClusterConfig{
		Name:        ClusterRHOBSUSEastOneProduction,
		Environment: EnvironmentProduction,
		Namespace:   "rhobs-production",
		GatewayConfig: NewGatewayConfig(
			WithMetricsEnabled(),
			WithLoggingEnabled(),
			WithSyntheticsEnabled(),
			WithInternalTracingSDKEnabled(),
			WithTenants(rhobsp01ue1Tenants()),
			WithRBAC(rhobsp01ue1RBAC()),
			WithCustomRoute("us-east-1-0.rhobs.api.openshift.com"),
		),
		Templates:  rhobsp01ue1TemplateMaps(),
		BuildSteps: rhobsp01ue1BuildSteps(),
	})
}

func rhobsp01ue1Tenants() observatoriumapi.Tenants {
	return observatoriumapi.Tenants{
		Tenants: []observatoriumapi.Tenant{
			{
				Name: "hcp",
				ID:   "EFD08939-FE1D-41A1-A28A-BE9A9BC68003",
				OIDC: &observatoriumapi.TenantOIDC{
					ClientID:      "${CLIENT_ID}",
					ClientSecret:  "${CLIENT_SECRET}",
					IssuerURL:     "https://sso.redhat.com/auth/realms/redhat-external",
					RedirectURL:   "https://observatorium-mst.api.stage.openshift.com/oidc/odfms/callback",
					UsernameClaim: "client_id",
				},
			},
		},
	}
}

func rhobsp01ue1RBAC() ObservatoriumRBAC {
	opts := &BindingOpts{}
	opts.WithServiceAccountName("cd54dce2-590e-4ea4-9b83-a83c58205962").
		WithTenant(HcpTenant).
		WithSignals([]Resource{MetricsResource, LogsResource, ProbesResource}).
		WithPerms([]rbac.Permission{rbac.Read, rbac.Write})

	config := GenerateClusterRBAC(opts)
	return *config
}

func rhobsp01ue1BuildSteps() []string {
	return []string{
		StepGateway,
		StepDefaultThanosStack,
		StepDefaultLokiStack,
		StepSyntheticsApi,
		StepAlertmanager,
	}
}

// rhobsp01ue1TemplateMaps returns template mappings specific to the rhobsp01ue1 production cluster
func rhobsp01ue1TemplateMaps() TemplateMaps {
	lokiOverrides := LokiOverridesMap{
		LokiConfig: LokiOverrides{
			LokiLimitOverrides: LokiLimitOverrides{
				IngestionRateLimitMB: 320,
				PerStreamRateLimitMB: 240,
				PerStreamBurstSizeMB: 480,
				QueryTimeout:         "5m",
			},
			Ingest: LokiComponentSpec{
				Replicas: 18,
			},
			Router: LokiComponentSpec{
				Replicas: 9,
			},
		},
	}

	overrideWith := []TemplateOverride{
		Replicas{
			// TODO: @moadz temporary scale out to deal with stampeding herd of retries
			ReceiveIngestorDefault: 6,
		},
		Resources{
			ReceiveIngestorDefault: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("40Gi"),
				},
			},
		},
		StorageSizes{
			CompactDefault: "80Gi",
		},
		lokiOverrides,
	}

	return DefaultBaseTemplate().Override(overrideWith...)
}
