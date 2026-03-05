package clusters

import (
	"github.com/observatorium/api/rbac"
	observatoriumapi "github.com/observatorium/observatorium/configuration_go/abstr/kubernetes/observatorium/api"
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
	return DefaultBaseTemplate().Override(
		Replicas{
			// TODO: @moadz temporary scale out to deal with stampeding herd of retries
			ReceiveIngestorDefault: 6,
		},
	)
}
