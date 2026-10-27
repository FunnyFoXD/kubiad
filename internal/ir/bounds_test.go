package ir

import (
	"strings"
	"testing"
)

func TestValidateDesiredIntegerLiteralBounds(t *testing.T) {
	for _, test := range []struct {
		name   string
		value  int32
		assign func(*Program, Expression)
		valid  bool
	}{
		{name: "negative replicas", value: -1, assign: setReplicas},
		{name: "zero replicas", value: 0, assign: setReplicas, valid: true},
		{name: "zero container port", value: 0, assign: setContainerPort},
		{name: "minimum container port", value: 1, assign: setContainerPort, valid: true},
		{name: "maximum container port", value: 65535, assign: setContainerPort, valid: true},
		{name: "container port overflow", value: 65536, assign: setContainerPort},
		{name: "zero service port", value: 0, assign: setServicePort},
		{name: "minimum service port", value: 1, assign: setServicePort, valid: true},
		{name: "maximum service port", value: 65535, assign: setServicePort, valid: true},
		{name: "service port overflow", value: 65536, assign: setServicePort},
		{name: "zero target port", value: 0, assign: setTargetPort},
		{name: "target port overflow", value: 65536, assign: setTargetPort},
	} {
		t.Run(test.name, func(t *testing.T) {
			program := WebApplication()
			test.assign(&program, Expression{Kind: IntConstantExpression, Type: IntType, BindingTime: CompileTime, Provenance: Desired, IntValue: test.value})
			err := program.Validate()
			if test.valid && err != nil {
				t.Fatalf("valid literal rejected: %v", err)
			}
			if !test.valid && (err == nil || !strings.Contains(err.Error(), "outside allowed range")) {
				t.Fatalf("Validate() error = %v, want literal bounds error", err)
			}
		})
	}
}

func setReplicas(program *Program, value Expression) { program.Deployments[0].Replicas = value }
func setContainerPort(program *Program, value Expression) {
	program.Deployments[0].ContainerPort = &value
}
func setServicePort(program *Program, value Expression) { program.Services[0].Port = value }
func setTargetPort(program *Program, value Expression)  { program.Services[0].TargetPort = value }
