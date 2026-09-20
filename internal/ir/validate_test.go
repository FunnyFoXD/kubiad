package ir

import (
	"strings"
	"testing"
)

func TestReferenceProgramsAreValid(t *testing.T) {
	for name, program := range map[string]Program{
		"WebApplication":         WebApplication(),
		"WorkerApplication":      WorkerApplication(),
		"ScalableWebApplication": ScalableWebApplication(),
	} {
		t.Run(name, func(t *testing.T) {
			if err := program.Validate(); err != nil {
				t.Fatalf("validation failed: %v", err)
			}
		})
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

func TestValidateRejectsCorpusStaticCases(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*Program)
		contains string
	}{
		{
			name: "P-01 observed ready replicas define desired deployment replicas",
			mutate: func(program *Program) {
				program.Deployments[0].Replicas = observableReference("deployment:application", "readyReplicas", IntType)
			},
			contains: "observed value cannot define desired state",
		},
		{
			name: "P-03 observed ready replicas define desired service port",
			mutate: func(program *Program) {
				program.Services[0].Port = observableReference("deployment:application", "readyReplicas", IntType)
			},
			contains: "observed value cannot define desired state",
		},
		{
			name: "T-01 string assigned to deployment replicas",
			mutate: func(program *Program) {
				program.Deployments[0].Replicas = specReference("spec:image", StringType)
			},
			contains: "expression has type \"string\", want \"int\"",
		},
		{
			name: "T-03 reconcile condition is not bool",
			mutate: func(program *Program) {
				program.Reconcile.Rules[0].Condition = specReference("spec:replicas", IntType)
			},
			contains: "reconcile condition must be a reconcile-time bool",
		},
		{
			name: "V-02 range lower bound exceeds upper bound",
			mutate: func(program *Program) {
				minimum, maximum := int32(10), int32(1)
				program.SpecFields[1].Constraints = Constraints{Minimum: &minimum, Maximum: &maximum}
			},
			contains: "has an invalid range",
		},
		{
			name: "V-03 default exceeds declared maximum",
			mutate: func(program *Program) {
				program.SpecFields[1].Default = &Constant{Type: IntType, Int: 11}
			},
			contains: "is above its maximum",
		},
		{
			name: "R-01 service targets an unknown deployment",
			mutate: func(program *Program) {
				program.Services[0].TargetDeploymentID = "deployment:backend"
			},
			contains: "targets unknown deployment",
		},
		{
			name: "R-03 service target has no container port",
			mutate: func(program *Program) {
				program.Deployments[0].ContainerPort = nil
			},
			contains: "targets deployment without a container port",
		},
		{
			name: "R-06 duplicate deployment identifier",
			mutate: func(program *Program) {
				program.Deployments = append(program.Deployments, program.Deployments[0])
			},
			contains: "duplicate deployment",
		},
		{
			name: "S-02 otherwise leaves status field unassigned",
			mutate: func(program *Program) {
				program.Reconcile.Otherwise.Assignments = program.Reconcile.Otherwise.Assignments[:1]
			},
			contains: "is not assigned on every reconcile path",
		},
		{
			name: "S-03 branch assigns one status field twice",
			mutate: func(program *Program) {
				assignment := program.Reconcile.Rules[0].Assignments[0]
				program.Reconcile.Rules[0].Assignments = append(program.Reconcile.Rules[0].Assignments, assignment)
			},
			contains: "is assigned more than once",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			program := WebApplication()
			test.mutate(&program)

			err := program.Validate()
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("Validate() error = %v, want message containing %q", err, test.contains)
			}
		})
	}
}
