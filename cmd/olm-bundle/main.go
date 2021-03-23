package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/alecthomas/kong"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/upbound/olm-bundle/internal/csv"
	"github.com/upbound/olm-bundle/internal/manifests"
	"github.com/upbound/olm-bundle/internal/writer"
)

type olmBundleCLI struct {
	ChartFilePath     string `help:"Path to Helm Chart.yaml file to produce metadata." type:"path"`
	OutputDir         string `help:"Output directory to save the OLM bundle files." type:"path" required:""`
	ExtraResourcesDir string `help:"Extra resources you would like to add to the OLM bundle." type:"path"`
}

func main() {
	cli := &olmBundleCLI{}
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
	result, err := csv.NewClusterServiceVersion(cli.OutputDir)
	ctx.FatalIfErrorf(err, "cannot initialize a new ClusterServiceVersion")
	if cli.ChartFilePath != "" {
		hm := &manifests.HelmMetadata{
			ChartFilePath: cli.ChartFilePath,
		}
		ctx.FatalIfErrorf(hm.Embed(context.TODO(), result), "cannot embed metadata from Helm Chart.yaml file")
	}
	e := csv.NewEmbedder()
	left, err := e.Embed(resources, result)
	ctx.FatalIfErrorf(err, "cannot embed resources into ClusterServiceVersion file")
	ann, err := csv.NewAnnotations(cli.OutputDir)
	ctx.FatalIfErrorf(err, "cannot create a new annotations object")
	if result.GetAnnotations() == nil {
		result.SetAnnotations(map[string]string{})
	}
	for k, v := range ann {
		result.GetAnnotations()[k] = v
	}
	ctx.FatalIfErrorf(csv.Validate(left), "cannot validate")
	out := make([]client.Object, len(left)+1)
	for i, u := range left {
		out[i] = u
	}
	out[len(out)-1] = result
	b := &writer.Bundle{
		PackageDir: cli.OutputDir,
		Manifests:  out,
		Version:    result.Spec.Version.String(),
		Metadata: writer.Metadata{
			Annotations: ann,
		},
	}
	ctx.FatalIfErrorf(b.Write(), "cannot write bundle")
	fmt.Printf("✨ You can find your OLM bundle in %s\n🚀 Have fun!\n", cli.OutputDir)
}
