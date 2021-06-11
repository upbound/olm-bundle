package csv

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// OverrideClusterServiceVersion reads the override file in the output directory
// and makes sure it's applied to the given base ClusterServiceVersion.
func OverrideClusterServiceVersion(base *v1alpha1.ClusterServiceVersion, outputDir string) error {
	baseCSVPath := filepath.Join(outputDir, "clusterserviceversion.yaml.tmpl")
	_, err := os.Stat(baseCSVPath)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "cannot stat clusterserviceversion.yaml.tmpl file")
	}
	d, err := ioutil.ReadFile(baseCSVPath)
	if err != nil {
		return errors.Wrap(err, "cannot read base ClusterServiceVersion file")
	}
	return errors.Wrap(yaml.Unmarshal(d, base), "cannot unmarshal given base ClusterServiceVersion file")
}

// OverrideAnnotations reads the local annotations file and merges it with the
// given annotations map.
func OverrideAnnotations(base map[string]string, outputDir string) error {
	overrideAnnPath := filepath.Join(outputDir, "annotations.yaml.tmpl")
	_, err := os.Stat(overrideAnnPath)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "cannot stat annotations.yaml.tmpl file")
	}
	d, err := ioutil.ReadFile(overrideAnnPath)
	if err != nil {
		return errors.Wrap(err, "cannot read base annotations file")
	}
	ann := map[string]map[string]string{}
	if err := yaml.Unmarshal(d, &ann); err != nil {
		return errors.Wrap(err, "cannot unmarshal given override annotations file")
	}
	for k, v := range ann["annotations"] {
		base[k] = v
	}
	return nil
}

// Scanner is a struct that can take information from a manifest to add it to
// given ClusterServiceVersion and return whether the manifest should be ignored
// in the final package.
type Scanner interface {
	Run(manifest *unstructured.Unstructured, csv *v1alpha1.ClusterServiceVersion) (ignore bool, err error)
}

// NewEmbedder returns a new *Embedder.
func NewEmbedder() *Embedder {
	return &Embedder{
		Scanners: []Scanner{
			&CustomResourceDefinition{},
			&Deployment{},
		},
	}
}

// Embedder runs given Scanners in order.
type Embedder struct {
	Scanners []Scanner
}

// Embed runs all scanners and validates whether the final list of manifests are
// of supported types by OLM.
func (g *Embedder) Embed(manifests []*unstructured.Unstructured, csv *v1alpha1.ClusterServiceVersion) ([]*unstructured.Unstructured, error) {
	var left []*unstructured.Unstructured
	for _, m := range manifests {
		anyIncluded := false
		for _, s := range g.Scanners {
			included, err := s.Run(m, csv)
			if err != nil {
				return nil, err
			}
			if !anyIncluded {
				anyIncluded = included
			}
		}
		if !anyIncluded {
			left = append(left, m)
		}
	}
	return left, nil
}
