package main

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/ghodss/yaml"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/upbound/olm-bundle/internal/csv"
	"github.com/upbound/olm-bundle/internal/manifests"
)

type OLMBundleCLI struct {
	ChartFilePath     string `help:"Path to Helm Chart.yaml file to produce metadata." type:"path"`
	OutputDir         string `help:"Output directory to save the OLM bundle files." type:"path" required:""`
	BaseCSVPath       string `help:"Base ClusterServiceVersion you want olm-bundle to use as template. This is useful for fields that cannot be filled by olm-bundle." type:"path"`
	ExtraResourcesDir string `help:"Extra resources you would like to add to the OLM bundle." type:"path"`
}

func main() {
	cli := &OLMBundleCLI{}
	ctx := kong.Parse(cli)
	var extraFiles []string
	if cli.ExtraResourcesDir != "" {
		err := filepath.Walk(cli.ExtraResourcesDir, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			extraFiles = append(extraFiles, path)
			return nil
		})
		ctx.FatalIfErrorf(err, "cannot walk the extra resources directory")
	}
	p := manifests.NewParser(extraFiles, os.Stdin)
	resources, err := p.Parse()
	ctx.FatalIfErrorf(err, "cannot parse resources")
	result, err := csv.NewClusterServiceVersion(cli.BaseCSVPath)
	ctx.FatalIfErrorf(err, "cannot initialize a new ClusterServiceVersion")
	if cli.ChartFilePath != "" {
		hm := &manifests.HelmMetadata{
			ChartFilePath: cli.ChartFilePath,
		}
		ctx.FatalIfErrorf(hm.Embed(result), "cannot embed metadata from Helm Chart.yaml file")
	}
	e := csv.NewEmbedder()
	left, err := e.Embed(resources, result)
	ctx.FatalIfErrorf(err, "cannot embed resources into ClusterServiceVersion file")
	ctx.FatalIfErrorf(csv.Validate(left), "cannot validate")
	ctx.FatalIfErrorf(os.MkdirAll(cli.OutputDir, os.ModePerm), "cannot create output dir")
	ctx.FatalIfErrorf(write(left, result, cli.OutputDir), "cannot write OLM bundle")
	fmt.Printf("✅ You can find your OLM bundle in %s\n🚀 Have fun!\n", cli.OutputDir)
}

func write(resources []*unstructured.Unstructured, csvo *v1alpha1.ClusterServiceVersion, dir string) error {
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
		if err = ioutil.WriteFile(filepath.Join(dir, name), o, os.ModePerm); err != nil {
			return errors.Wrap(err, "cannot write object YAML file")
		}
	}
	return nil
}

func cleanup(s string) string {
	return strings.ReplaceAll(s, ":", "_")
}
