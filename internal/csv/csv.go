package csv

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// NewClusterServiceVersion returns a new *ClusterServiceVersion with the YAML in
// outputDir if it exists, otherwise it will return an empty *ClusterServiceVersion.
func NewClusterServiceVersion(outputDir string) (*v1alpha1.ClusterServiceVersion, error) {
	base := &v1alpha1.ClusterServiceVersion{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1alpha1",
			Kind:       "ClusterServiceVersion",
		},
	}
	baseCSVPath := filepath.Join(outputDir, "clusterserviceversion.yaml.tmpl")
	_, err := os.Stat(baseCSVPath)
	if err != nil && os.IsNotExist(err) {
		return base, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "cannot stat clusterserviceversion.yaml.tmpl file")
	}
	d, err := ioutil.ReadFile(baseCSVPath)
	if err != nil {
		return nil, errors.Wrap(err, "cannot read base ClusterServiceVersion file")
	}
	if err := yaml.Unmarshal(d, base); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal given base ClusterServiceVersion file")
	}
	return base, nil
}

// NewAnnotations returns a new annotation object. If outputDir contains a template,
// it will be read and added.
func NewAnnotations(outputDir string) (map[string]string, error) {
	baseAnnPath := filepath.Join(outputDir, "annotations.yaml.tmpl")
	_, err := os.Stat(baseAnnPath)
	if err != nil && os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "cannot stat annotations.yaml.tmpl file")
	}
	d, err := ioutil.ReadFile(baseAnnPath)
	if err != nil {
		return nil, errors.Wrap(err, "cannot read base annotations file")
	}
	// This type is imposed by the format.
	base := map[string]map[string]string{}
	if err := yaml.Unmarshal(d, &base); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal given base annotations file")
	}
	if base["annotations"] == nil {
		return nil, errors.Errorf("existing annotations template in %s is empty", baseAnnPath)
	}
	return base["annotations"], nil
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
	if err := Validate(left); err != nil {
		return nil, errors.Wrap(err, "cannot validate resources")
	}
	return left, nil
}
