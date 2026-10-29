# Go port differential code review

Reviewed 2026-10-29 at source revision `9e80e40f9052ee36d5749c8f633f4a601c44cfe0`.

This replaces the previous report. Its six findings were already fixed; their regression tests remain in the core, CLI, and Shai packages. The **14 new findings below are not fixed** by this audit. P1 denotes incorrect model-checking results; P2 denotes a functional or compatibility defect.

## Executed validation

Go was rebuilt with the repository's pinned dependencies using `go build -mod=readonly -p=8 -o build/apalache ./cmd/apalache`. The initially available Scala distribution was older than the checkout. Consequently, `sbt -batch 'Universal/stage'` rebuilt Scala, and all reported comparisons were repeated against `target/universal/stage/bin/apalache-mc`. Commands were CPU-capped with `taskset -c 1-8`.

| Validation | Result |
| --- | --- |
| Live Scala/Go oracle test selection | PASS, 258.054 seconds, no skips |
| Safety differential manifest | All 50 cases passed, including nine temporal cases |
| Additional oracle checks | Exact frontend diagnostics, three ReTLA/VMT comparisons, undefined VMT functions, and two documented nonlinear-underapproximation cases passed |
| Scala fixture corpus through Go frontend | PASS, 12.383 seconds: 267 syntax fixtures and 227 annotated fixtures, with expected negative cases; PolyExp deliberately excluded |
| Additional CLI probes | 29 cases run against both implementations; failures and divergences retained |
| JSON-RPC and Shai probes | Real loopback HTTP/gRPC servers for both implementations; results retained |
| Malformed fully typed JSON | Go incorrectly accepts; rebuilt Scala rejects |

The corpus test is **not** a live Scala differential comparison. The manifest passing does not negate the new findings: its cases miss the triggers below. The full Go regression/race suite was not rerun for this audit; no production implementation was changed.

Reproducible drivers, input, and recorded results: [testdata/code_review](testdata/code_review/README.md). These are diagnostic probes, not passing regression tests. They record both implementations' behavior, including disagreements.

Unless otherwise stated, CLI reproducers replace `Inv` in this complete module and run `check --length=0 --inv=Inv Audit.tla`:

```tla
---- MODULE Audit ----
EXTENDS Integers, Sequences, FiniteSets, Apalache
VARIABLE
\* @type: Int;
x
Init == x = 2
Next == x' = x
Inv == TRUE
====
```

“Success” below means no counterexample at the requested bound, not an unbounded proof.

## 1. [P1] Parser rewiring changes unrelated user-defined operators

**Go:** [frontend_tlago_compat.go](frontend_tlago_compat.go), `normalizeTlagoTLCCalls` (1604), `removeTlagoNativeVariantDeclarations` (1582), `normalizeTlagoLambdaExpr` (1004).

**Scala:** [ModuleTranslator.scala](../tla-parser/src/main/scala/at/forsyte/apalache/tla/imp/ModuleTranslator.scala), `addAliasesOfBuiltins`; [OpApplTranslator.scala](../tla-parser/src/main/scala/at/forsyte/apalache/tla/imp/OpApplTranslator.scala), library-origin lookup and user-operator translation.

The TLC adapter strips instance prefixes and rewrites calls by their final spelling whenever TLC is imported anywhere in the spec. It also removes declarations by that spelling. Add these definitions before VARIABLE and replace Inv:

```tla
T == INSTANCE TLC
Print(a,b) == a
Inv == Print(FALSE, TRUE)
```

Scala reports the violation (exit 12); Go reports success (exit 0). The user operator returns FALSE; Go replaces the call with its second argument, TRUE.

Two other confirmed paths have the same resolution problem. With `V == INSTANCE Variants`, root definition `Variant(a,b) == a+b` is removed and `Inv == Variant(1,2)=3` fails typing in Go. A user definition `a @@ b == a-b` is unconditionally lowered as function override, so `(3 @@ 2)=1` also fails typing. Scala accepts both.

**Fix direction:** Identify the resolved declaration and original module before replacing calls or deleting definitions. Cover root, instance-qualified, and local/bound operator names.

## 2. [P1] Compact symbolic ranges are consumed as empty enumerations

