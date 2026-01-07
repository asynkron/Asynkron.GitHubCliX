# Asynkron.GitHubCliX

A thin wrapper around GitHub CLI (`gh`) with extra visualizations.

Features
- Pass-through to `gh` for all commands
- `ghx issue tree` renders parent-child issue trees
- Flags: `--open`, `--closed`, `--root <title|#id|id>`
- Styled output using charmbracelet/lipgloss

Install
- From source: `go install ./cmd/ghx`

Usage
- `ghx issue tree`
- `ghx issue tree --open --closed`
- `ghx issue tree --root "Task 1"`
- `ghx issue tree --root 399`

Notes
- Parent relationship parsed from issue body line: `Parent: #<number>`

Updated: 2026-01-07T10:53:41.763Z

## Examples (2026-01-07T11:09:39.491Z)

Command: ghx issue tree

text
├── ○ 364 Epic: Design: IR-only execution + IR→IL compilation backend
├── ○ 399 Task 1: Remove statement-level AST delegation
│   ├── ○ 404 Task 1.1: IR support for block statements with lexical bindings
│   ├── ○ 405 Task 1.2: IR support for for-in loops
│   └── ○ 410 Task 1.7: IR support for yield in binding target defaults
├── ○ 400 Task 2: Introduce expression bytecode
│   ├── ○ 411 Task 2.1: Design expression bytecode format
│   ├── ○ 412 Task 2.2: Expression bytecode emitter
│   ├── ○ 413 Task 2.3: Expression bytecode interpreter
│   └── ○ 414 Task 2.4: Replace ExpressionNode operands with bytecode in IR instructions
├── ○ 401 Task 3: Remove / quarantine AST evaluators
├── ○ 402 Task 4: IL backend for sync bytecode
├── ○ 403 Task 5: IL backend for generator/async stepping
├── ○ 426 Roadmap
├── ○ 441 Task 0: Profiler: Inlined JIT analysis
└── ○ 448 Investigate flaky for-of typedarray resizable buffer tests


Command: ghx issue tree --open --closed

text
├── ● 48 Implement dynamic import() (ES dynamic module loading)
├── ● 49 Add async iteration support (for await...of loops)
├── ● 50 Implement BigInt primitive and operator support
├── ● 51 Implement JavaScript standard error types (TypeError, RangeError, ReferenceError, SyntaxError)
├── ● 52 Finish Object property descriptor helpers (defineProperty/getOwnPropertyDescriptor/Names/hasOwn)
├── ● 53 Add logical assignment operators (&&=, ||=, ??=) and nullish assignment
├── ● 54 Implement Proxy and Reflect for metaprogramming
├── ● 55 Object rest/spread for objects and destructuring
├── ● 56 Implement Typed Arrays and ArrayBuffer/DataView
├── ● 344 Swarm: Workers investigating shared slot assignment issue
├── ● 351 Slot Assignment and Environment Pooling Fix Plan
├── ● 357 Scope Fix
├── ● 359 Optimization: Flat slot indexing for static lexical scopes
├── ● 360 ● Bug: nested function declaration with let shadowing returns wrong value
├── ● 361 Unify sync execution on IR; prep IR→IL JIT backend
├── ● 362 ● Bug: ShadowingInFunctionInsideBlock
├── ● 363 ● Bug: ClassFieldInitializerCanAccessSuper
├── ○ 364 Epic: Design: IR-only execution + IR→IL compilation backend
├── ● 365 Epic: Refactor: De-monolith ExecutionPlanRunner (perf-safe cleanup)
├── ● 366 Task 1: Extract ExecutionPlanRunner profiling helpers
├── ● 368 Task 2: Split instruction handlers into partial file
├── ● 372 ● bug: IR execution path breaks private field support
├── ● 374 Task 3: Split ExecutePlan/ExecuteInstructionLoop into partial file
├── ● 375 Task 4: Split environment/context setup into partial file
├── ● 376 Task 5: Split constructors + entrypoints into partial file
├── ● 378 Task 6: Split flat-slot plumbing into partial file
├── ● 380 Task 7: Split handlers - Control flow
├── ● 382 Task 8: Split handlers - Statements
├── ● 383 Task 9: Split handlers - Declarations
├── ● 385 Task 10: Split handlers - Scope
├── ● 388 Task 11: Split handlers - Try/Catch/Finally
├── ● 389 Task 12: Split handlers - Iterators
├── ● 390 Task 13: Split handlers - Generators
├── ● 391 Task 14: Split handlers - Operators
├── ● 398 Task 0: Inventory + invariants
│   ├── ● 415 Task 0.1: Add AST-free assertion guard
│   └── ● 416 Task 0.2: Inventory of all StatementInstruction usages
├── ○ 399 Task 1: Remove statement-level AST delegation
│   ├── ○ 404 Task 1.1: IR support for block statements with lexical bindings
│   ├── ○ 405 Task 1.2: IR support for for-in loops
│   ├── ● 406 Task 1.3: IR support for with statements (full coverage)
│   ├── ● 407 P1.4: IR support for complex variable declarations (destructuring)
│   ├── ● 408 P1.5: IR support for class declarations with await
│   ├── ● 409 P1.6: IR support for expression statements with await
│   └── ○ 410 Task 1.7: IR support for yield in binding target defaults
├── ○ 400 Task 2: Introduce expression bytecode
│   ├── ○ 411 Task 2.1: Design expression bytecode format
│   ├── ○ 412 Task 2.2: Expression bytecode emitter
│   ├── ○ 413 Task 2.3: Expression bytecode interpreter
│   └── ○ 414 Task 2.4: Replace ExpressionNode operands with bytecode in IR instructions
├── ○ 401 Task 3: Remove / quarantine AST evaluators
├── ○ 402 Task 4: IL backend for sync bytecode
├── ○ 403 Task 5: IL backend for generator/async stepping
├── ● 420 ● Bug: block-decl-onlystrict.js - strict mode block function scoping
├── ● 421 ● Bug: switch-case-decl-onlystrict.js - strict mode switch case function scoping
├── ● 423 ● Bug: switch-dflt-decl-onlystrict.js - strict mode switch default function scoping
├── ○ 426 Roadmap
├── ● 427 Add Test262 grouped runs to CI
├── ● 428 Remove GitHub Pages deployment
├── ● 432 ● Bug: Exception handling broken when block environment combined with for await...of
├── ● 433 Task 1.2.1: Implement ForInEmitter with property enumeration instructions
├── ● 434 Task 1.7.1: Implement yield lowering for binding pattern defaults
├── ● 438 ● Bug: IR path doesn't hoist block-scoped function declarations
├── ● 440 ● Bug: IR for-await-of loop environments corrupt after PR #437
├── ○ 441 Task 0: Profiler: Inlined JIT analysis
├── ● 443 Fix nested catch parameter shadowing in IR
├── ● 446 Fix build errors from C# 14 extension blocks
└── ○ 448 Investigate flaky for-of typedarray resizable buffer tests


Command: ghx issue tree -root 399

text
└── ○ 399 Task 1: Remove statement-level AST delegation
    ├── ○ 404 Task 1.1: IR support for block statements with lexical bindings
    ├── ○ 405 Task 1.2: IR support for for-in loops
    └── ○ 410 Task 1.7: IR support for yield in binding target defaults


Command: ghx issue tree -root "AST delegation"

text
└── ○ 399 Task 1: Remove statement-level AST delegation
    ├── ○ 404 Task 1.1: IR support for block statements with lexical bindings
    ├── ○ 405 Task 1.2: IR support for for-in loops
    └── ○ 410 Task 1.7: IR support for yield in binding target defaults

