package clusters

import (
	"fmt"

	"github.com/bwplotka/mimic"
	"github.com/observatorium/api/rbac"
)

type TenantID string

const (
	// HcpTenant is the name of the HCP tenant.
	HcpTenant TenantID = "hcp"
)

type Resource string

const (
	MetricsResource Resource = "metrics"
	LogsResource    Resource = "logs"
	ProbesResource  Resource = "probes"
	TracesResource  Resource = "traces"
)

// GenerateClusterRBAC generates rbac.json for the cluster
// RBAC defines roles and role binding for each tenant and matching subject names that will be validated
// against 'user' field in the incoming JWT token that contains service account.
func GenerateClusterRBAC(opts ...*BindingOpts) *ObservatoriumRBAC {
	obsRBAC := ObservatoriumRBAC{
		mappedRoleNames: map[RoleMapKey]string{},
	}

	for _, o := range opts {
		attachBinding(&obsRBAC, *o)
	}

	// Use JSON because we want to have jsonnet using that in configmaps/secrets.
	return &obsRBAC
}

type RoleMapKey struct {
	tenant TenantID
	signal Resource
	perm   rbac.Permission
}

// ObservatoriumRBAC represents the structure that is sued to parse RBAC configuration
// in Observatorium API: https://github.com/observatorium/api/blob/078b7ce75837bb03984f5ed99d2b69a512b696b5/rbac/rbac.go#L181.
type ObservatoriumRBAC struct {
	// mappedRoleNames is used for deduplication logic.
	mappedRoleNames map[RoleMapKey]string

	Roles        []rbac.Role        `json:"roles"`
	RoleBindings []rbac.RoleBinding `json:"roleBindings"`
}

type BindingOpts struct {
	// NOTE(bwplotka): Name is strongly correlated to subject name that corresponds to the service account username (it has to match it)/
	// Any change, require changes on tenant side, so be careful.
	name    string
	tenant  TenantID
	signals []Resource
	perms   []rbac.Permission
}

func (bo *BindingOpts) WithServiceAccountName(n string) *BindingOpts {
	bo.name = n
	return bo
}

func (bo *BindingOpts) WithTenant(t TenantID) *BindingOpts {
	bo.tenant = t
	return bo
}

func (bo *BindingOpts) WithSignals(signals []Resource) *BindingOpts {
	bo.signals = signals
	return bo
}

func (bo *BindingOpts) WithPerms(perms []rbac.Permission) *BindingOpts {
	bo.perms = perms
	return bo
}

func getOrCreateRoleName(o *ObservatoriumRBAC, tenant TenantID, s Resource, p rbac.Permission) string {
	k := RoleMapKey{tenant: tenant, signal: s, perm: p}

	n, ok := o.mappedRoleNames[k]
	if !ok {
		n = fmt.Sprintf("%s-%s-%s", k.tenant, k.signal, k.perm)
		o.Roles = append(o.Roles, rbac.Role{
			Name:        n,
			Permissions: []rbac.Permission{k.perm},
			Resources:   []string{string(k.signal)},
			Tenants:     []string{string(k.tenant)},
		})
		o.mappedRoleNames[k] = n
	}
	return n
}

func attachBinding(o *ObservatoriumRBAC, opts BindingOpts) {
	for _, b := range o.RoleBindings {
		if b.Name == opts.name {
			mimic.Panicf("found duplicate binding name", opts.name)

		}
	}

	// Is there role that satisfy this already? If not, create.
	var roles []string
	for _, s := range opts.signals {
		for _, p := range opts.perms {
			roles = append(roles, getOrCreateRoleName(o, opts.tenant, s, p))
		}
	}

	o.RoleBindings = append(o.RoleBindings, rbac.RoleBinding{
		Name:  opts.name,
		Roles: roles,
		Subjects: []rbac.Subject{
			{Name: opts.name, Kind: rbac.User},
		},
	})
}
