package controller

type ResourcePlan struct { ID string; Kind string; Name string }
type AssignmentPlan struct { TargetField string; Expression string }
type RulePlan struct { Condition string; Assignments []AssignmentPlan }
type ReconcilePlan struct { Resources []ResourcePlan; Rules []RulePlan; Otherwise []AssignmentPlan }
type DeploymentStatus struct { Deleting bool; Generation int64; ObservedGeneration int64; UpdatedReplicas int32; Replicas int32; AvailableReplicas int32; ReadyReplicas int32 }

func EvaluateStatus(status DeploymentStatus, desiredReplicas int32) (string, int32) {
if status.Deleting || status.ObservedGeneration < status.Generation || status.UpdatedReplicas != desiredReplicas || status.Replicas != desiredReplicas || status.AvailableReplicas != desiredReplicas { return "Progressing", status.ReadyReplicas }
return "Ready", status.ReadyReplicas }

func Plan() ReconcilePlan { return ReconcilePlan{
Resources: []ResourcePlan{
{ID: "deployment:application", Kind: "Deployment", Name: "application"},{ID: "service:applicationService", Kind: "Service", Name: "applicationService"},},
Rules: []RulePlan{
{Condition: "deployment:application.ready", Assignments: []AssignmentPlan{{TargetField: "status:phase", Expression: "Ready"},{TargetField: "status:readyReplicas", Expression: "deployment:application.readyReplicas"},}},
}, Otherwise: []AssignmentPlan{
{TargetField: "status:phase", Expression: "Progressing"},{TargetField: "status:readyReplicas", Expression: "deployment:application.readyReplicas"},},} }
