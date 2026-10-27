package v1alpha1

import (
	"encoding/json"
	"testing"
)

func TestJSONPreservesZeroReplicas(t *testing.T) {
	application := WorkerApplication{Spec: WorkerApplicationSpec{Image: "nginx:1.27", Replicas: 0}}
	data, err := json.Marshal(application)
	if err != nil {
		t.Fatal(err)
	}
	var encoded struct {
		Spec map[string]json.RawMessage `json:"spec"`
	}
	if err := json.Unmarshal(data, &encoded); err != nil {
		t.Fatal(err)
	}
	if value := string(encoded.Spec["replicas"]); value != "0" {
		t.Fatalf("explicit zero replicas serialized as %q instead of 0: %s", value, data)
	}
}
