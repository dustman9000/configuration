package clusters

import (
	"github.com/observatorium/api/rbac"
	observatoriumapi "github.com/observatorium/observatorium/configuration_go/abstr/kubernetes/observatorium/api"
)

const (
	ClusterRHOBSAsiaPacificSouthEastTwoProduction ClusterName = "rhobsp01apse2"
)

func init() {
	RegisterCluster(ClusterConfig{
		Name:        ClusterRHOBSAsiaPacificSouthEastTwoProduction,
		Environment: EnvironmentProduction,
		Namespace:   "rhobs-production",
		GatewayConfig: NewGatewayConfig(
			WithMetricsEnabled(),
			WithLoggingEnabled(),
			WithSyntheticsEnabled(),
			WithInternalTracingSDKEnabled(),
			WithTenants(rhobsp01apse2Tenants()),
			WithRBAC(rhobsp01apse2RBAC()),
			WithCustomRoute("ap-southeast-2-0.rhobs.api.openshift.com"),
		),
		Templates:  rhobsp01apse2TemplateMaps(),
		BuildSteps: rhobsp01apse2BuildSteps(),
	})
}

func rhobsp01apse2Tenants() observatoriumapi.Tenants {
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

func rhobsp01apse2RBAC() ObservatoriumRBAC {
	opts := &BindingOpts{}
	opts.WithServiceAccountName("cd54dce2-590e-4ea4-9b83-a83c58205962").
		WithTenant(HcpTenant).
		WithSignals([]Resource{MetricsResource, LogsResource, ProbesResource}).
		WithPerms([]rbac.Permission{rbac.Read, rbac.Write})

	config := GenerateClusterRBAC(opts)
	return *config
}

func rhobsp01apse2BuildSteps() []string {
	return []string{
		StepGateway,
		StepDefaultThanosStack,
		StepDefaultLokiStack,
		StepSyntheticsApi,
		StepAlertmanager,
	}
}

// rhobsp01apse2TemplateMaps returns template mappings specific to the rhobsp01apse2 production cluster
func rhobsp01apse2TemplateMaps() TemplateMaps {

	lokiOverrides := LokiOverridesMap{
		LokiConfig: LokiOverrides{
			LokiLimitOverrides: LokiLimitOverrides{
				IngestionRateLimitMB: 40,
				PerStreamRateLimitMB: 30,
				PerStreamBurstSizeMB: 60,
				QueryTimeout:         "5m",
			},
			Ingest: LokiComponentSpec{
				Replicas: 6,
			},
		},
	}

	return DefaultBaseTemplate().Override(lokiOverrides)
}
