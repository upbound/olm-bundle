# OLM Bundle

This tool can be used to produce an [Operator Lifecycle Manager](https://docs.openshift.com/container-platform/4.7/operators/understanding/olm-packaging-format.html)
bundle from a stream of YAMLs. You can additionally add metadata by using a Helm
`Chart.yaml` file.

Example:
```
helm template <chart> | \
olm-bundle \
  --chart-file-path <chart>/Chart.yaml \
  --extra-resources-dir <folder containing extra YAMLs> \
  --output-dir <where to write the output>
```

You can use base template for the generated `ClusterServiceVersion` by having
`clusterserviceversion.yaml.tmpl` in the given output directory. Following is an
example:
```yaml
apiVersion: v1alpha1
kind: ClusterServiceVersion
metadata:
  creationTimestamp: null
spec:
  minKubeVersion: 1.16.0
  maturity: stable
  installModes:
    - supported: false
      type: OwnNamespace
    - supported: false
      type: SingleNamespace
    - supported: false
      type: MultiNamespace
    - supported: true
      type: AllNamespaces
```

Similarly, you can have `annotations.yaml.tmpl` in the output directory to have
it used as base. Following is an example:
```yaml
annotations:
  operators.operatorframework.io.bundle.mediatype.v1: "registry+v1"
  operators.operatorframework.io.bundle.manifests.v1: "manifests/"
  operators.operatorframework.io.bundle.metadata.v1: "metadata/"
  operators.operatorframework.io.bundle.package.v1: "test-operator"
  operators.operatorframework.io.bundle.channels.v1: "beta,stable"
  operators.operatorframework.io.bundle.channel.default.v1: "stable"
```