package manifests

import (
	"fmt"
	"github.com/blang/semver/v4"
	"github.com/operator-framework/api/pkg/lib/version"
	"io/ioutil"

	"github.com/ghodss/yaml"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/chart"
)

type HelmMetadata struct {
	ChartFilePath string
}

func (hm *HelmMetadata) Embed(csv *v1alpha1.ClusterServiceVersion) error {
	f, err := ioutil.ReadFile(hm.ChartFilePath)
	if err != nil {
		return errors.Wrap(err, "cannot read chart file")
	}
	c := &chart.Metadata{}
	if err := yaml.Unmarshal(f, c); err != nil {
		return errors.Wrap(err, "cannot unmarshal chart file into metadata object")
	}
	csv.Name = fmt.Sprintf("%s.%s", c.Name, c.Version)
	v, err := semver.Make(c.Version)
	if err != nil {
		return errors.Wrap(err, "cannot make a semver version from version string in Helm metadata")
	}
	csv.Spec.Version = version.OperatorVersion{Version: v}
	csv.Spec.Description = c.Description
	csv.Spec.DisplayName = c.Name
	//csv.Spec.Icon = c.Icon
	csv.Spec.Provider = v1alpha1.AppLink{Name: c.Name, URL: c.Home}
	csv.Spec.Maintainers = make([]v1alpha1.Maintainer, len(c.Maintainers))
	for i := range c.Maintainers {
		csv.Spec.Maintainers[i] = v1alpha1.Maintainer{
			Name:  c.Maintainers[i].Name,
			Email: c.Maintainers[i].Email,
		}
	}
	csv.Spec.Keywords = c.Keywords
	return nil
}