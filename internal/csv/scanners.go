package csv

import (
	"strings"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"

	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// OLM does not support aggregation for ClusterRoles, meaning we cannot embed a
// ClusterRoles whose rules are aggregated because CSV accepts only a list of rules.
// So we skip it and write ClusterRoles directly to the disk.
// See https://github.com/operator-framework/operator-lifecycle-manager/issues/2039

// TODO(muvaf): In order to embed Roles, we need to create a map of bindings
// because OLM accepts the list of permissions for the deployment you'd like to
// bind it to. We skip doing this for now.

const (
	installStrategyName = "deployment"
)

// CustomResourceDefinition adds metadata of given CustomResourceDefinitions
// to ClusterServiceVersion as owned CRD.
type CustomResourceDefinition struct{}

// Run adds CRD ownership information to ClusterServiceVersion if given manifest
// is a CRD.
func (*CustomResourceDefinition) Run(manifest *unstructured.Unstructured, csv *v1alpha1.ClusterServiceVersion) (bool, error) {
	if !strings.EqualFold(manifest.GetObjectKind().GroupVersionKind().Kind, "CustomResourceDefinition") {
		return false, nil
	}
	crd := &v1.CustomResourceDefinition{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Object, crd); err != nil {
		return false, err
	}
	var v string
	for _, ver := range crd.Spec.Versions {
		if ver.Served {
			v = ver.Name
			break
		}
	}
	owned := v1alpha1.CRDDescription{
		Name:        crd.Name,
		Version:     v,
		Kind:        crd.Spec.Names.Kind,
		DisplayName: crd.Spec.Names.Kind,
	}
	csv.Spec.CustomResourceDefinitions.Owned = append(csv.Spec.CustomResourceDefinitions.Owned, owned)
	return false, nil
}

// Deployment scans Deployments to add their spec to ClusterServiceVersion.
type Deployment struct {
	permSet        []v1alpha1.StrategyDeploymentPermissions
	permClusterSet []v1alpha1.StrategyDeploymentPermissions
	rules          map[string][]rbacv1.PolicyRule
}

// populateRules populates ClusterRole and Role for further processing
func (d *Deployment) populateRules(manifest *unstructured.Unstructured) error {
	r := &rbacv1.Role{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Object, r); err != nil {
		return err
	}
	if len(d.rules) == 0 {
		d.rules = make(map[string][]rbacv1.PolicyRule)
	}
	d.rules[r.Name] = r.Rules
	return nil
}

// appendRules appens StrategyDeploymentPermissions.Rules under the same ServiceAccountName
func appendRules(existing []v1alpha1.StrategyDeploymentPermissions, add v1alpha1.StrategyDeploymentPermissions) []v1alpha1.StrategyDeploymentPermissions {
	saExists := false
	for i := range existing {
		if existing[i].ServiceAccountName == add.ServiceAccountName {
			existing[i].Rules = append(existing[i].Rules, add.Rules...)
			saExists = true
			break
		}
	}
	if !saExists {
		existing = append(existing, add)
	}
	return existing
}

// formatPermissions generates StrategyDeploymentPermissions
func (d *Deployment) formatPermissions(manifest *unstructured.Unstructured, cluster bool) error {
	perm := v1alpha1.StrategyDeploymentPermissions{}
	rb := &rbacv1.RoleBinding{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Object, rb); err != nil {
		return err
	}

	for _, sub := range rb.Subjects {
		if sub.Kind == "ServiceAccount" {
			perm.ServiceAccountName = sub.Name
			perm.Rules = d.rules[rb.RoleRef.Name]
			if cluster {
				d.permClusterSet = appendRules(d.permClusterSet, perm)
			}
			if !cluster {
				d.permSet = appendRules(d.permSet, perm)
			}
		}
	}
	return nil
}

// Run adds the spec of Deployment manifests to ClusterServiceVersion. If successful,
// their manifests should not be included in the bundle separately.
func (d *Deployment) Run(manifest *unstructured.Unstructured, csv *v1alpha1.ClusterServiceVersion) (bool, error) {
	switch manifest.GetObjectKind().GroupVersionKind().Kind {
	case "Deployment":
		dep := &appsv1.Deployment{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Object, dep); err != nil {
			return false, err
		}
		spec := v1alpha1.StrategyDeploymentSpec{
			Name: dep.Name,
			Spec: dep.Spec,
		}
		csv.Spec.InstallStrategy.StrategyName = installStrategyName
		csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs = append(csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs, spec)
		csv.Spec.InstallStrategy.StrategySpec.ClusterPermissions = d.permClusterSet
		csv.Spec.InstallStrategy.StrategySpec.Permissions = d.permSet
		return true, nil
	case "ClusterRole", "Role":
		err := d.populateRules(manifest)
		if err != nil {
			return false, err
		}
		return true, nil
	case "ClusterRoleBinding":
		err := d.formatPermissions(manifest, true)
		if err != nil {
			return false, err
		}
		return true, nil
	case "RoleBinding":
		err := d.formatPermissions(manifest, false)
		if err != nil {
			return false, err
		}
		return true, nil
	default:
		return false, nil
	}
}
