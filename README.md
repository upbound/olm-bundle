# OLM Bundle

This tool can be used to produce an Operator Lifecycle Manager bundle from a
stream of YAMLs. You can additionally add metadata by using a Helm `Chart.yaml`
file.

Example:
```
helm template <chart> | olm-bundle --chart-file-path <chart>/Chart.yaml
```