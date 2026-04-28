package clusters

import (
	"github.com/observatorium/api/rbac"
	observatoriumapi "github.com/observatorium/observatorium/configuration_go/abstr/kubernetes/observatorium/api"
)

const (
	ClusterRHOBSSouthAmericaEastOneProduction ClusterName = "rhobsp01sae1"
)

func init() {
	RegisterCluster(ClusterConfig{
		Name:        ClusterRHOBSSouthAmericaEastOneProduction,
		Environment: EnvironmentProduction,
		Namespace:   "rhobs-production",
		GatewayConfig: NewGatewayConfig(
			WithMetricsEnabled(),
			WithLoggingEnabled(),
			WithSyntheticsEnabled(),
			WithInternalTracingSDKEnabled(),
			WithTenants(rhobsp01sae1Tenants()),
			WithRBAC(rhobsp01sae1RBAC()),
			WithCustomRoute("sa-east-1-0.rhobs.api.openshift.com"),
		),
		Templates:  rhobsp01sae1TemplateMaps(),
		BuildSteps: rhobsp01sae1BuildSteps(),
	})
}

func rhobsp01sae1Tenants() observatoriumapi.Tenants {
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

func rhobsp01sae1RBAC() ObservatoriumRBAC {
	opts := &BindingOpts{}
	opts.WithServiceAccountName("cd54dce2-590e-4ea4-9b83-a83c58205962").
		WithTenant(HcpTenant).
		WithSignals([]Resource{MetricsResource, LogsResource, ProbesResource}).
		WithPerms([]rbac.Permission{rbac.Read, rbac.Write})

	config := GenerateClusterRBAC(opts)
	return *config
}

func rhobsp01sae1BuildSteps() []string {
	return []string{
		StepGateway,
		StepDefaultThanosStack,
		StepDefaultLokiStack,
		StepSyntheticsApi,
		StepAlertmanager,
	}
}

// rhobsp01sae1TemplateMaps returns template mappings specific to the rhobsp01sae1 production cluster
func rhobsp01sae1TemplateMaps() TemplateMaps {

	lokiOverrides := LokiOverridesMap{
		LokiConfig: LokiOverrides{
			LokiLimitOverrides: LokiLimitOverrides{
				IngestionRateLimitMB:      250,
				MaxGlobalStreamsPerTenant: 30000,
				PerStreamRateLimitMB:      240,
				PerStreamBurstSizeMB:      480,
				QueryTimeout:              "5m",
			},
			Ingest: LokiComponentSpec{
				Replicas: 6,
			},
			Router: LokiComponentSpec{
				Replicas: 6,
			},
		},
	}

	return DefaultBaseTemplate().Override(lokiOverrides)
}
