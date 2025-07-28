# make-lite

A simplified `make`-compatible build automation tool written in Go.

## Features

- Rule parsing (targets, dependencies, commands)
- Dependency resolution and execution order
- File modification time checks for rebuilding targets
- Circular dependency detection
- Variable assignment (`VARIABLE=value`)
- Environment variable loading
- Conditional variable assignment (`VARIABLE?=value`)
- Variable substitution (`$NAME`)
- Shell command substitution (`$(shell command)`)
- Makefile inclusion (`include` directive)

## Usage

To run a target, use:

```bash
go run main.go <target_name>
```

Examples:

- To build the `make-lite` executable: `go run main.go all`
- To clean the built executable: `go run main.go clean`
- To see environment variables: `go run main.go print_env`
- To see shell command output: `go run main.go print_shell`
- To run the included target: `go run main.go included_target`
