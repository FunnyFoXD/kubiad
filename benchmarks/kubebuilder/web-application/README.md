# WebApplication Kubebuilder baseline

This standalone Go module is the manual reference implementation for the `WebApplication` Kubiad example.

It creates and maintains one Deployment and one ClusterIP Service through the `apps.kubiad.dev/v1alpha1` API.

Run the checks from this directory with:

```sh
go vet ./...
go test ./...
go build ./...
```
