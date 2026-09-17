package v1alpha1

type WebApplicationSpec struct {
	Image string
	Replicas int32
	Port int32
}

type WebApplicationStatus struct {
	Phase string
	ReadyReplicas int32
}

type WebApplication struct {
	Spec WebApplicationSpec
	Status WebApplicationStatus
}
