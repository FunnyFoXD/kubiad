# Frontend analysis corpus

`manifest.json` is the versioned inventory of 43 research scenarios and 19 additional frontend regression cases. Executable cases contain independent `.kbi` files under `invalid/` and `valid/`, with an expected diagnostic phase, message, line, and column for the invalid source. The valid counterpart checks for false rejection by the frontend; it does not promise that every accepted program fits the generator's current resource-count limits.

These are synthetic minimal examples of language rules, not verbatim copies of production bugs. Additional regression examples are kept separate from the research cases and do not increase the reported research coverage.

| Disposition | Research cases | Meaning |
| --- | --- | --- |
| `semantic` | 16 | Parsed successfully but rejected by lowering or IR validation |
| `structural` | 7 | Excluded by the grammar, not by a semantic inference rule |
| `not_expressible` | 7 | The exact scenario cannot be represented by the current language |
| `integration` | 6 | Requires generated-controller or API-server tests |
| `cluster` | 4 | Requires cluster or fault-injection tests |
| `out_of_scope` | 3 | Outside the supported ownership and side-effect model |

`P-01` and `P-07` are executable provenance cases. `P-02` through `P-06` require observations or expressions that are not supported. Rejecting an unknown property or unsupported syntax is not counted as detecting an observed-to-desired dependency. The supplemental observed-Service-port test uses `readyReplicas`; it is deliberately not labelled `P-03`, whose original scenario reads an observed port.

`B-01` through `B-03` reject runtime expressions in literal-only contexts at the syntax phase. `R-06` is not represented by a duplicate-name test: the supported Service-to-Deployment dependency graph cannot express a cycle. `R-05` requires a hyphenated source identifier, which the grammar does not allow.

Run the inventory, negative cases, and valid counterparts:

```sh
go test ./internal/parser -run 'TestAnalysisCorpusInventory|TestStaticAnalysisCorpus|TestFrontendRegressionCorpus' -v
```

Cases assigned to other testing layers or outside the language are reported as skipped with a reason. They are not successful frontend checks. Manifest validation rejects duplicate IDs, missing pairs, and unregistered `.kbi` fixtures.

Try an individual diagnostic:

```sh
go run ./cmd/kubiad check testdata/analysis/invalid/P-01.kbi
go run ./cmd/kubiad check testdata/analysis/valid/P-01.kbi
```

Run parser fuzzing:

```sh
go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2
```

Fuzzing uses the valid and invalid fixtures as seeds and checks that parsing does not panic, accepted inputs produce valid IR, and repeated parsing produces the same IR or diagnostic. A bounded fuzz run is a regression check, not a proof of correctness.

Literal Service/container ports are checked against `1..65535`, and literal replica counts must be nonnegative. These checks do not perform interval analysis over runtime spec values. Runtime spec restrictions still depend on declared CRD constraints.

## Implementation references

- [Go type-checker negative fixtures](https://go.googlesource.com/go/+/bc5aa2f7d3e1deb5468432230c438a8a9e8de316/src/internal/types/testdata/check/decls0.go) illustrate keeping invalid source examples with expected errors. Kubiad uses a separate JSON manifest and its own test runner rather than copying the Go harness or source examples
- [Official Go fuzzing documentation](https://go.dev/doc/security/fuzz/) provides the seed-corpus and `testing.F` workflow used by the fuzz target
- [Kubernetes apimachinery v0.35.0 validation](https://github.com/kubernetes/apimachinery/blob/v0.35.0/pkg/util/validation/validation.go) defines the nonzero port bounds in `IsValidPortNum`; Kubiad implements the same bounds for compile-time literals without adding a Kubernetes dependency to the frontend
