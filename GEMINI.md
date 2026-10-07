# Common Expression Language (CEL) Skills

This repository provides tools and skills to author and debug CEL expressions.

## Provided Tools

### `cel-expr` CLI (Recommended)

The standalone `cel-expr` CLI requires no special agent configuration and runs directly via standard terminal commands:

- `cel-expr env -env <path|json>`: Defines/validates variables, functions, and types.
- `cel-expr prompt -env <path|json> -prompt <requirement>`: Generates authoring prompt.
- `cel-expr compile -env <path|json> -expr <expr>`: Compiles and type-checks CEL expressions.
- `cel-expr eval -env <path|json> -expr <expr> -tests <path|json>`: Evaluates expressions against test cases.

In open source, build the CLI with `go build -o cel-expr ./cmd/cel-expr` or install via `go install github.com/cel-expr/skills/cmd/cel-expr@latest` (and `alias cel-expr="$(go env GOPATH)/bin/cel-expr"` if `$(go env GOPATH)/bin` is not on your `PATH`).

### `cel-expr-mcp` MCP Server

The `cel-expr-mcp` MCP server provides matching tools (`cel_create_environment`,
`cel_generate_prompt`, `cel_compile`, `cel_evaluate`) for MCP-compatible clients.

The server accepts startup flags to tailor the tool experience:

- `-environment` (alias `-env`): Path to a JSON file or JSON string containing
   the CEL environment configuration.
- `-file_descriptor_set` (alias `-file_descriptors`): Path to a binary
   `FileDescriptorSet` file containing protobuf definitions referenced as types
   within an environment.

## Available Skills

To understand how to best use these tools, please refer to the `cel-authoring`,
`cel-debugging`, and `cel-testing` skills, which will be automatically available
to the agent when this configuration is loaded.
