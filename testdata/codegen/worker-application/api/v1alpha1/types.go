package v1alpha1

type WorkerApplicationSpec struct {
	Image string
	Replicas int32
}

type WorkerApplicationStatus struct {
	Phase string
	ReadyReplicas int32
}

type WorkerApplication struct {
	Spec WorkerApplicationSpec
	Status WorkerApplicationStatus
}
