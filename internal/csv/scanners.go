package csv

import (
	"strings"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
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
type Deployment struct{}

// Run adds the spec of Deployment manifests to ClusterServiceVersion. If successful,
// their manifests should not be included in the bundle separately.
func (*Deployment) Run(manifest *unstructured.Unstructured, csv *v1alpha1.ClusterServiceVersion) (bool, error) {
	if !strings.EqualFold(manifest.GetObjectKind().GroupVersionKind().Kind, "Deployment") {
		return false, nil
	}
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
	return true, nil
}
