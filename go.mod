module github.com/upbound/olm-bundle

go 1.16

replace (
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
)

require (
	github.com/blang/semver/v4 v4.0.0
	github.com/ghodss/yaml v1.0.0
	github.com/operator-framework/api v0.6.1
	github.com/operator-framework/operator-registry v1.16.1
	github.com/pkg/errors v0.9.1
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	helm.sh/helm/v3 v3.5.3
	k8s.io/api v0.20.4
	k8s.io/apiextensions-apiserver v0.20.4
	k8s.io/apimachinery v0.20.4
	sigs.k8s.io/controller-runtime v0.8.3
)
