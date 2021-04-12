package manifests

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/blang/semver/v4"
	"github.com/ghodss/yaml"
	"github.com/operator-framework/api/pkg/lib/version"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/chart"
)

// HelmMetadata writes metadata parsed from a Helm chart to ClusterServiceVersion.
type HelmMetadata struct {
	ChartFilePath string
	Version       string
}

// Embed reads Chart.yaml and puts all the matching available metadata into
// given ClusterServiceVersion.
func (hm *HelmMetadata) Embed(ctx context.Context, csv *v1alpha1.ClusterServiceVersion) error {
	f, err := ioutil.ReadFile(hm.ChartFilePath)
	if err != nil {
		return errors.Wrap(err, "cannot read chart file")
	}
	c := &chart.Metadata{}
	if err := yaml.Unmarshal(f, c); err != nil {
		return errors.Wrap(err, "cannot unmarshal chart file into metadata object")
	}
	ver := c.Version
	if hm.Version != "" {
		ver = hm.Version
	}
	csv.Name = fmt.Sprintf("%s.%s", c.Name, ver)
	v, err := semver.Make(ver)
	if err != nil {
		return errors.Wrap(err, "cannot make a semver version from version string in Helm metadata")
	}
	csv.Spec.Version = version.OperatorVersion{Version: v}
	csv.Spec.Description = c.Description
	csv.Spec.DisplayName = c.Name
	if c.Icon != "" {
		i, err := getIconData(ctx, c.Icon)
		if err != nil {
			return errors.Wrapf(err, "cannot get icon data from %s", c.Icon)
		}
		csv.Spec.Icon = []v1alpha1.Icon{i}
	}
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

func getIconData(ctx context.Context, url string) (v1alpha1.Icon, error) {
	// TODO(muvaf): Is there way to avoid having variable url?
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return v1alpha1.Icon{}, errors.Wrapf(err, "cannot create request for %s", url)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return v1alpha1.Icon{}, errors.Wrapf(err, "cannot do request for %s", url)
	}
	defer resp.Body.Close() // nolint:errcheck
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return v1alpha1.Icon{}, errors.Wrap(err, "cannot read downloaded image data")
	}
	i := v1alpha1.Icon{
		Data:      base64.StdEncoding.EncodeToString(content),
		MediaType: http.DetectContentType(content),
	}
	if i.MediaType != "image/png" && i.MediaType != "image/svg" {
		return v1alpha1.Icon{}, errors.Errorf("media type %s is not supported. supported media types are image/png and image/svg", i.MediaType)
	}
	return i, nil
}
