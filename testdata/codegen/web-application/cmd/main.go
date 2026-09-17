package main

import (
	"fmt"
	"os"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"example.test/webapplication/internal/controller"
)

func main() {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil { fail(err) }
	if err := appsv1.AddToScheme(scheme); err != nil { fail(err) }
	if err := corev1.AddToScheme(scheme); err != nil { fail(err) }
	manager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{Scheme: scheme})
	if err != nil { fail(err) }
	reconciler := &controller.Reconciler{Client: manager.GetClient(), Scheme: manager.GetScheme()}
	if err := reconciler.SetupWithManager(manager); err != nil { fail(err) }
	if err := manager.Start(ctrl.SetupSignalHandler()); err != nil { fail(err) }
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}