# ScalableWebApplication Kubebuilder baseline

This standalone Go module is the manual reference implementation for the `ScalableWebApplication` Kubiad example.

It creates and maintains one Deployment and one ClusterIP Service through the `apps.kubiad.dev/v1alpha1` API. The port is required and replicas may be zero.

Run the checks from this directory with:

```sh
go vet ./...
go test ./...
go build ./...
```
