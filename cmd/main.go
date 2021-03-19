package main

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/pkg/errors"
	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ghodss/yaml"

	"github.com/upbound/olm-bundle/internal/csv"
	"github.com/upbound/olm-bundle/internal/manifests"
)

var (
	chartFilePath = kingpin.Flag("chart-file-path", "Path to Helm Chart.yaml file to produce metadata").Short('c').String()
	// TODO(muvaf): Support printing to stdout.
	outputDir              = kingpin.Flag("output-dir", "Output directory to save the OLM bundle files").Short('o').Required().String()
	baseCSVPath            = kingpin.Flag("base-csv-path", "Base ClusterServiceVersion you want olm-bundle to use as template. This is useful for fields that cannot be filled by olm-bundle.").String()
	additionalResourcesDir = kingpin.Flag("extra-resources-dir", "Extra resources you would like to add to the OLM bundle.").Short('e').String()
)

func main() {
	kingpin.Parse()
	var extraFiles []string
	if *additionalResourcesDir != "" {
		err := filepath.Walk(*additionalResourcesDir, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			extraFiles = append(extraFiles, path)
			return nil
		})
		kingpin.FatalIfError(err, "cannot walk the extra resources directory")
	}
	p := manifests.NewParser(extraFiles, os.Stdin)
	resources, err := p.Parse()
	kingpin.FatalIfError(err, "cannot parse resources")
	result, err := csv.NewClusterServiceVersion(*baseCSVPath)
	kingpin.FatalIfError(err, "cannot initialize a new ClusterServiceVersion")
	if *chartFilePath != "" {
		hm := &manifests.HelmMetadata{
			ChartFilePath: *chartFilePath,
		}
		kingpin.FatalIfError(hm.Embed(result), "cannot embed metadata from Helm Chart.yaml file")
	}
	e := csv.NewEmbedder()
	left, err := e.Embed(resources, result)
	kingpin.FatalIfError(err, "cannot embed resources into ClusterServiceVersion file")
	kingpin.FatalIfError(csv.Validate(left), "cannot validate")
	kingpin.FatalIfError(os.MkdirAll(*outputDir, os.ModePerm), "cannot create output dir")
	kingpin.FatalIfError(write(left, result), "cannot write OLM bundle")
	fmt.Printf("✅ You can find your OLM bundle in %s\n🚀 Have fun!\n", *outputDir)
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
