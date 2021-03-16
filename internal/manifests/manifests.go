package manifests

import (
	"io"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func Parse(in io.Reader) ([]*unstructured.Unstructured, error) {
	dec := yaml.NewYAMLOrJSONDecoder(in, 1024)
	var result []*unstructured.Unstructured
	for {
		u := &unstructured.Unstructured{}
		err := dec.Decode(u)
		if err != nil && err != io.EOF {
			return nil, errors.Wrap(err, "cannot decode yaml into unstructured")
		}
		if err == io.EOF {
			break
		}
		// Helm does not have any built-in validation like Kustomize, so, we
		// have to do some basic sanity check to skip empty templates.
		if u.GetName() == "" || u.GetAPIVersion() == "" || u.GetKind() == "" {
			continue
		}
		// delete(u.GetAnnotations(), "release")
		result = append(result, u)
	}
	return result, nil
}
