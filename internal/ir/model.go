// Package ir defines the validated, generator-facing representation of a Kubiad operator
package ir

type FieldID string
type ResourceID string
type Type string

const (
	StringType              Type = "string"
	IntType                 Type = "int"
	BoolType                Type = "bool"
	DeploymentReferenceType Type = "deployment-reference"
)

type BindingTime string

const (
	CompileTime   BindingTime = "compile-time"
	ReconcileTime BindingTime = "reconcile-time"
)

type Provenance string

const (
	Desired  Provenance = "desired"
	Observed Provenance = "observed"
)

type ExpressionKind string

const (
	StringConstantExpression      ExpressionKind = "string-constant"
	IntConstantExpression         ExpressionKind = "int-constant"
	SpecFieldReferenceExpression  ExpressionKind = "spec-field-reference"
	ObservableReferenceExpression ExpressionKind = "observable-property-reference"
)

type SourceSpan struct {
	File                                       string
	StartLine, StartColumn, EndLine, EndColumn int
}
type Operator struct{ Name, Plural string }
type API struct{ Group, Version, Kind, Plural string }
type Constraints struct{ Minimum, Maximum *int32 }
type Constant struct {
	Type   Type
	String string
	Int    int32
	Source SourceSpan
}
type SpecField struct {
	ID          FieldID
	Name        string
	Type        Type
	Required    bool
	Default     *Constant
	Constraints Constraints
	Source      SourceSpan
}
type StatusField struct {
	ID     FieldID
	Name   string
	Type   Type
	Source SourceSpan
}

type Expression struct {
	Kind        ExpressionKind
	Type        Type
	BindingTime BindingTime
	Provenance  Provenance
	StringValue string
	IntValue    int32
	FieldID     FieldID
	ResourceID  ResourceID
	Property    string
	Source      SourceSpan
}

type Deployment struct {
	ID              ResourceID
	Name            string
	Image, Replicas Expression
	ContainerPort   *Expression
	Source          SourceSpan
}
type Service struct {
	ID                 ResourceID
	Name               string
	TargetDeploymentID ResourceID
	Port, TargetPort   Expression
	Source             SourceSpan
}
type StatusAssignment struct {
	TargetFieldID FieldID
	Value         Expression
	Source        SourceSpan
}
type ReconcileRule struct {
	Condition   Expression
	Assignments []StatusAssignment
	Source      SourceSpan
}
type ReconcileBlock struct {
	Assignments []StatusAssignment
	Source      SourceSpan
}
type Reconcile struct {
	Rules     []ReconcileRule
	Otherwise *ReconcileBlock
}
type Program struct {
	Operator     Operator
	API          API
	SpecFields   []SpecField
	StatusFields []StatusField
	Deployments  []Deployment
	Services     []Service
	Reconcile    Reconcile
}
