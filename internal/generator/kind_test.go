//go:build kind

package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestGeneratedKind exercises the source CLI and deployed operators with real Pods
func TestGeneratedKind(t *testing.T) {
	if os.Getenv("KUBIAD_KIND_TEST") != "1" {
		t.Fatal("set KUBIAD_KIND_TEST=1; see docs/kind-tests.md")
	}
	t.Setenv("KIND_EXPERIMENTAL_PROVIDER", "docker")
	for _, executable := range []string{"go", "docker", "kind", "kubectl"} {
		if _, err := exec.LookPath(executable); err != nil {
			t.Fatal(err)
		}
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	cli := filepath.Join(work, "kubiad")
	kindCommand(t, root, nil, "go", "build", "-o", cli, "./cmd/kubiad")
	kindCommand(t, root, nil, "docker", "info", "--format", "{{.ServerVersion}}")
	runID := fmt.Sprintf("%d", time.Now().UnixNano())
	workloadImage := "kubiad-smoke-workload:" + runID
	writeKindFile(t, filepath.Join(work, "main.go"), kindWorkloadSource)
	writeKindFile(t, filepath.Join(work, "Dockerfile"), kindWorkloadDockerfile)
	t.Cleanup(func() { kindCleanupCommand(t, "docker", "image", "rm", workloadImage) })
	kindCommand(t, work, nil, "docker", "build", "-t", workloadImage, ".")
	for _, golden := range operatorGoldens() {
		t.Run(golden.name, func(t *testing.T) {
			project := filepath.Join(t.TempDir(), "operator")
			kindCommand(t, root, nil, cli, "build", filepath.Join("examples", golden.directory+".kbi"), "--output", project, "--module", golden.module)
			managerImage := "kubiad-smoke-manager:" + strings.ToLower(golden.name) + "-" + runID
			t.Cleanup(func() { kindCleanupCommand(t, "docker", "image", "rm", managerImage) })
			kindCommand(t, project, nil, "docker", "build", "-t", managerImage, ".")
			e := &kindEnvironment{t: t, kubeconfig: filepath.Join(t.TempDir(), "kubeconfig"), primary: golden.program().API.Plural + ".apps.kubiad.dev"}
			cluster := "kubiad-" + strings.ToLower(golden.name) + "-" + runID
			nodeImage := os.Getenv("KUBIAD_KIND_NODE_IMAGE")
			if nodeImage == "" {
				nodeImage = "kindest/node:v1.35.0"
			}
			t.Cleanup(func() {
				if t.Failed() {
					e.diagnostics()
				}
				kindCleanupCommand(t, "kind", "delete", "cluster", "--name", cluster, "--kubeconfig", e.kubeconfig)
			})
			kindCommand(t, root, nil, "kind", "create", "cluster", "--name", cluster, "--image", nodeImage, "--kubeconfig", e.kubeconfig, "--wait", "180s")
			kindCommand(t, root, nil, "kind", "load", "docker-image", "--name", cluster, managerImage, workloadImage)
			writeKindFile(t, filepath.Join(project, "smoke", "kustomization.yaml"), "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - ../config/default\nimages:\n  - name: controller\n    newName: kubiad-smoke-manager\n    newTag: "+strings.TrimPrefix(managerImage, "kubiad-smoke-manager:")+"\n")
			e.kubectl(nil, "apply", "-k", filepath.Join(project, "smoke"))
			e.kubectl(nil, "wait", "--for=condition=Established", "crd/"+golden.program().API.Plural+".apps.kubiad.dev", "--timeout=60s")
			e.kubectl(nil, "-n", "kubiad-system", "rollout", "status", "deployment/kubiad-controller-manager", "--timeout=120s")
			e.exercise(golden, workloadImage)
		})
	}
}

type kindEnvironment struct {
	t          *testing.T
	kubeconfig string
	primary    string
}

func (e *kindEnvironment) kubectl(input []byte, args ...string) string {
	e.t.Helper()
	return kindCommand(e.t, "", input, "kubectl", append([]string{"--kubeconfig", e.kubeconfig, "--request-timeout=15s"}, args...)...)
}

func (e *kindEnvironment) exercise(golden operatorGolden, image string) {
	t := e.t
	program := golden.program()
	resource := program.API.Plural + "." + program.API.Group
	name := "smoke"
	namespace := "kubiad-smoke"
	deployment := name + "-" + kubernetesNamePart(program.Deployments[0].Name)
	e.kubectl(nil, "create", "namespace", namespace)
	spec := map[string]any{"image": image}
	if golden.name == "ScalableWebApplication" {
		spec["port"] = 8080
		spec["replicas"] = 0
	}
	application := map[string]any{
		"apiVersion": program.API.Group + "/" + program.API.Version,
		"kind":       program.API.Kind,
		"metadata":   map[string]any{"name": name, "namespace": namespace},
		"spec":       spec,
	}
	data, err := json.Marshal(application)
	if err != nil {
		t.Fatal(err)
	}
	e.kubectl(data, "apply", "-f", "-")
	initial := 1
	if golden.name == "ScalableWebApplication" {
		initial = 0
	}
	e.waitReady(resource, namespace, name, deployment, initial)
	t.Log("source CLI, container startup, ServiceAccount authentication and initial readiness passed")
	for _, replicas := range []int{0, 3} {
		e.kubectl(nil, "-n", namespace, "patch", resource, name, "--type=merge", "-p", fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
		e.waitReady(resource, namespace, name, deployment, replicas)
	}
	t.Log("real Deployment scale-to-zero and zero-to-three passed")
	var service string
	if len(program.Services) != 0 {
		service = name + "-" + kubernetesNamePart(program.Services[0].Name)
		e.kubectl(nil, "-n", namespace, "run", "service-probe", "--image="+image, "--image-pull-policy=Never", "--restart=Never", "--command", "--", "/workload", "--check", "http://"+service+":8080")
		e.kubectl(nil, "-n", namespace, "wait", "--for=jsonpath={.status.phase}=Succeeded", "pod/service-probe", "--timeout=60s")
		if logs := e.kubectl(nil, "-n", namespace, "logs", "service-probe"); !strings.Contains(logs, "kubiad-smoke") {
			t.Fatalf("unexpected Service response: %s", logs)
		}
		e.kubectl(nil, "-n", namespace, "delete", "pod/service-probe", "--wait=true")
	} else if services := e.kubectl(nil, "-n", namespace, "get", "services", "-o", "name"); strings.TrimSpace(services) != "" {
		t.Fatalf("worker created Services: %s", services)
	}
	e.kubectl(nil, "-n", namespace, "patch", "deployment", deployment, "--type=merge", "-p", `{"spec":{"replicas":1}}`)
	e.waitReady(resource, namespace, name, deployment, 3)
	e.kubectl(nil, "-n", namespace, "delete", "deployment", deployment, "--wait=true")
	e.waitReady(resource, namespace, name, deployment, 3)
	if service != "" {
		e.kubectl(nil, "-n", namespace, "delete", "service", service, "--wait=true")
		e.wait("Service recreation", func() bool {
			return e.get("service", namespace, service) != nil
		})
	}
	e.kubectl(nil, "-n", "kubiad-system", "rollout", "restart", "deployment/kubiad-controller-manager")
	e.kubectl(nil, "-n", "kubiad-system", "rollout", "status", "deployment/kubiad-controller-manager", "--timeout=120s")
	e.kubectl(nil, "-n", namespace, "patch", resource, name, "--type=merge", "-p", `{"spec":{"replicas":2}}`)
	e.waitReady(resource, namespace, name, deployment, 2)
	t.Log("drift correction, owned-resource recreation and manager restart passed")
	e.kubectl(nil, "-n", namespace, "delete", resource, name, "--wait=true")
	e.wait("garbage collection of workload resources", func() bool {
		for _, kind := range []string{"deployments", "replicasets", "pods", "services"} {
			list := e.get(kind, namespace, "")
			items, ok := list["items"].([]any)
			if !ok || len(items) != 0 {
				return false
			}
		}
		return true
	})
	t.Log("CR deletion garbage-collected Deployment, ReplicaSets, Pods and Service")
	logs := e.kubectl(nil, "-n", "kubiad-system", "logs", "deployment/kubiad-controller-manager")
	if strings.Contains(logs, "log.SetLogger(...) was never called") {
		t.Fatal("manager did not initialize controller-runtime logging")
	}
}

func (e *kindEnvironment) waitReady(resource, namespace, name, deployment string, replicas int) {
	e.t.Helper()
	e.wait(fmt.Sprintf("%s Ready with %d actual Pods", resource, replicas), func() bool {
		app := e.get(resource, namespace, name)
		dep := e.get("deployment", namespace, deployment)
		if app == nil || dep == nil {
			return false
		}
		status := kindMap(app, "status")
		depStatus := kindMap(dep, "status")
		if status["phase"] != "Ready" || kindNumber(status, "readyReplicas") != replicas || kindNumber(kindMap(dep, "spec"), "replicas") != replicas {
			return false
		}
		if kindNumber(depStatus, "observedGeneration") < kindNumber(kindMap(dep, "metadata"), "generation") {
			return false
		}
		for _, field := range []string{"replicas", "updatedReplicas", "availableReplicas", "readyReplicas"} {
			if kindNumber(depStatus, field) != replicas {
				return false
			}
		}
		pods := e.get("pods", namespace, "")
		items, ok := pods["items"].([]any)
		if !ok || len(items) != replicas {
			return false
		}
		for _, item := range items {
			pod := item.(map[string]any)
			podStatus := kindMap(pod, "status")
			containers, ok := podStatus["containerStatuses"].([]any)
			if podStatus["phase"] != "Running" || !ok || len(containers) == 0 {
				return false
			}
			for _, container := range containers {
				if container.(map[string]any)["ready"] != true {
					return false
				}
			}
		}
		return true
	})
}

func (e *kindEnvironment) get(resource, namespace, name string) map[string]any {
	args := []string{"--kubeconfig", e.kubeconfig, "--request-timeout=10s", "-n", namespace, "get", resource}
	if name != "" {
		args = append(args, name, "--ignore-not-found")
	}
	args = append(args, "-o", "json")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "kubectl", args...).Output()
	if err != nil {
		e.t.Logf("query %s/%s: %v", resource, name, err)
		return nil
	}
	var object map[string]any
	if len(output) != 0 {
		if err := json.Unmarshal(output, &object); err != nil {
			e.t.Fatalf("decode %s: %v", resource, err)
		}
	}
	return object
}

func (e *kindEnvironment) wait(description string, check func() bool) {
	e.t.Helper()
	deadline := time.Now().Add(150 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(time.Second)
	}
	e.t.Fatalf("timed out waiting for %s", description)
}

func (e *kindEnvironment) diagnostics() {
	for _, args := range [][]string{
		{"get", "pods,deployments,services", "-A", "-o", "wide"},
		{"get", "events", "-A", "--sort-by=.lastTimestamp"},
		{"-n", "kubiad-system", "logs", "deployment/kubiad-controller-manager", "--all-containers", "--tail=100"},
		{"-n", "kubiad-smoke", "get", "deployments,pods", "-o", "yaml"},
		{"-n", "kubiad-smoke", "get", e.primary, "-o", "yaml"},
	} {
		kindDiagnosticCommand(e.t, "kubectl", append([]string{"--kubeconfig", e.kubeconfig, "--request-timeout=10s"}, args...)...)
	}
}

func kindMap(object map[string]any, key string) map[string]any {
	value, _ := object[key].(map[string]any)
	return value
}

func kindNumber(object map[string]any, key string) int {
	value, _ := object[key].(float64)
	return int(value)
}

func kindCommand(t *testing.T, directory string, input []byte, executable string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = directory
	command.Stdin = strings.NewReader(string(input))
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", executable, strings.Join(args, " "), err, output)
	}
	if executable == "docker" && len(output) > 3000 {
		output = append([]byte("... build output truncated ...\n"), output[len(output)-3000:]...)
	}
	t.Logf("%s %s\n%s", executable, strings.Join(args, " "), output)
	return string(output)
}

