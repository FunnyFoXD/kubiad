package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// WebApplicationSpec defines the desired state of WebApplication
type WebApplicationSpec struct {
	// Image is the container image for the application
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Replicas is the desired number of application replicas
	// +optional
	// +kubebuilder:default:=1
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	Replicas int32 `json:"replicas,omitempty"`

	// Port is the application container and Service port
	// +optional
	// +kubebuilder:default:=8080
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`
}

// WebApplicationStatus defines the observed state of WebApplication
type WebApplicationStatus struct {
	// Phase is Ready when the Deployment has reached its desired state
	Phase string `json:"phase,omitempty"`

	// ReadyReplicas is the observed number of ready Deployment replicas
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
}

// WebApplication is the Schema for the webapplications API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=webapp
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type WebApplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WebApplicationSpec   `json:"spec,omitempty"`
	Status WebApplicationStatus `json:"status,omitempty"`
}

// WebApplicationList contains a list of WebApplication resources
// +kubebuilder:object:root=true
type WebApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WebApplication `json:"items"`
}

// DeepCopyInto copies this object into out
func (in *WebApplication) DeepCopyInto(out *WebApplication) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
}

// DeepCopy creates a deep copy of this object
func (in *WebApplication) DeepCopy() *WebApplication {
	if in == nil {
		return nil
	}
	out := new(WebApplication)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject creates a deep copy as a runtime object
func (in *WebApplication) DeepCopyObject() runtime.Object {
	if copy := in.DeepCopy(); copy != nil {
		return copy
	}
	return nil
}

// DeepCopyInto copies this list into out
func (in *WebApplicationList) DeepCopyInto(out *WebApplicationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]WebApplication, len(in.Items))
		for index := range in.Items {
			in.Items[index].DeepCopyInto(&out.Items[index])
		}
	}
}

// DeepCopy creates a deep copy of this list
func (in *WebApplicationList) DeepCopy() *WebApplicationList {
	if in == nil {
		return nil
	}
	out := new(WebApplicationList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject creates a deep copy as a runtime object
func (in *WebApplicationList) DeepCopyObject() runtime.Object {
	if copy := in.DeepCopy(); copy != nil {
		return copy
	}
	return nil
}
