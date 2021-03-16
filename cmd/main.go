package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"

	"github.com/upbound/olm-bundle/internal/csv"
	"github.com/upbound/olm-bundle/internal/manifests"
)

const out = "/Users/monus/go/src/github.com/upbound/crossplane-distro/.work/test"

func main() {
	objects, err := manifests.Parse(os.Stdin)
	if err != nil {
		panic(err)
	}
	e := csv.NewEmbedder()
	result := csv.NewClusterServiceVersion()
	hm := &manifests.HelmMetadata{
		ChartFilePath: "/Users/monus/go/src/github.com/upbound/crossplane-distro/cluster/helm/project-uruk-hai/Chart.yaml",
	}
	if err := hm.Embed(result); err != nil {
		panic(err)
	}
	left, err := e.Embed(objects, result)
	if err != nil {
		panic(err)
	}
	left = append(left, result)
	for _, m := range left {
		name := fmt.Sprintf("%s.%s.yaml", cleanup(m.GetName()), m.GetObjectKind().GroupVersionKind().Kind)
		o, err := yaml.Marshal(m)
		if err != nil {
			panic(err)
		}
		err = ioutil.WriteFile(filepath.Join(out, name), o, 0644)
		if err != nil {
			panic(err)
		}
	}
	fmt.Println("done!")
}

func cleanup(s string) string {
	return strings.ReplaceAll(s, ":", "_")
}
