package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// WorkerApplicationSpec defines the desired state of WorkerApplication
type WorkerApplicationSpec struct {
	// Image is the container image for the worker
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Replicas is the desired number of worker replicas
	// +optional
	// +kubebuilder:default:=1
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	Replicas int32 `json:"replicas,omitempty"`
}

// WorkerApplicationStatus defines the observed state of WorkerApplication
type WorkerApplicationStatus struct {
	// Phase is Ready when the Deployment has reached its desired state
	Phase string `json:"phase,omitempty"`

	// ReadyReplicas is the observed number of ready Deployment replicas
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
}

// WorkerApplication is the Schema for the workerapplications API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=workerapp
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type WorkerApplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkerApplicationSpec   `json:"spec,omitempty"`
	Status WorkerApplicationStatus `json:"status,omitempty"`
}

// WorkerApplicationList contains a list of WorkerApplication resources
// +kubebuilder:object:root=true
type WorkerApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkerApplication `json:"items"`
}

// DeepCopyInto copies this object into out
func (in *WorkerApplication) DeepCopyInto(out *WorkerApplication) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
}

// DeepCopy creates a deep copy of this object
func (in *WorkerApplication) DeepCopy() *WorkerApplication {
	if in == nil {
		return nil
	}
	out := new(WorkerApplication)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject creates a deep copy as a runtime object
func (in *WorkerApplication) DeepCopyObject() runtime.Object {
	if copy := in.DeepCopy(); copy != nil {
		return copy
	}
	return nil
}

// DeepCopyInto copies this list into out
func (in *WorkerApplicationList) DeepCopyInto(out *WorkerApplicationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]WorkerApplication, len(in.Items))
		for index := range in.Items {
			in.Items[index].DeepCopyInto(&out.Items[index])
		}
	}
}

// DeepCopy creates a deep copy of this list
func (in *WorkerApplicationList) DeepCopy() *WorkerApplicationList {
	if in == nil {
		return nil
	}
	out := new(WorkerApplicationList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject creates a deep copy as a runtime object
func (in *WorkerApplicationList) DeepCopyObject() runtime.Object {
	if copy := in.DeepCopy(); copy != nil {
		return copy
	}
	return nil
}