**Go:** [integer_range.go](integer_range.go), `rewriteIntegerRange`; [stratified_rewriter.go](stratified_rewriter.go), `rewriteFunctionConstructor` (552) and `rewriteSetUnion` (2582).

**Scala:** [IntDotDotRule.scala](../tla-bmcmt/src/main/scala/at/forsyte/apalache/tla/bmcmt/rules/IntDotDotRule.scala), `getRange`; [FunCtorRule.scala](../tla-bmcmt/src/main/scala/at/forsyte/apalache/tla/bmcmt/rules/FunCtorRule.scala); [SetCupRule.scala](../tla-bmcmt/src/main/scala/at/forsyte/apalache/tla/bmcmt/rules/SetCupRule.scala).

Go represents nonconstant `1..x` using bounds and no enumerated Has edges. Function construction and binary union iterate those edges without rejecting or materializing the compact representation. They silently construct an empty function relation or discard the range from the union.

With `Init == x=2`:

| Invariant | Scala | Go |
| --- | --- | --- |
| `[i \in 1..x \|-> i][1] = 1` | rejects unsupported nonconstant range | false violation, exit 12 |
| `[i \in 1..x \|-> i][1] = 0` | rejects unsupported nonconstant range | false success, exit 0 |
| `((1..x) \cup {3}) = {1,2,3}` | rejects unsupported nonconstant range | false violation, exit 12 |
| `((1..x) \cup {3}) = {3}` | rejects unsupported nonconstant range | false success, exit 0 |

The correct mathematical values follow directly from `x=2`; Scala's rejection is not itself the correctness oracle. The bug is accepting the expressions with incorrect meanings.

**Fix direction:** Reject compact operands at unsupported enumeration boundaries, or materialize them soundly. Audit every GetHas consumer by cell representation, not just its TLA Set type. Checks in set-map, filter, and unary-union paths do not protect these paths.

## 3. [P1] Legacy record equality includes fields outside the actual domain

**Go:** [lazy_equality.go](lazy_equality.go), `recordEqualityCondition` (349); [except.go](except.go), `rewriteRecordExcept`.

**Scala:** [LazyEquality.scala](../tla-bmcmt/src/main/scala/at/forsyte/apalache/tla/bmcmt/LazyEquality.scala), `mkRecordEq`.

Go compares every field of the static legacy record type unconditionally, then compares domains. Scala guards each field comparison with actual domain membership. Imprecise typing can allocate storage for fields that the value does not contain.

With `--features=no-rows`:

```tla
Inv == LET r == IF x = 2 THEN [a |-> 1] ELSE [a |-> 1, b |-> 2]
       IN [r EXCEPT !.b = 3] = r
```

Scala succeeds; Go reports a violation. Here r has domain `{"a"}`, so EXCEPT on nonexistent field b preserves the record. Both implementations retain that domain, but Go's equality observes the irrelevant changed storage slot.

**Fix direction:** Guard field-value equality by actual membership while retaining domain equality. Cover absent fields after EXCEPT, conditionals, widening, and record-set membership.

## 4. [P2] Imported LET type annotations collide by unqualified name

**Go:** [frontend_tlago_annotations.go](frontend_tlago_annotations.go), `parseTlagoModuleAnnotations` (121), `mergeTlagoImportedAnnotations` (299).

**Scala:** [OpDefTranslator.scala](../tla-parser/src/main/scala/at/forsyte/apalache/tla/imp/OpDefTranslator.scala), declaration-ID annotation storage.

Imported annotation collection keys nested declarations by simple name. Two separate LET scopes may legally use the same name with different types. The second annotation overwrites the first and is applied to both declarations.

Create Helper.tla:

```tla
---- MODULE Helper ----
Foo == LET (* @type: Int; *) Local == 1 IN Local
Bar == LET (* @type: Str; *) Local == "s" IN Local
====
```

Add Helper to Audit's EXTENDS and use `Inv == Foo=1 /\ Bar="s"`. Scala succeeds; Go exits 4, expecting the integer Local to have type `() => Str`.

**Fix direction:** Carry declaration identity, source location, and lexical/instance scope through annotation import.

## 5. [P2] Fully tagged JSON bypasses type inference and validation

