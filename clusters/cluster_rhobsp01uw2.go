package clusters

import (
	"github.com/observatorium/api/rbac"
	observatoriumapi "github.com/observatorium/observatorium/configuration_go/abstr/kubernetes/observatorium/api"
)

const (
	ClusterRHOBSUSWestTwoProduction ClusterName = "rhobsp01uw2"
)

func init() {
	RegisterCluster(ClusterConfig{
		Name:        ClusterRHOBSUSWestTwoProduction,
		Environment: EnvironmentProduction,
		Namespace:   "rhobs-production",
		GatewayConfig: NewGatewayConfig(
			WithMetricsEnabled(),
			WithLoggingEnabled(),
			WithSyntheticsEnabled(),
			WithInternalTracingSDKEnabled(),
			WithTenants(rhobsp01uw2Tenants()),
			WithRBAC(rhobsp01uw2RBAC()),
			WithCustomRoute("us-west-2-0.rhobs.api.openshift.com"),
		),
		Templates:  rhobsp01uw2TemplateMaps(),
		BuildSteps: rhobsp01uw2BuildSteps(),
	})
}

func rhobsp01uw2Tenants() observatoriumapi.Tenants {
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

func rhobsp01uw2RBAC() ObservatoriumRBAC {
	opts := &BindingOpts{}
	opts.WithServiceAccountName("cd54dce2-590e-4ea4-9b83-a83c58205962").
		WithTenant(HcpTenant).
		WithSignals([]Resource{MetricsResource, LogsResource, ProbesResource}).
		WithPerms([]rbac.Permission{rbac.Read, rbac.Write})

	config := GenerateClusterRBAC(opts)
	return *config
}

func rhobsp01uw2BuildSteps() []string {
	return []string{
		StepGateway,
		StepDefaultThanosStack,
		StepDefaultLokiStack,
		StepSyntheticsApi,
		StepAlertmanager,
	}
}

// rhobsp01uw2TemplateMaps returns template mappings specific to the rhobsp01uw2 production cluster
func rhobsp01uw2TemplateMaps() TemplateMaps {

	lokiOverrides := LokiOverridesMap{
		LokiConfig: LokiOverrides{
			LokiLimitOverrides: LokiLimitOverrides{
				IngestionRateLimitMB: 20,
				PerStreamRateLimitMB: 15,
				PerStreamBurstSizeMB: 30,
				QueryTimeout:         "5m",
			},
			Ingest: LokiComponentSpec{
				Replicas: 3,
			},
		},
	}

	return DefaultBaseTemplate().Override(lokiOverrides)
}
