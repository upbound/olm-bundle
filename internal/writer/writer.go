package writer

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Bundle represents the final state that will be written to disk.
type Bundle struct {
	PackageDir string
	Version    string
	Manifests  []client.Object
	Metadata   Metadata
}

// Metadata contains the content of /metadata folder in the bundle.
type Metadata struct {
	Annotations map[string]string
	// dependencies.yaml is not supported.
}

// Write writes the bundle files to disk.
func (b *Bundle) Write() (string, error) {
	versionDir := filepath.Join(b.PackageDir, b.Version)
	if err := os.MkdirAll(versionDir, os.ModePerm); err != nil {
		return "", errors.Wrapf(err, "cannot create folder %s", versionDir)
	}

	dfPath := filepath.Join(versionDir, "Dockerfile")
	if err := b.writeDockerfile(dfPath); err != nil {
		return "", errors.Wrap(err, "cannot write bundle.Dockerfile ")
	}

	manifestsDir := filepath.Join(versionDir, "manifests")
	if err := b.writeManifests(manifestsDir); err != nil {
		return "", errors.Wrap(err, "cannot write manifests")
	}

	metadataDir := filepath.Join(versionDir, "metadata")
	if err := b.writeAnnotations(metadataDir); err != nil {
		return "", errors.Wrap(err, "cannot write annotations")
	}
	return versionDir, nil
}

func (b *Bundle) writeDockerfile(path string) error {
	out := "FROM scratch\n\n"
	for k, v := range b.Metadata.Annotations {
		out += fmt.Sprintf("LABEL %s=%s\n", k, v)
	}
	out += "\nCOPY manifests /manifests/\n"
	out += "COPY metadata /metadata/\n"
	return ioutil.WriteFile(path, []byte(out), os.ModePerm)
}

func (b *Bundle) writeManifests(dir string) error {
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return errors.Wrapf(err, "cannot create folder %s", dir)
	}
	for _, m := range b.Manifests {
		name := fmt.Sprintf("%s.%s.yaml", cleanName(m.GetName()), m.GetObjectKind().GroupVersionKind().Kind)
		o, err := yaml.Marshal(m)
		if err != nil {
			return errors.Wrap(err, "cannot marshal object into YAML")
		}
		if err = ioutil.WriteFile(filepath.Join(dir, name), o, os.ModePerm); err != nil {
			return errors.Wrap(err, "cannot write object YAML file")
		}
	}
	return nil
}

func (b *Bundle) writeAnnotations(dir string) error {
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return errors.Wrapf(err, "cannot create folder %s", dir)
	}
	a := map[string]map[string]string{
		"annotations": b.Metadata.Annotations,
	}
	out, err := yaml.Marshal(a)
	if err != nil {
		return errors.Wrap(err, "cannot marshal annotations map")
	}
	annotationsPath := filepath.Join(dir, "annotations.yaml")
	return ioutil.WriteFile(annotationsPath, out, os.ModePerm)
}

func cleanName(s string) string {
	return strings.ReplaceAll(s, ":", "_")
}
