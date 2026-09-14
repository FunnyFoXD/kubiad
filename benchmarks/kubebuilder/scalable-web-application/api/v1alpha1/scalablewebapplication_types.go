package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ScalableWebApplicationSpec defines the desired state of ScalableWebApplication
type ScalableWebApplicationSpec struct {
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

	// Port is the required application container and Service port
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`
}

// ScalableWebApplicationStatus defines the observed state of ScalableWebApplication
type ScalableWebApplicationStatus struct {
	// Phase is Ready when the Deployment has reached its desired state
	Phase string `json:"phase,omitempty"`

	// ReadyReplicas is the observed number of ready Deployment replicas
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
}

// ScalableWebApplication is the Schema for the scalablewebapplications API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=scalablewebapp
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type ScalableWebApplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScalableWebApplicationSpec   `json:"spec,omitempty"`
	Status ScalableWebApplicationStatus `json:"status,omitempty"`
}

// ScalableWebApplicationList contains a list of ScalableWebApplication resources
// +kubebuilder:object:root=true
type ScalableWebApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScalableWebApplication `json:"items"`
}

// DeepCopyInto copies this object into out
func (in *ScalableWebApplication) DeepCopyInto(out *ScalableWebApplication) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
}

// DeepCopy creates a deep copy of this object
func (in *ScalableWebApplication) DeepCopy() *ScalableWebApplication {
	if in == nil {
		return nil
	}
	out := new(ScalableWebApplication)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject creates a deep copy as a runtime object
func (in *ScalableWebApplication) DeepCopyObject() runtime.Object {
	if copy := in.DeepCopy(); copy != nil {
		return copy
	}
	return nil
}

// DeepCopyInto copies this list into out
func (in *ScalableWebApplicationList) DeepCopyInto(out *ScalableWebApplicationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]ScalableWebApplication, len(in.Items))
		for index := range in.Items {
			in.Items[index].DeepCopyInto(&out.Items[index])
		}
	}
}

// DeepCopy creates a deep copy of this list
func (in *ScalableWebApplicationList) DeepCopy() *ScalableWebApplicationList {
	if in == nil {
		return nil
	}
	out := new(ScalableWebApplicationList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject creates a deep copy as a runtime object
func (in *ScalableWebApplicationList) DeepCopyObject() runtime.Object {
	if copy := in.DeepCopy(); copy != nil {
		return copy
	}
	return nil
}
