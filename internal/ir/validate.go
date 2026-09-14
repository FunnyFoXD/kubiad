package ir

import "fmt"

// Validate checks the invariants required by generators
func (p Program) Validate() error {
	if p.Operator.Name == "" || p.Operator.Plural == "" || p.API.Group == "" || p.API.Version == "" || p.API.Kind == "" || p.API.Plural == "" {
		return fmt.Errorf("operator and API identity are required")
	}
	if p.Operator.Name != p.API.Kind || p.Operator.Plural != p.API.Plural {
		return fmt.Errorf("operator and API identity must match")
	}
	spec := map[FieldID]SpecField{}
	for _, f := range p.SpecFields {
		if f.ID == "" || f.Name == "" {
			return fmt.Errorf("spec field ID and name are required")
		}
		if _, ok := spec[f.ID]; ok {
			return fmt.Errorf("duplicate spec field %q", f.ID)
		}
		if f.Required && f.Default != nil {
			return fmt.Errorf("spec field %q cannot be required and defaulted", f.ID)
		}
		if f.Constraints.Minimum != nil && f.Constraints.Maximum != nil && *f.Constraints.Minimum > *f.Constraints.Maximum {
			return fmt.Errorf("spec field %q has an invalid range", f.ID)
		}
		if f.Default != nil {
			if f.Default.Type != f.Type {
				return fmt.Errorf("default for spec field %q has an invalid type", f.ID)
			}
			if f.Type == IntType && f.Constraints.Minimum != nil && f.Default.Int < *f.Constraints.Minimum {
				return fmt.Errorf("default for spec field %q is below its minimum", f.ID)
			}
			if f.Type == IntType && f.Constraints.Maximum != nil && f.Default.Int > *f.Constraints.Maximum {
				return fmt.Errorf("default for spec field %q is above its maximum", f.ID)
			}
		}
		spec[f.ID] = f
	}
	status := map[FieldID]StatusField{}
	for _, f := range p.StatusFields {
		if f.ID == "" || f.Name == "" {
			return fmt.Errorf("status field ID and name are required")
		}
		if _, exists := spec[f.ID]; exists {
			return fmt.Errorf("field ID %q is used by spec and status", f.ID)
		}
		if _, ok := status[f.ID]; ok {
			return fmt.Errorf("duplicate status field %q", f.ID)
		}
		status[f.ID] = f
	}
	deployments := map[ResourceID]Deployment{}
	for _, d := range p.Deployments {
		if d.ID == "" || d.Name == "" {
			return fmt.Errorf("deployment ID and name are required")
		}
		if _, ok := deployments[d.ID]; ok {
			return fmt.Errorf("duplicate deployment %q", d.ID)
		}
		if err := validateDesired(d.Image, StringType, spec, deployments); err != nil {
			return fmt.Errorf("deployment %q image: %w", d.ID, err)
		}
		if err := validateDesired(d.Replicas, IntType, spec, deployments); err != nil {
			return fmt.Errorf("deployment %q replicas: %w", d.ID, err)
		}
		if d.ContainerPort != nil {
			if err := validateDesired(*d.ContainerPort, IntType, spec, deployments); err != nil {
				return fmt.Errorf("deployment %q container port: %w", d.ID, err)
			}
		}
		deployments[d.ID] = d
	}
	serviceIDs := map[ResourceID]struct{}{}
	for _, s := range p.Services {
		if s.ID == "" || s.Name == "" {
			return fmt.Errorf("service ID and name are required")
		}
		if _, exists := deployments[s.ID]; exists {
			return fmt.Errorf("resource ID %q is used by a deployment and service", s.ID)
		}
		if _, exists := serviceIDs[s.ID]; exists {
			return fmt.Errorf("duplicate service %q", s.ID)
		}
		target, ok := deployments[s.TargetDeploymentID]
		if !ok {
			return fmt.Errorf("service %q targets unknown deployment %q", s.ID, s.TargetDeploymentID)
		}
		if target.ContainerPort == nil {
			return fmt.Errorf("service %q targets deployment without a container port", s.ID)
		}
		if err := validateDesired(s.Port, IntType, spec, deployments); err != nil {
			return fmt.Errorf("service %q port: %w", s.ID, err)
		}
		if err := validateDesired(s.TargetPort, IntType, spec, deployments); err != nil {
			return fmt.Errorf("service %q target port: %w", s.ID, err)
		}
		serviceIDs[s.ID] = struct{}{}
	}
	for _, rule := range p.Reconcile.Rules {
		if rule.Condition.Type != BoolType || rule.Condition.BindingTime != ReconcileTime {
			return fmt.Errorf("reconcile condition must be a reconcile-time bool")
		}
		if err := validateExpression(rule.Condition, spec, deployments); err != nil {
			return err
		}
		if err := validateAssignments(rule.Assignments, status, spec, deployments); err != nil {
			return err
		}
	}
	if len(p.Reconcile.Rules) > 0 && p.Reconcile.Otherwise == nil {
		return fmt.Errorf("reconcile with rules requires otherwise")
	}
	if p.Reconcile.Otherwise != nil {
		return validateAssignments(p.Reconcile.Otherwise.Assignments, status, spec, deployments)
	}
	return nil
}

