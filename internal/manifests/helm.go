package manifests

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/ghodss/yaml"
	"github.com/operator-framework/api/pkg/lib/version"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/chart"
	"k8s.io/apimachinery/pkg/util/mergepatch"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// HelmMetadata writes metadata parsed from a Helm chart to ClusterServiceVersion.
type HelmMetadata struct {
	ChartFilePath      string
	Version            string
	HelmChartOverrides bool
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
	v, err := semver.Make(strings.TrimPrefix(ver, "v"))
	if err != nil {
		return errors.Wrap(err, "cannot make a semver version from version string in Helm metadata")
	}
	csv.Spec.Version = version.OperatorVersion{Version: v}
	csv.Spec.Description = c.Description
	if c.Icon != "" {
		i, err := getIconData(ctx, c.Icon)
		if err != nil {
			return errors.Wrapf(err, "cannot get icon data from %s", c.Icon)
		}
		csv.Spec.Icon = []v1alpha1.Icon{i}
	}
	csv.Spec.Maintainers = make([]v1alpha1.Maintainer, len(c.Maintainers))
	for i := range c.Maintainers {
		csv.Spec.Maintainers[i] = v1alpha1.Maintainer{
			Name:  c.Maintainers[i].Name,
			Email: c.Maintainers[i].Email,
		}
	}
	csv.Spec.Links = getLinks(c)
	csv.Spec.Keywords = c.Keywords
	if csv.GetAnnotations() == nil {
		csv.SetAnnotations(map[string]string{})
	}
	if c.Annotations != nil {
		err = enrichWithArtifacthubAnns(c.Annotations, csv, hm.HelmChartOverrides)
	}

	return err
}

func getLinks(c *chart.Metadata) []v1alpha1.AppLink {
	links := []v1alpha1.AppLink{}
	if c.Home != "" {
		links = append(links, v1alpha1.AppLink{
			Name: "Home",
			URL:  c.Home,
		})
	}
	if len(c.Sources) > 1 {
		for i, s := range c.Sources {
			links = append(links, v1alpha1.AppLink{
				Name: fmt.Sprintf("Source %d", i+1),
				URL:  s,
			})
		}
	} else if len(c.Sources) == 1 {
		links = append(links, v1alpha1.AppLink{
			Name: "Source",
			URL:  c.Sources[0],
		})
	}

	return links
}

// Using fields defined in https://artifacthub.io/docs/topics/annotations/helm/
func enrichWithArtifacthubAnns(ann map[string]string, csv *v1alpha1.ClusterServiceVersion, overrides bool) error {
	if ann["artifacthub.io/operator"] != "true" {
		return nil
	}

	// capabilities
	if val, ok := ann["artifacthub.io/operatorCapabilities"]; ok {
		csv.Annotations["capabilities"] = val
	}
	// maintainers
	hm, errM := unmarshalField(ann, "artifacthub.io/maintainers", []v1alpha1.Maintainer{})
	if errM != nil {
		return errM
	}
	if hm != nil && overrides {
		csv.Spec.Maintainers = hm.([]v1alpha1.Maintainer)
	}

	// crd definitions
	err := enrichCrds(ann, csv, overrides)
	if err != nil {
		return err
	}

	// crd examples
	if val, ok := ann["artifacthub.io/crdsExamples"]; ok {
		json, err := yaml.YAMLToJSON([]byte(val))
		if err != nil {
			return errors.Wrap(err, "cannot unmarshal artifacthub.io/crdsExamples in Chart.yaml")
		}
		csv.Annotations["alm-examples"] = string(json)
	}

	return nil
}

func enrichCrds(ann map[string]string, csv *v1alpha1.ClusterServiceVersion, overrides bool) error {
	hCrdV, errC := unmarshalField(ann, "artifacthub.io/crds", []v1alpha1.CRDDescription{})
	if errC != nil {
		return errC
	}
	if hCrdV != nil {
		hCrd := hCrdV.([]v1alpha1.CRDDescription)
		var err error
		csv.Spec.CustomResourceDefinitions.Owned, err = mergeCrds(csv.Spec.CustomResourceDefinitions.Owned, hCrd, overrides)
		if err != nil {
			return errors.Wrap(err, "cannot merge owned crds (coming from helm chart w/ those coming from the filesystem)")
		}
	}
	return nil
}

// merge the CRDDescription slice coming from filesystem traversal with the slice coming from Chart.yaml
// using strategic merge (the function doesn't work with slices directly therefore we need to hook it on
// CustomResourceDefinitions struct)
func mergeCrds(orig, fromHelm []v1alpha1.CRDDescription, overrides bool) ([]v1alpha1.CRDDescription, error) {
	if orig != nil {
		if overrides {
			return fromHelm, nil
		}
		origCrds, helmCrds := &v1alpha1.CustomResourceDefinitions{}, &v1alpha1.CustomResourceDefinitions{}
		origCrds.Owned, helmCrds.Owned = orig, fromHelm
		origCrdsJSON, _ := json.Marshal(origCrds)
		helmCrdsJSON, _ := json.Marshal(fromHelm)
		if origCrdsJSON == nil || helmCrdsJSON == nil {
			return nil, errors.New("can't marshal to json")
		}
		if c, e := mergepatch.HasConflicts(origCrds, helmCrds); c || e != nil {
			return fromHelm, nil
		}
		patchBytes, e1 := strategicpatch.CreateTwoWayMergePatch(origCrdsJSON, helmCrdsJSON, v1alpha1.CustomResourceDefinitions{})

		if e1 != nil {
			return nil, errors.Wrap(e1, "cannot calculate patch")
		}
		resultBytes, e2 := strategicpatch.StrategicMergePatch(origCrdsJSON, patchBytes, orig)
		if e2 != nil {
			return nil, errors.Wrap(e2, "cannot apply patch")
		}
		result := &v1alpha1.CustomResourceDefinitions{}
		err := json.Unmarshal(resultBytes, result)
		if err != nil {
			return nil, errors.Wrap(err, "cannot unmarshal the result")
		}
		return result.Owned, nil
	}
	return fromHelm, nil
}

func unmarshalField(ann map[string]string, key string, t interface{}) (interface{}, error) {
	if val, ok := ann[key]; ok {
		switch typ := t.(type) {
		case []v1alpha1.CRDDescription:
			helmVal := &[]v1alpha1.CRDDescription{}
			if err := yaml.Unmarshal([]byte(val), helmVal); err != nil {
				return nil, errors.Wrap(err, fmt.Sprintf("cannot unmarshal %s as type %T in Chart.yaml", key, typ))
			}
			return *helmVal, nil
		case []v1alpha1.Maintainer:
			helmVal := &[]v1alpha1.Maintainer{}
			if err := yaml.Unmarshal([]byte(val), helmVal); err != nil {
				return nil, errors.Wrap(err, fmt.Sprintf("cannot unmarshal %s as type %T in Chart.yaml", key, typ))
			}
			return *helmVal, nil
		default:
			return nil, errors.New(fmt.Sprintf("Unsupported type: %T", typ))
		}
	}
	return nil, nil
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