func kindCleanupCommand(t *testing.T, executable string, args ...string) {
	t.Helper()
	if !kindDiagnosticCommand(t, executable, args...) {
		t.Errorf("cleanup failed: %s %s", executable, strings.Join(args, " "))
	}
}

func kindDiagnosticCommand(t *testing.T, executable string, args ...string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	output, err := exec.CommandContext(ctx, executable, args...).CombinedOutput()
	t.Logf("%s %s: %v\n%s", executable, strings.Join(args, " "), err, output)
	return err == nil
}

func writeKindFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const kindWorkloadDockerfile = `FROM golang:1.26 AS builder
WORKDIR /workspace
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o /workload main.go
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /workload /workload
USER 65532:65532
ENTRYPOINT ["/workload"]
`

const kindWorkloadSource = `package main

import (
    "fmt"
    "io"
    "net/http"
    "os"
    "time"
)

func main() {
    if len(os.Args) == 3 && os.Args[1] == "--check" {
        client := &http.Client{Timeout: 3*time.Second}
        for attempt := 0; attempt < 20; attempt++ {
            response, err := client.Get(os.Args[2])
            if err == nil {
                body, readErr := io.ReadAll(response.Body)
                response.Body.Close()
                if readErr == nil && response.StatusCode == http.StatusOK && string(body) == "kubiad-smoke\n" {
                    fmt.Print(string(body))
                    return
                }
            }
            time.Sleep(time.Second)
        }
        fmt.Fprintln(os.Stderr, "Service did not return the smoke response")
        os.Exit(1)
    }
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, "kubiad-smoke") })
    if err := http.ListenAndServe(":8080", nil); err != nil { panic(err) }
}
`