**Go:** [frontend.go](frontend.go), `ParseAndTypeCheckFile` (385), early return after TypedLanguagePredicate.

**Scala:** [EtcTypeCheckerPassImpl.scala](../passes/src/main/scala/at/forsyte/apalache/tla/passes/typecheck/EtcTypeCheckerPassImpl.scala), `execute`; [TypeCheckerTool.scala](../passes/src/main/scala/at/forsyte/apalache/tla/typecheck/TypeCheckerTool.scala), `check`.

For non-TLA input, Go treats the presence of type tags on every node as proof of consistency, marks the module TypeChecked, and skips inference. The predicate checks coverage, not correctness.

Run `typecheck testdata/code_review/BadTypedIR.json` from pkg. It encodes `Bad == 1 + TRUE`, with Int/Bool operand tags and Int on PLUS. Scala rejects the arguments (exit 120). Go prints `type-checked module BadTypedIR` and exits 0.

**Fix direction:** Validate typed input through inference, using existing tags as constraints where appropriate. Check JSON and typed Quint entry points; keep type coverage separate from successful inference.

## 6. [P2] Nested function defaults reject valid sequence operations

**Go:** [default_value.go](default_value.go), `DefaultValueCache.GetOrCreate`, `defaultValueExpression` (101).

**Scala:** [DefaultValueFactory.scala](../tla-bmcmt/src/main/scala/at/forsyte/apalache/tla/bmcmt/rules/support/DefaultValueFactory.scala), recursive `makeUpValue`; [SeqOpsRule.scala](../tla-bmcmt/src/main/scala/at/forsyte/apalache/tla/bmcmt/rules/SeqOpsRule.scala).

The cache supports a function type directly, but tuple/row-record/variant defaults recurse through defaultValueExpression, which has no function case. Valid operations needing internal padding/default values fail on nested functions:

```tla
Inv == SubSeq(<< <<[i \in {1} |-> i], 2>> >>, 1, x)[1][2] = 2
```

Scala succeeds; Go exits 1 with `default values are not yet implemented for (Int -> Int)`. Concatenating this SubSeq with another singleton of the same tuple type also fails. The examples access only in-range elements.

**Fix direction:** Use a recursive, arena-aware default factory for nested types. Cover tuples, row records, variants, and sequences containing functions.

## 7. [P2] Gen creates legacy records without domain edges

**Go:** [value_generator.go](value_generator.go), RecordType branch and `generateProduct` (137).

**Scala:** [ValueGenerator.scala](../tla-bmcmt/src/main/scala/at/forsyte/apalache/tla/bmcmt/rules/support/ValueGenerator.scala), `genRecord` (71).

Go routes legacy records through a generic product constructor that only appends field edges. It omits the domain needed by legacy-record EXCEPT, equality, and decoding. Scala attaches the domain from the type's keys.

With `--features=no-rows`:

```tla
Inv == LET (* @type: [a: Int]; *) R == Gen(1)
       IN [R EXCEPT !.a = 0] = [a |-> 0]
```

Scala succeeds; Go exits 1 with `cell ... has no domain edge`. Testing DOMAIN alone misses this because it can be optimized from the type.

**Fix direction:** Attach a legacy record domain in Gen, including records nested inside larger generated values.

## 8. [P2] JSON-RPC query and compact lose a pending destination after an assertion

**Go:** [exploration_service.go](exploration_service.go), `query` (465), `compact` (604), and operation logging.

**Scala:** [JsonRpcServer.scala](../json-rpc/src/main/scala/com/github/apalachemc/apalache/jsonrpc/JsonRpcServer.scala), query/compact decoding of the executor's current execution.

Go calls CheckPendingAndDecode only when the last logged operation is a transition. A state assertion changes that last entry without clearing the executor's pending transition; decoding then drops the destination.

Live sequence for `Init == x=0`, `Next == x'=x+1`:

1. Load; assume Init with enabledness checking; nextStep.
2. Assume Next; query STATE/TRACE: both return `[0,1]`, state 1.
3. `assumeState(equalities={"x":0}, checkEnabled=true)`: both return ENABLED.
4. Query: Scala returns `[0,1]`, state 1; Go returns `[0]`, state 0.
5. `compact(snapshotId=0)` and query: Scala keeps singleton state 1; Go keeps singleton state 0.

