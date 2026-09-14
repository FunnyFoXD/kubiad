package ir

import (
	"strings"
	"testing"
)

func TestWebApplicationIsValid(t *testing.T) {
	if err := WebApplication().Validate(); err != nil {
		t.Fatalf("validate WebApplication IR: %v", err)
	}
}

func TestValidateRejectsObservedDesiredDependency(t *testing.T) {
	program := WebApplication()
	program.Deployments[0].Replicas = observableReference("deployment:application", "readyReplicas", IntType)

	err := program.Validate()
	if err == nil || !strings.Contains(err.Error(), "observed value cannot define desired state") {
		t.Fatalf("validate error = %v, want observed desired dependency error", err)
	}
}

func TestValidateRequiresOtherwiseForStatusCoverage(t *testing.T) {
	program := WebApplication()
	program.Reconcile.Otherwise = nil

	err := program.Validate()
	if err == nil || !strings.Contains(err.Error(), "requires otherwise") {
		t.Fatalf("validate error = %v, want missing otherwise error", err)
	}
}
