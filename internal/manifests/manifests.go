package manifests

import (
	"io"
	"os"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func NewParser(files []string, streams ...io.Reader) *Parser {
	return &Parser{
		Streams: streams,
		Files:   files,
	}
}

type Parser struct {
	Streams []io.Reader
	Files   []string
}

func (p *Parser) Parse() ([]*unstructured.Unstructured, error) {
	var result []*unstructured.Unstructured
	for _, s := range p.Streams {
		u, err := parseStream(s)
		if err != nil {
			return nil, errors.Wrap(err, "cannot parse stream")
		}
		result = append(result, u...)
	}
	for _, p := range p.Files {
		f, err := os.Open(p)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot open file in %s", p)
		}
		u, err := parseStream(f)
		if err != nil {
			return nil, errors.Wrap(err, "cannot parse file stream")
		}
		result = append(result, u...)
	}
	return result, nil
}

func parseStream(in io.Reader) ([]*unstructured.Unstructured, error) {
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
		result = append(result, u)
	}
	return result, nil
}
