package ir

// WorkerApplication returns the validated IR for a Deployment-only operator
func WorkerApplication() Program {
	const (
		imageField         FieldID    = "spec:image"
		replicasField      FieldID    = "spec:replicas"
		phaseField         FieldID    = "status:phase"
		readyReplicasField FieldID    = "status:readyReplicas"
		workerDeployment   ResourceID = "deployment:worker"
	)

	return Program{
		Operator: Operator{Name: "WorkerApplication", Plural: "workerapplications"},
		API:      API{Group: "apps.kubiad.dev", Version: "v1alpha1", Kind: "WorkerApplication", Plural: "workerapplications"},
		SpecFields: []SpecField{
			{ID: imageField, Name: "image", Type: StringType, Required: true},
			{ID: replicasField, Name: "replicas", Type: IntType, Default: &Constant{Type: IntType, Int: 1}, Constraints: Constraints{Minimum: int32Pointer(0), Maximum: int32Pointer(10)}},
		},
		StatusFields: []StatusField{
			{ID: phaseField, Name: "phase", Type: StringType},
			{ID: readyReplicasField, Name: "readyReplicas", Type: IntType},
		},
		Deployments: []Deployment{{
			ID: workerDeployment, Name: "worker",
			Image:    specReference(imageField, StringType),
			Replicas: specReference(replicasField, IntType),
		}},
		Reconcile: deploymentStatusReconcile(workerDeployment, phaseField, readyReplicasField),
	}
}

// ScalableWebApplication returns the validated IR for an operator with a required Service port
func ScalableWebApplication() Program {
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
		Operator: Operator{Name: "ScalableWebApplication", Plural: "scalablewebapplications"},
		API:      API{Group: "apps.kubiad.dev", Version: "v1alpha1", Kind: "ScalableWebApplication", Plural: "scalablewebapplications"},
		SpecFields: []SpecField{
			{ID: imageField, Name: "image", Type: StringType, Required: true},
			{ID: replicasField, Name: "replicas", Type: IntType, Default: &Constant{Type: IntType, Int: 1}, Constraints: Constraints{Minimum: int32Pointer(0), Maximum: int32Pointer(10)}},
			{ID: portField, Name: "port", Type: IntType, Required: true, Constraints: Constraints{Minimum: int32Pointer(1), Maximum: int32Pointer(65535)}},
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
		Reconcile: deploymentStatusReconcile(applicationDeployment, phaseField, readyReplicasField),
	}
}

func deploymentStatusReconcile(deployment ResourceID, phase, readyReplicas FieldID) Reconcile {
	return Reconcile{
		Rules: []ReconcileRule{{
			Condition: observableReference(deployment, "ready", BoolType),
			Assignments: []StatusAssignment{
				{TargetFieldID: phase, Value: Expression{Kind: StringConstantExpression, Type: StringType, BindingTime: ReconcileTime, Provenance: Desired, StringValue: "Ready"}},
				{TargetFieldID: readyReplicas, Value: observableReference(deployment, "readyReplicas", IntType)},
			},
		}},
		Otherwise: &ReconcileBlock{Assignments: []StatusAssignment{
			{TargetFieldID: phase, Value: Expression{Kind: StringConstantExpression, Type: StringType, BindingTime: ReconcileTime, Provenance: Desired, StringValue: "Progressing"}},
			{TargetFieldID: readyReplicas, Value: observableReference(deployment, "readyReplicas", IntType)},
		}},
	}
}