func validateDesired(e Expression, expected Type, spec map[FieldID]SpecField, deployments map[ResourceID]Deployment) error {
	if e.Provenance != Desired {
		return fmt.Errorf("observed value cannot define desired state")
	}
	if e.Type != expected {
		return fmt.Errorf("expression has type %q, want %q", e.Type, expected)
	}
	return validateExpression(e, spec, deployments)
}

func validateExpression(e Expression, spec map[FieldID]SpecField, deployments map[ResourceID]Deployment) error {
	switch e.Kind {
	case StringConstantExpression:
		if e.Type != StringType || e.Provenance != Desired {
			return fmt.Errorf("invalid string constant")
		}
	case IntConstantExpression:
		if e.Type != IntType || e.Provenance != Desired {
			return fmt.Errorf("invalid int constant")
		}
	case SpecFieldReferenceExpression:
		f, ok := spec[e.FieldID]
		if !ok || e.Type != f.Type || e.BindingTime != ReconcileTime || e.Provenance != Desired {
			return fmt.Errorf("invalid spec field reference %q", e.FieldID)
		}
	case ObservableReferenceExpression:
		if _, ok := deployments[e.ResourceID]; !ok || e.BindingTime != ReconcileTime || e.Provenance != Observed {
			return fmt.Errorf("invalid observable reference %q", e.ResourceID)
		}
		if (e.Property == "ready" && e.Type != BoolType) || (e.Property == "readyReplicas" && e.Type != IntType) || (e.Property != "ready" && e.Property != "readyReplicas") {
			return fmt.Errorf("invalid observable property %q", e.Property)
		}
	default:
		return fmt.Errorf("unknown expression kind %q", e.Kind)
	}
	return nil
}

func validateAssignments(assignments []StatusAssignment, status map[FieldID]StatusField, spec map[FieldID]SpecField, deployments map[ResourceID]Deployment) error {
	assigned := map[FieldID]struct{}{}
	for _, a := range assignments {
		f, ok := status[a.TargetFieldID]
		if !ok {
			return fmt.Errorf("assignment targets unknown status field %q", a.TargetFieldID)
		}
		if _, ok := assigned[a.TargetFieldID]; ok {
			return fmt.Errorf("status field %q is assigned more than once", a.TargetFieldID)
		}
		if a.Value.Type != f.Type {
			return fmt.Errorf("assignment to status field %q has an invalid type", f.ID)
		}
		if err := validateExpression(a.Value, spec, deployments); err != nil {
			return err
		}
		assigned[a.TargetFieldID] = struct{}{}
	}
	for id := range status {
		if _, ok := assigned[id]; !ok {
			return fmt.Errorf("status field %q is not assigned on every reconcile path", id)
		}
	}
	return nil
}
