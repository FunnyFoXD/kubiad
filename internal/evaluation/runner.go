package evaluation

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Run executes one suite and preserves both raw evidence and a bounded summary
func Run(ctx context.Context, suite, directory string, console io.Writer) (runErr error) {
	commands, err := suiteCommands(suite)
	if err != nil {
		return err
	}
	if data, err := os.ReadFile("go.mod"); err != nil || !strings.Contains(string(data), "module "+module+"\n") {
		return fmt.Errorf("run evaluation from the Kubiad repository root")
	}
	if err := os.MkdirAll(filepath.Dir(directory), 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(directory, 0o755); err != nil {
		return fmt.Errorf("report directory must be new: %w", err)
	}
	report := Report{Version: 1, Suite: suite, Status: "incomplete", Started: time.Now().UTC(), Platform: runtime.GOOS + "/" + runtime.GOARCH, Tools: map[string]string{"reporter_go": runtime.Version()}}
	for _, args := range commands {
		report.Commands = append(report.Commands, CommandResult{Args: args, Status: "not_run", ExitCode: -1})
	}
	defer func() {
		finished := time.Now().UTC()
		report.Finished = &finished
		report.Status = "pass"
		if runErr != nil {
			report.Status, report.Error = "fail", runErr.Error()
		}
		if err := writeReport(directory, report); err != nil {
			runErr = errors.Join(runErr, err)
		}
		fmt.Fprintf(console, "Evaluation %s: %s (%s)\n", suite, report.Status, directory)
	}()
	if err := writeReport(directory, report); err != nil {
		return err
	}
	if report.Revision, err = toolOutput(ctx, "git", "rev-parse", "HEAD"); err != nil {
		return err
	}
	status, err := toolOutput(ctx, "git", "status", "--porcelain")
	if err != nil {
		return err
	}
	report.Dirty = status != ""
	if os.Getenv("GITHUB_RUN_ID") != "" {
		report.CIURL = os.Getenv("GITHUB_SERVER_URL") + "/" + os.Getenv("GITHUB_REPOSITORY") + "/actions/runs/" + os.Getenv("GITHUB_RUN_ID")
	}
	data, err := os.ReadFile(filepath.Join("testdata", "analysis", "manifest.json"))
	if err != nil {
		return err
	}
	report.ManifestHash = fmt.Sprintf("%x", sha256.Sum256(data))
	if err := os.WriteFile(filepath.Join(directory, "corpus-manifest.json"), data, 0o644); err != nil {
		return err
	}
	for name, args := range map[string][]string{
		"go":      {"go", "version"},
		"kind":    {"kind", "version"},
		"kubectl": {"kubectl", "version", "--client", "-o", "json"},
		"docker":  {"docker", "version", "--format", "{{.Server.Version}}"},
	} {
		if suite != "kind" && name != "go" {
			continue
		}
		value, err := toolOutput(ctx, args[0], args[1:]...)
		if err != nil {
			value = "unavailable: " + err.Error()
		}
		report.Tools[name] = value
	}
	if suite == "kind" {
		report.Tools["node_image"] = os.Getenv("KUBIAD_KIND_NODE_IMAGE")
		if report.Tools["node_image"] == "" {
			report.Tools["node_image"] = "kindest/node:v1.35.0"
		}
	}
	if suite == "envtest" {
		report.Tools["envtest_kubernetes"] = "1.35.0"
		if os.Getenv("KUBEBUILDER_ASSETS") != "" {
			report.Tools["envtest_kubernetes"] = "external assets; verify binary versions separately"
		}
	}
	for index := range report.Commands {
		if ctx.Err() != nil {
			return errors.Join(runErr, ctx.Err())
		}
		result := &report.Commands[index]
		fmt.Fprintf(console, "Running %s\n", strings.Join(result.Args, " "))
		commandErr := runCommand(ctx, directory, index, result, console)
		if index == 0 && commandErr == nil {
			commandErr = checkResults(suite, result.Tests)
		}
		if index == 0 && suite == "unit" {
			report.Corpus, err = corpusResults(data, result.Tests)
			commandErr = errors.Join(commandErr, err)
			for _, item := range report.Corpus {
				if item.Invalid != "" && (item.Invalid != "pass" || item.Valid != "pass") {
					commandErr = errors.Join(commandErr, fmt.Errorf("corpus pair %s did not pass", item.ID))
				}
			}
		}
		result.Status = "pass"
		if commandErr != nil {
			result.Status, result.Error = "fail", commandErr.Error()
			runErr = errors.Join(runErr, commandErr)
		}
		if err := writeReport(directory, report); err != nil {
			return errors.Join(runErr, err)
		}
	}
	return runErr
}

func suiteCommands(suite string) ([][]string, error) {
	switch suite {
	case "unit":
		return [][]string{
			{"go", "test", "-json", "./...", "-count=1", "-timeout=15m"},
			{"go", "vet", "-tags=kind,envtest", "./..."},
			{"go", "build", "./..."},
		}, nil
	case "envtest":
		return [][]string{{"go", "test", "-json", "-tags=envtest", "./internal/generator", "-run", "^(TestGeneratedEnvtest|TestKubebuilderEnvtest)$", "-count=1", "-timeout=15m"}}, nil
	case "kind":
		return [][]string{{"go", "test", "-json", "-tags=kind", "./internal/generator", "-run", "^TestGeneratedKind$", "-count=1", "-timeout=35m"}}, nil
	default:
		return nil, fmt.Errorf("unknown suite %q; choose unit, envtest, or kind", suite)
	}
}

func runCommand(ctx context.Context, directory string, index int, result *CommandResult, console io.Writer) error {
	started := time.Now()
	defer func() { result.Elapsed = time.Since(started).Seconds() }()
	log, err := os.Create(filepath.Join(directory, fmt.Sprintf("%02d.log", index)))
	if err != nil {
		return err
	}
	defer log.Close()
	command := exec.CommandContext(ctx, result.Args[0], result.Args[1:]...)
	command.Stderr = log
	var parseErr error
	if len(result.Args) > 2 && result.Args[1] == "test" {
		raw, rawErr := os.Create(filepath.Join(directory, "tests.jsonl"))
		if rawErr != nil {
			return rawErr
		}
		defer raw.Close()
		pipe, pipeErr := command.StdoutPipe()
		if pipeErr != nil {
			return pipeErr
		}
		if err := command.Start(); err != nil {
			return err
		}
		reader := io.TeeReader(pipe, raw)
		result.Tests, parseErr = readEvents(reader, io.MultiWriter(log, console))
		if parseErr != nil {
			io.Copy(io.Discard, reader)
		}
		err = command.Wait()
	} else {
		command.Stdout = io.MultiWriter(log, console)
		err = command.Run()
	}
	if command.ProcessState != nil {
		result.ExitCode = command.ProcessState.ExitCode()
	}
	return errors.Join(err, parseErr)
}

func toolOutput(ctx context.Context, executable string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, executable, args...).CombinedOutput()
	return strings.TrimSpace(string(output)), err
}