This is persistent state loss through compaction, not just trace formatting.

**Fix direction:** Derive pending-state presence from executor state. Cover assertions, model exclusions, recovery, and batched calls before query/compact.

## 9. [P2] Remote type checking defaults to the wrong record mode

**Go:** [shai/cmd_executor_service.go](shai/cmd_executor_service.go), `remoteTypecheckOptions` (361); [exploration_service.go](exploration_service.go), type options in `loadExplorationSource`.

**Scala:** [EtcTypeCheckerPassImpl.scala](../passes/src/main/scala/at/forsyte/apalache/tla/passes/typecheck/EtcTypeCheckerPassImpl.scala), rows enabled unless no-rows is selected.

Shai initializes zero-value TypeCheckOptions and enables rows only for explicit `features:["rows"]`; it also never populates NoRows. JSON-RPC source loading likewise supplies zero-value options.

Live Shai TYPECHECK of `R == {[a |-> 1], [b |-> 2]}` without features: Scala fails TypeCheckerSnowcat; Go succeeds with legacy unification. Explicit rows makes both reject it. The JSON-RPC mismatch is established from source; the live rows probe exercised Shai.

**Fix direction:** Share CLI defaults and feature validation, including explicit no-rows annotation checks.

## 10. [P2] gRPC frontend failures have incompatible shapes and incorrect stages

**Go:** [shai/trans_explorer_service.go](shai/trans_explorer_service.go), LoadModel (69); [shai/cmd_executor_service.go](shai/cmd_executor_service.go), failure handling (240–280).

**Scala:** [TransExplorerService.scala](../shai/src/main/scala/at/forsyte/apalache/shai/v1/TransExplorerService.scala), [CmdExecutorService.scala](../shai/src/main/scala/at/forsyte/apalache/shai/v1/CmdExecutorService.scala), PassFailure serialization.

For malformed source `---- MODULE Bad ----\nO == (\n====\n`:

| RPC | Scala | Go |
| --- | --- | --- |
| LoadModel error Data | object with pass_name/error_data/exit_code | bare Diagnostic array |
| CmdExecutor PARSE | SanyParser, exit 255 | SanyParser, exit 150 |
| CmdExecutor TYPECHECK/CHECK | SanyParser, exit 255 | TypeCheckerSnowcat, exit 120 |

For actual typing errors, Scala's error_data is an array of `[location,message]` pairs; Go supplies Diagnostic objects. Clients following the Scala protocol cannot decode the results consistently, and syntax errors are misclassified as type errors.

**Fix direction:** Preserve the originating stage and its structured protocol shape at both boundaries. This is separate from exact exception text.

## 11. [P2] CmdExecutor rejects supported JSON input

**Go:** [shai/cmd_executor_service.go](shai/cmd_executor_service.go), TLA-only format guard (102).

**Scala:** [CmdExecutorService.scala](../shai/src/main/scala/at/forsyte/apalache/shai/v1/CmdExecutorService.scala), normal input resolution/pass-chain dispatch.

Feed a successful PARSE response back into PARSE with `source.format:"json"`. Scala succeeds; Go returns UNEXPECTED, `Go command executor does not support json source format`. The shared Go frontend already implements JSON and typed Quint, but the service blocks those readers.

**Fix direction:** Dispatch through the shared InputSource/frontend pipeline and add remote round-trip coverage.

## 12. [P2] CmdExecutor rejects implemented SMT encodings

**Go:** [shai/cmd_executor_service.go](shai/cmd_executor_service.go), `validateRemoteCheckerConfig` (128) and checker construction.

**Scala:** [CmdExecutorService.scala](../shai/src/main/scala/at/forsyte/apalache/shai/v1/CmdExecutorService.scala), checker configuration resolution.

A CHECK of the trivial integer/stuttering spec with `checker:{"length":0,"smt-encoding":"arrays"}` succeeds in Scala and fails as UNEXPECTED in Go. funArrays fails identically. The rejection incorrectly says the port supports only oopsla19.

