```markdown
# Cognitive Complexity — Calculation Spec

Source: G. Ann Campbell, *"Cognitive Complexity: A New Way of Measuring Understandability,"* SonarSource, v1.7 (29 Aug 2023)

This document is a precise, implementation-oriented reference for computing Cognitive Complexity (intended for a tree-sitter based analyzer).

## Core Idea

Cognitive Complexity is an additive, rule-based score that estimates the mental effort required to understand a method’s control flow.  

It deliberately abandons graph-theory mathematics (unlike Cyclomatic Complexity’s `V(G) = E − N + 2P`).  

A method starts at **0**. There is no floor of 1. This is what makes the metric meaningful when aggregated at the class, package, or application level.

## Three Basic Rules

1. **Ignore** structures that allow multiple statements to be readably shorthanded into one (+0).
2. **Increment (+1)** for each break in the linear flow of control.
3. **Increment further** when flow-breaking structures are nested.

## Four Increment Types

| Type | Definition | Effect |
|------|------------|--------|
| **A. Nesting** | Assessed when a flow-breaking structure is nested inside another | +N where N = current nesting depth |
| **B. Structural** | Control-flow structures that *both* receive a nesting increment *and* increase the nesting count | +1 (structural) + nesting penalty when nested |
| **C. Fundamental** | Statements that break linear flow but are *never* subject to a nesting increment | +1 |
| **D. Hybrid** | Structures that increase the nesting count for anything inside them but are *themselves* never subject to a nesting increment | +1 |

## Structures and Scoring

### Ignored / Shorthand (+0)

- Method / function declarations (including the outermost function)
- Null-coalescing / optional chaining operators (when present)
- `try` / `finally` equivalents (Go has none; ignored in languages that do)
- Individual `case` / `default` labels inside a `switch`
- Unlabeled `break` / `continue` (single-level)
- Early `return` (outside of loops/conditionals)

> **Important**: function literals (closures), nested functions, and method-like constructs cost **+0 structurally** but **do increase the nesting level** for everything inside them.

### Hybrid Increments (+1, never receive nesting penalty)

- `else`
- `else if` / `elif`

These raise the nesting level for their bodies but are never themselves charged a nesting increment.

### Structural Increments (+1, *and* subject to nesting)

These both receive a structural +1 **and** a nesting penalty when nested. They also raise the nesting level.

- `if` (including the condition)
- Ternary / conditional expression
- `switch` (the entire switch + all its cases counts as **one** structural increment)
- `for`, `for range`, `while`, `do-while`
- `catch` / equivalent (one +1 regardless of number of exception types)
- Preprocessor conditionals (`#if`, `#ifdef`, …) when present

### Fundamental Increments (+1, never nested)

- Sequences of binary logical operators: **+1 per sequence of identical operators**. A change of operator starts a new sequence.  
  Example: `a && b || c && d` → +3
- `goto LABEL`
- Labeled / numbered `break` / `continue` (`break LABEL`, `continue 2`, …)
- Each method that participates in a recursion cycle (direct or indirect). Treated as a “meta-loop”.

## Nesting Mechanics

**Structures that increase the nesting level**:

- All Structural structures listed above
- All Hybrid structures (`else`, `else if`)
- Nested functions / function literals / closures / method-like constructs

**Structures that receive a nesting increment** (when nested inside any of the above):

- `if`, ternary
- `switch`
- `for` / `for range` / `while` / `do-while`
- `catch`

Cost of a nested structure = **1 (structural) + N**  
where N is the nesting depth at the point the structure is entered (number of enclosing structures that increase nesting).

The outermost method body starts at nesting depth 0.

## Worked Examples (Go)

### 1. Basic nesting + logical sequence

```go
func processNumbers(numbers []int) {          // +0 (method itself)
    for _, num := range numbers {             // +1 structural (depth → 1)
        if num > 1 {                          // +1 structural +1 nesting = +2 (depth → 2)
            if isOdd(num) && isValid(num) {   // +1 structural +2 nesting = +3
                                              // +1 fundamental (&& sequence)
                fmt.Println(num)
            }
        }
    }
}
// Total = 1 + 2 + 3 + 1 = 7
```

### 2. Hybrid `else if` / `else` (no nesting penalty on them)

```go
func classify(n int) string {
    if n < 0 {                    // +1 structural
        return "negative"
    } else if n == 0 {            // +1 hybrid (no nesting penalty)
        return "zero"
    } else {                      // +1 hybrid
        if n%2 == 0 {             // +1 structural +1 nesting = +2
            return "even"
        }
        return "odd"
    }
}
// Total = 1 + 1 + 1 + 2 = 5
```

### 3. `switch` counts as a single structural increment

```go
func describe(code int) string {
    switch code {                 // +1 structural (covers all cases)
    case 200:
        return "ok"
    case 404:
        return "not found"
    case 500:
        if isRetryable(code) {    // +1 structural +1 nesting = +2
            return "retry"
        }
        return "error"
    default:
        return "unknown"
    }
}
// Total = 1 + 2 = 3
```

### 4. Function literal raises nesting level

```go
func outer(items []string) {
    process := func(s string) {   // +0 structural, but depth → 1
        if len(s) > 0 {           // +1 structural +1 nesting = +2
            for _, c := range s { // +1 structural +2 nesting = +3
                fmt.Print(c)
            }
        }
    }
    process("hello")
}
// Total = 2 + 3 = 5
```

### 5. Logical operator sequences + labeled jump

```go
func search(matrix [][]int, target int) bool {
OUTER:
    for i, row := range matrix {              // +1 structural (depth → 1)
        for j, v := range row {               // +1 structural +1 nesting = +2 (depth → 2)
            if v == target && i > 0 || j > 0 { // +1 structural +2 nesting = +3
                                               // +1 (&& sequence)
                                               // +1 (|| starts new sequence)
                break OUTER                   // +1 fundamental (labeled break)
            }
        }
    }
    return false
}
// Total = 1 + 2 + 3 + 1 + 1 + 1 = 9
```

### 6. Recursion (fundamental)

```go
func factorial(n int) int {
    if n <= 1 {                   // +1 structural
        return 1
    }
    return n * factorial(n-1)     // +1 fundamental (recursion)
}
// Total = 2
```

## Quick Reference for Implementers (tree-sitter)

While walking the AST:

1. Maintain a **nesting depth** counter (starts at 0 inside a function).
2. On entering a Structural or Hybrid node → increment depth *after* scoring the node.
3. On a Structural node: score `1 + current_depth` (depth is the depth *before* entering).
4. On a Hybrid node: score `1` only (never add depth).
5. On a Fundamental node: score `1` (depth is irrelevant).
6. Function literals / nested funcs: do **not** score structurally, but *do* increment depth for their body.
7. Logical sequences: walk binary expressions of `&&` / `||` and count runs of identical operators.
8. Recursion: detect call graph cycles that include the current function (static analysis or simple call-site check for direct recursion).

## vs. Cyclomatic Complexity

| Aspect                        | Cyclomatic Complexity          | Cognitive Complexity                  |
|-------------------------------|--------------------------------|---------------------------------------|
| Basis                         | Graph theory (`E−N+2P`)        | Additive human-oriented rules         |
| Focus                         | Testability / paths            | Understandability / mental effort     |
| Floor per method              | 1                              | 0                                     |
| `switch`                      | +1 per case                    | +1 for the entire switch              |
| `else if`                     | Treated like `if`              | Hybrid: +1, no nesting penalty        |
| Aggregates above method level | Not meaningful                 | Meaningful                            |
```