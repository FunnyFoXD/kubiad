# CI and evaluation evidence

## Run locally

From the repository root, select one independent suite:

```sh
go run ./cmd/kubiad-evaluate --suite unit
env KUBIAD_ENVTEST_DOWNLOAD=1 go run ./cmd/kubiad-evaluate --suite envtest
env KUBIAD_KIND_TEST=1 go run ./cmd/kubiad-evaluate --suite kind
```

These commands work in POSIX shells and fish. Unit checks need Go; envtest
needs its API-server assets; kind needs Docker, kind and kubectl. See
[API-level integration setup](integration-tests.md) and
[cluster prerequisites](kind-tests.md).

By default, each invocation creates a new timestamped directory under
`reports/`, which is ignored by Git. Use `--output reports/my-run` for an
explicit directory. An existing output directory is rejected so that old
successful evidence cannot survive a failed rerun under the same name.
The tool does not enable envtest downloads or cluster creation implicitly.

| Suite | Commands and required completion |
| --- | --- |
| unit | Uncached ordinary tests, including compiler, source corpus, goldens, generated project checks and all baseline module checks; then vet with both integration build tags and root build |
| envtest | All three generated operators and three Kubebuilder baselines must pass the common API-level contract |
| kind | All three generated operators must pass the disposable-cluster smoke suite |

An exit status of zero is insufficient by itself: the reporter also requires
the named suite profiles to appear as passed in the Go event stream. Unit
reports additionally require both halves of every executable corpus pair.
An absent or skipped required profile fails evaluation. The static corpus
intentionally skips its non-executable inventory entries; those skips are
not treated as static detections or as missing executable profiles.

## Files in a run

| File | Meaning |
| --- | --- |
| report.json | Versioned summary, source revision, dirty flag, UTC timestamps, host architecture, tool versions, exact command arguments, exit status, terminal Go test events, and per-case corpus results for unit runs |
| summary.md | Human-readable suite result, command durations, and separate corpus disposition counts |
| tests.jsonl | Original stdout from `go test -json`, including failed and skipped events |
| 00.log and later numbered logs | Readable test output and command stderr; later logs cover vet and build for unit runs |
| corpus-manifest.json | Exact versioned input manifest used by the report, also identified by SHA-256 in report.json |

The reporter writes an initial **incomplete** report before executing checks,
updates it between commands, and writes a final pass/fail result when it can
finish. It preserves raw output on failure and returns a nonzero exit code.
A forcibly killed process can leave an incomplete report; that is not a pass.
Malformed JSON and failing subprocesses are errors even if earlier tests
passed. Setup failures before the reporter starts can produce no report.

Git metadata describes the checked-out source. A local dirty run explicitly
records that its modifications are not represented by the revision alone.
Use a clean checkout when archiving evidence for the thesis. In pull-request
CI, the recorded revision can be GitHub's synthetic merge commit; the CI run
URL identifies the corresponding execution.

The test event list records the root Go processes. Child-module tests run
inside generator and baseline harnesses appear in their readable output,
not as additional independent root-level events. Do not sum all parent and
nested events as a count of research cases.

## GitHub Actions

`.github/workflows/evaluation.yml` runs on branch pushes, pull requests into
dev, and manual dispatch. Three independent jobs use Ubuntu 24.04 and Go
1.26.1. The cluster job installs kind v0.31.0 and checksum-verifies kubectl
v1.35.0; Kubernetes API assets and the default kind node use v1.35.0. Official
checkout/setup/upload actions are fixed by commit SHA. The unit job also
validates workflow syntax with actionlint v1.7.11.

Jobs have read-only repository permissions, do not need repository secrets,
and do not publish images or deploy into an external cluster. Matrix failures
do not stop other suites. A newer run can cancel an older run for the same
branch or pull request. Push and pull-request events can both run for an open
PR; they record distinct executions rather than being combined.

Open a workflow run in the repository's Actions tab:

1. Read each job's Summary for its result
2. Download `evaluation-unit-*`, `evaluation-envtest-*`, and
   `evaluation-kind-*` from Artifacts
3. Keep the three bundles together with their run URL and revision

Summary publication and artifact upload are attempted even after failure.
Artifacts are retained for 30 days; archive important runs separately before
expiry. Missing artifacts after setup failure or cancellation require reading
the job logs and must not be interpreted as successful evaluation.

## Interpretation for the thesis

The current research inventory contains 43 cases: 16 semantic, 7 structural,
7 not expressible, 6 integration, 4 cluster, and 3 out of scope. The 19 extra
frontend regression pairs are reported as a separate group. These are
manifest classifications, not findings inferred from the number of tests.

For an executable pair, an invalid result of pass means the test reproduced
the expected diagnostic phase, text, line and column. Its valid counterpart
must also parse and produce valid IR. Structural rejection is distinct from
semantic analysis; integration and cluster suites do not increase either
static count. No aggregate error-detection percentage is computed.

The envtest report supports the shared six-implementation API contract. The
kind report supports the generated-operator deployment smoke contract. Their
results do not establish arbitrary-trace equivalence, exhaustive least
privilege, formal convergence, or complete crash/stale-cache fault coverage.

Recorded seconds describe one run with existing Go/Docker build caches,
downloads and cluster setup. They are not compiler-speed or reconciliation
benchmarks. Fixed tool versions and revision metadata make runs traceable;
the hosted runner image and generated container base-image tags can change.
This is not a fully hermetic build or a completed comparative experiment.

## Implementation references

The runner and report format are project-specific implementations. The
approach uses these primary references (checked 15 November 2026):

- [Go test2json format](https://pkg.go.dev/cmd/test2json): structured actions,
  package/test identities, elapsed seconds, and output records
- [GitHub workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax):
  matrix jobs, failure behavior, concurrency, event triggers and permissions
- [GitHub job summaries](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands#adding-a-job-summary):
  publishing Markdown through GITHUB_STEP_SUMMARY
- [Official upload-artifact action](https://github.com/actions/upload-artifact):
  evidence bundles, missing-file behavior and retention
- [Official setup-go action](https://github.com/actions/setup-go): exact Go
  version selection and dependency/build caching
- [actionlint](https://github.com/rhysd/actionlint): workflow validation
- [kubectl installation and checksum validation](https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/):
  verifying the downloaded client before installation
