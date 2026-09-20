package v1alpha1

type ScalableWebApplicationSpec struct {
	Image string
	Replicas int32
	Port int32
}

type ScalableWebApplicationStatus struct {
	Phase string
	ReadyReplicas int32
}

type ScalableWebApplication struct {
	Spec ScalableWebApplicationSpec
	Status ScalableWebApplicationStatus
}
