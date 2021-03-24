package csv

import (
	"github.com/operator-framework/operator-registry/pkg/lib/bundle"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Validate checks whether objects can be included in an OLM bundle.
func Validate(resources []*unstructured.Unstructured) error {
	for _, r := range resources {
		s, _ := bundle.IsSupported(r.GroupVersionKind().Kind)
		if !s {
			return errors.Errorf("kind %s is not supported by OLM Bundle specification", r.GroupVersionKind().Kind)
		}
	}
	return nil
}
