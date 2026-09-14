package main

import (
	"flag"
	"os"

	"go.uber.org/zap/zapcore"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	kubiadv1alpha1 "github.com/FunnyFoXD/kubiad/benchmarks/kubebuilder/web-application/api/v1alpha1"
	"github.com/FunnyFoXD/kubiad/benchmarks/kubebuilder/web-application/internal/controller"
)

func main() {
	var development bool
	flag.BoolVar(&development, "development", false, "Enable development logging")
	flag.Parse()

	options := zap.Options{Development: development}
	options.BindFlags(flag.CommandLine)
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&options), zap.Level(zapcore.InfoLevel)))

	scheme := runtime.NewScheme()
	must(clientgoscheme.AddToScheme(scheme))
	must(appsv1.AddToScheme(scheme))
	must(corev1.AddToScheme(scheme))
	must(kubiadv1alpha1.AddToScheme(scheme))

	manager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{Scheme: scheme})
	if err != nil {
		ctrl.Log.WithName("setup").Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := (&controller.WebApplicationReconciler{Client: manager.GetClient(), Scheme: manager.GetScheme()}).SetupWithManager(manager); err != nil {
		ctrl.Log.WithName("setup").Error(err, "unable to create controller")
		os.Exit(1)
	}

	if err := manager.Start(ctrl.SetupSignalHandler()); err != nil {
		ctrl.Log.WithName("setup").Error(err, "manager exited")
		os.Exit(1)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
