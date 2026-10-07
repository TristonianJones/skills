---
name: cel-authoring
description: >-
  Authoring, configuring, and testing Common Expression Language (CEL)
  expressions, policies, rules, and environment JSON configurations.
---

# Common Expression Language (CEL) Authoring Skill

Use this skill to guide authoring, compiling, and testing Common Expression
Language (CEL) expressions using the `cel-expr` CLI:

```bash
go install github.com/cel-expr/skills/cmd/cel-expr@latest
alias cel-expr="$(go env GOPATH)/bin/cel-expr"
```

## Workflow

Follow this end-to-end workflow to author CEL expressions:

1.  **Locate or Create Environment**: Identify/write the CEL environment
    configuration JSON file (`envConfig`). Run
    `cel-expr env -env <env.json>` to validate
    the `envConfig`.
2.  **Generate Prompt**: Run
    `cel-expr prompt -env <env.json> -prompt "<requirement>"`
    with the user request to create the CEL authoring prompt.
3.  **Create Expression**: Generate CEL using only the authoring prompt. Run
    `cel-expr compile -env <env.json> -expr "<expr>"`
    to validate the expression.
4.  **Evaluate and Test**: Run
    `cel-expr eval -env <env.json> -expr "<expr>" -tests <tests.json>`
    to verify behavior and iterate on test coverage until there is >95% node and
    branch coverage.

## Locate or Create Environment

Find the environment definition containing the variables, functions, macros and
features supported in your CEL expression using `code_search`, `find_by_name`,
or `grep_search` and the `view_file` tool to read it.

If the environment does not exist, generate an `envConfig` object and run
`cel-expr env -env <env.json>` to validate it.

*   Namespaced variables with simple types improve correctness checks.
*   Use namespacing for related concepts, e.g. `request.path`, `request.time`.
*   Use protobuf object types for strongly typed structured data.
*   Only use `map<string, dyn>` the variable is a dynamic structure like JSON
*   Type names are formatted according to:
    [type_grammar_ebnf.txt](skills/cel-authoring/references/type_grammar_ebnf.txt).

Save the `envConfig` json using `write_to_file` after validation.

## Generate Prompt

Provide the JSON environment to `cel-expr prompt` with the user's original
request:

```bash
cel-expr prompt -env <env.json> -prompt "<user request>"
```

Exclusively use the output CEL prompt for generating expressions.

## Create Expression

Use the CEL prompt to create the simplest possible expression which satisfies
the user's requirements. Run `cel-expr compile` to validate the CEL:

```bash
cel-expr compile -env <env.json> -expr "<expression>"
```

Correct errors with the
[cel-debugging](skills/cel-debugging/SKILL.md)
skill.

## Evaluate and Test

Run `cel-expr eval` with an expression and test cases to validate an expression:

```bash
cel-expr eval -env <env.json> -expr "<expression>" -tests <tests.json>
```

Test cases must cover edge cases and missing fields to ensure robustness
and correctness. Prune tests which do not increase coverage.

Edge cases:

*   Using a map where a qualified identifier with a simple type would be safer.
    *   Before: `request: map<string, dyn>` and `request.path == 'value'`
    *   After: `request.path: string` and `request.path == 'value'`
*   Unguarded map key or list index accesses:
    *   Before: `m[k]`, After: `k in m && m[k]` or `m[?k].orValue(<default>)`
    *   Before: `l[i]`, After: `l.size() > i && l[i]` or `l[?i].orValue(<default>)`
Note: the `?` syntax requires enabling `optional` extension in the `envConfig`.

The coverage report for `cel-expr eval` indicates the node and branch coverage
percentages as a pre-formatted string (e.g., `"Node: %.2f%%, Branch: %.2f%%"`).
Generate additional test cases until you achieve >95% node and branch coverage.

Report the test results with node and branch coverage percentages.

## Complete Example

The following is a complete example of the artifacts generated from the prompt:
"Validate the request is authenticated by checking for a bearer token"

*   Environment:
    [network_env.json](skills/cel-authoring/examples/network_env.json)
*   Expression:
    [network_headers.cel](skills/cel-authoring/examples/network_headers.cel)
*   Tests:
    [network_headers_tests.json](skills/cel-authoring/examples/network_headers_tests.json)