**Fix direction:** Accept and propagate the implemented encodings to the solver/rewriter. Removing the guard alone does not wire the option through.

## 13. [P2] JSON-RPC returns successful null operator values on evaluation failure

**Go:** [exploration_service.go](exploration_service.go), OPERATOR branch of query.

**Scala:** [JsonRpcServer.scala](../json-rpc/src/main/scala/com/github/apalachemc/apalache/jsonrpc/JsonRpcServer.scala), getViewInJson/query.

Export `View == x`, exhaust the unique current view with `nextModel(operator="View")` (hasNext FALSE), then query `kinds:["OPERATOR"], operator:"View"`. Scala returns error -32603, `Failed to evaluate View within 0 seconds`. Go returns success with operatorValue null. Its branch assigns a value only for sat and silently leaves it nil for unsat/unknown.

Missing exported operators also get a different category: -32602 in Go versus -32603 in Scala.

**Fix direction:** Propagate failed/inconclusive evaluation as the Scala-compatible RPC error.

## 14. [P2] gRPC rejects equivalent uppercase connection UUIDs

**Go:** [shai/trans_explorer_service.go](shai/trans_explorer_service.go), UUID validation and string-keyed connection lookup (57).

**Scala:** [TransExplorerService.scala](../shai/src/main/scala/at/forsyte/apalache/shai/v1/TransExplorerService.scala), UUID.fromString and UUID-keyed connections.

Open a connection, uppercase its UUID, and call LoadModel. Scala accepts it; Go returns FailedPrecondition. Go's regex accepts uppercase hexadecimal, but lookup compares the original lowercase string literally.

**Fix direction:** Parse/canonicalize UUIDs consistently before connection lookup.

## Coverage ledger and limits

| Area | Review coverage and result |
| --- | --- |
| Parsing | tlago compatibility normalization, native export boundary, imports/instances, builtin replacement, local annotations, type annotations/aliases, bundled-library resolution, JSON/Quint entry points. Findings 1, 4, 5, 11. |
| Full type inference | ETC translation and operator signatures, context lookup, overloaded constraints, LET polymorphism, occurs checks, row/legacy/sparse-tuple unification, substitutions, annotation parsing, inferred-type rewriting, and caller defaults. Findings 4, 5, 9; no additional confirmed defect in the core ETC solver. |
| Symbolic rewriting | All main Scala rule families: constants/substitution; Boolean/ITE/LET/assignment/prime; arithmetic/comparison/ranges; finite sets, membership, inclusion, map/filter/union, cardinality, powersets; function constructor/application/domain/EXCEPT/function sets/SetAsFun; tuples/records/sequences; quantification/CHOOSE/Guess; Gen/MkSeq/folds/Repeat/variants. Also array/funArrays variants, structural equality, choice/default helpers, preprocessing, and ReTLA/VMT lowering. Findings 2, 3, 6, 7. |
| Temporal checking | Loop/saved-state encoding, tableau recurrences for always/eventually and propositional structure, lookback/unroll/loop closure, initial-state properties, action atoms, multiple/nested formulas, unsupported temporal constructs, level classification, and pipeline integration. Nine live temporal manifest cases passed; no new confirmed temporal finding. |
| Server transports | JSON-RPC dispatch/parameters, loading, transitions/assertions, snapshots/recovery/compaction, queries/model enumeration, batching, errors, HTTP compression; Shai protobuf services, connection lifecycle, source/config dispatch and error serialization. Live HTTP, gzip/zstd request and response round trips, and gRPC probes. Findings 8–14. |

This is a broad source audit with differential evidence, not a proof of equivalence for every input or combination of rules. Live tests do not exhaust infinite domains, all SMT outcomes, concurrent schedules, or malformed transport payloads. No independent audit of the entire tlago/g3/SANY/gRPC dependencies or race stress was performed. Findings are confirmed examples, not a claim that no other bugs remain.

Shared Scala behavior is not automatically reported as a Go regression. For example, the tested negative constant division/modulo behavior and empty Tail behavior agree; Scala's nonlinear underapproximation cases remain explicitly documented by their existing tests. CVC5 support, unavailable global Z3 parameters, and exact Scala exception stacks remain the port's declared exclusions.
