package csv

import (
	"github.com/ghodss/yaml"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/pkg/errors"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// NewClusterServiceVersion returns a new *ClusterServiceVersion with type information
// since it doesn't come by default when you initialize it.
func NewClusterServiceVersion(baseCSVPath string) (*v1alpha1.ClusterServiceVersion, error) {
	base := &v1alpha1.ClusterServiceVersion{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1alpha1",
			Kind:       "ClusterServiceVersion",
		},
	}
	if baseCSVPath == "" {
		return base, nil
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

type Scanner interface {
	Run(manifest *unstructured.Unstructured, csv *v1alpha1.ClusterServiceVersion) (included bool, err error)
}

func NewEmbedder() *Embedder {
	return &Embedder{
		Scanners: []Scanner{
			&CustomResourceDefinition{},
			&Deployment{},
		},
	}
}

type Embedder struct {
	Scanners []Scanner
}

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
