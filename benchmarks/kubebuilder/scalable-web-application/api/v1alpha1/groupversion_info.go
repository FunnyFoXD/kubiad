// Package v1alpha1 contains API Schema definitions for the apps.kubiad.dev v1alpha1 API group
// +kubebuilder:object:generate=true
// +groupName=apps.kubiad.dev
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion identifies this API version
	GroupVersion = schema.GroupVersion{Group: "apps.kubiad.dev", Version: "v1alpha1"}

	// SchemeBuilder registers this API version with a runtime scheme
	SchemeBuilder = &schemeBuilder{GroupVersion: GroupVersion}

	// AddToScheme adds this API version to a runtime scheme
	AddToScheme = SchemeBuilder.AddToScheme
)

type schemeBuilder struct {
	GroupVersion schema.GroupVersion
}

// AddToScheme registers the ScalableWebApplication API types
func (b *schemeBuilder) AddToScheme(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(b.GroupVersion, &ScalableWebApplication{}, &ScalableWebApplicationList{})
	metav1.AddToGroupVersion(scheme, b.GroupVersion)
	return nil
}
