package main

import (
	"fmt"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/pkg/errors"
	"gopkg.in/alecthomas/kingpin.v2"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"

	"github.com/ghodss/yaml"

	"github.com/upbound/olm-bundle/internal/csv"
	"github.com/upbound/olm-bundle/internal/manifests"
)

var (
	chartFilePath = kingpin.Flag("chart-file-path", "Path to Helm Chart.yaml file to produce metadata").Short('c').String()
	// TODO(muvaf): Support printing to stdout.
	outputDir = kingpin.Flag("output-dir", "Output directory to save the OLM bundle files").Short('o').Required().String()
)

func main() {
	kingpin.Parse()
	resources, err := manifests.Parse(os.Stdin)
	kingpin.FatalIfError(err, "cannot parse resources")
	e := csv.NewEmbedder()
	result := csv.NewClusterServiceVersion()
	if *chartFilePath != "" {
		hm := &manifests.HelmMetadata{
			ChartFilePath: *chartFilePath,
		}
		kingpin.FatalIfError(hm.Embed(result), "cannot embed metadata from Helm Chart.yaml file")
	}
	left, err := e.Embed(resources, result)
	kingpin.FatalIfError(err, "cannot embed resources into ClusterServiceVersion file")
	kingpin.FatalIfError(csv.Validate(left), "cannot validate")
	kingpin.FatalIfError(os.MkdirAll(*outputDir, os.ModePerm), "cannot create output dir")
	kingpin.FatalIfError(write(left, result), "cannot write OLM bundle")
	fmt.Printf("\\U0001f47f Completed!\nYou can find your OLM bundle in %s\n", *outputDir)
}

func write(resources []*unstructured.Unstructured, csvo *v1alpha1.ClusterServiceVersion) error {
	out := make([]client.Object, len(resources)+1)
	for i, u := range resources {
		out[i] = u
	}
	out[len(out)-1] = csvo
	for _, m := range out {
		name := fmt.Sprintf("%s.%s.yaml", cleanup(m.GetName()), m.GetObjectKind().GroupVersionKind().Kind)
		o, err := yaml.Marshal(m)
		if err != nil {
			return errors.Wrap(err, "cannot marshal object into YAML")
		}
		if err = ioutil.WriteFile(filepath.Join(*outputDir, name), o, os.ModePerm); err != nil {
			return errors.Wrap(err, "cannot write object YAML file")
		}
	}
	return nil
}

func cleanup(s string) string {
	return strings.ReplaceAll(s, ":", "_")
}
