package clusters

import (
	"github.com/observatorium/api/rbac"
	observatoriumapi "github.com/observatorium/observatorium/configuration_go/abstr/kubernetes/observatorium/api"
)

const (
	ClusterRHOBSUSEastOneShardOneProduction ClusterName = "rhobsp02ue1"
)

func init() {
	RegisterCluster(ClusterConfig{
		Name:        ClusterRHOBSUSEastOneShardOneProduction,
		Environment: EnvironmentProduction,
		Namespace:   "rhobs-production",
		GatewayConfig: NewGatewayConfig(
			WithMetricsEnabled(),
			WithLoggingEnabled(),
			WithSyntheticsEnabled(),
			WithTenants(rhobsp02ue1Tenants()),
			WithRBAC(rhobsp02ue1RBAC()),
			WithCustomRoute("us-east-1-1.rhobs.api.openshift.com"),
		),
		Templates:  rhobsp02ue1TemplateMaps(),
		BuildSteps: rhobsp02ue1BuildSteps(),
	})
}

func rhobsp02ue1Tenants() observatoriumapi.Tenants {
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

func rhobsp02ue1RBAC() ObservatoriumRBAC {
	opts := &BindingOpts{}
	opts.WithServiceAccountName("cd54dce2-590e-4ea4-9b83-a83c58205962").
		WithTenant(HcpTenant).
		WithSignals([]Resource{MetricsResource, LogsResource, ProbesResource}).
		WithPerms([]rbac.Permission{rbac.Read, rbac.Write})

	config := GenerateClusterRBAC(opts)
	return *config
}

func rhobsp02ue1BuildSteps() []string {
	return []string{
		StepGateway,
		StepDefaultThanosStack,
		StepDefaultLokiStack,
		StepSyntheticsApi,
		StepAlertmanager,
	}
}

// rhobsp02ue1TemplateMaps returns template mappings specific to the ClusterRHOBSUSEastOneShardOneProduction cluster
func rhobsp02ue1TemplateMaps() TemplateMaps {

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

	return DefaultBaseTemplate().Override(lokiOverrides)
}
