# WorkerApplication Kubebuilder baseline

This standalone Go module is the manual reference implementation for the `WorkerApplication` Kubiad example.

It creates and maintains one Deployment through the `apps.kubiad.dev/v1alpha1` API. It intentionally does not create a Service.

Run the checks from this directory with:

```sh
go vet ./...
go test ./...
go build ./...
```
