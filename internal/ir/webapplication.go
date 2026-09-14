package ir

// WebApplication returns the manually constructed, validated IR for the first generator slice
func WebApplication() Program {
	const (
		imageField            FieldID    = "spec:image"
		replicasField         FieldID    = "spec:replicas"
		portField             FieldID    = "spec:port"
		phaseField            FieldID    = "status:phase"
		readyReplicasField    FieldID    = "status:readyReplicas"
		applicationDeployment ResourceID = "deployment:application"
		applicationService    ResourceID = "service:applicationService"
	)

	return Program{
		Operator: Operator{Name: "WebApplication", Plural: "webapplications"},
		API:      API{Group: "apps.kubiad.dev", Version: "v1alpha1", Kind: "WebApplication", Plural: "webapplications"},
		SpecFields: []SpecField{
			{ID: imageField, Name: "image", Type: StringType, Required: true},
			{ID: replicasField, Name: "replicas", Type: IntType, Default: &Constant{Type: IntType, Int: 1}, Constraints: Constraints{Minimum: int32Pointer(0), Maximum: int32Pointer(10)}},
			{ID: portField, Name: "port", Type: IntType, Default: &Constant{Type: IntType, Int: 8080}, Constraints: Constraints{Minimum: int32Pointer(1), Maximum: int32Pointer(65535)}},
		},
		StatusFields: []StatusField{
			{ID: phaseField, Name: "phase", Type: StringType},
			{ID: readyReplicasField, Name: "readyReplicas", Type: IntType},
		},
		Deployments: []Deployment{{
			ID: applicationDeployment, Name: "application",
			Image:         specReference(imageField, StringType),
			Replicas:      specReference(replicasField, IntType),
			ContainerPort: expressionPointer(specReference(portField, IntType)),
		}},
		Services: []Service{{
			ID: applicationService, Name: "applicationService", TargetDeploymentID: applicationDeployment,
			Port: specReference(portField, IntType), TargetPort: specReference(portField, IntType),
		}},
		Reconcile: Reconcile{
			Rules: []ReconcileRule{{
				Condition: observableReference(applicationDeployment, "ready", BoolType),
				Assignments: []StatusAssignment{
					{TargetFieldID: phaseField, Value: Expression{Kind: StringConstantExpression, Type: StringType, BindingTime: ReconcileTime, Provenance: Desired, StringValue: "Ready"}},
					{TargetFieldID: readyReplicasField, Value: observableReference(applicationDeployment, "readyReplicas", IntType)},
				},
			}},
			Otherwise: &ReconcileBlock{Assignments: []StatusAssignment{
				{TargetFieldID: phaseField, Value: Expression{Kind: StringConstantExpression, Type: StringType, BindingTime: ReconcileTime, Provenance: Desired, StringValue: "Progressing"}},
				{TargetFieldID: readyReplicasField, Value: observableReference(applicationDeployment, "readyReplicas", IntType)},
			}},
		},
	}
}

func specReference(id FieldID, typ Type) Expression {
	return Expression{Kind: SpecFieldReferenceExpression, Type: typ, BindingTime: ReconcileTime, Provenance: Desired, FieldID: id}
}

func observableReference(id ResourceID, property string, typ Type) Expression {
	return Expression{Kind: ObservableReferenceExpression, Type: typ, BindingTime: ReconcileTime, Provenance: Observed, ResourceID: id, Property: property}
}

func int32Pointer(value int32) *int32                { return &value }
func expressionPointer(value Expression) *Expression { return &value }
