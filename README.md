# make-lite

A simplified build automation tool written in Go, inspired by GNU Make but with a streamlined feature set.

## Features

- **Simplified Rule Definition**: Rules are defined as the first line after an empty line, with targets on the left of a colon and sources (dependencies) on the right.
- **Automatic Phony Targets**: Any target that does not correspond to an existing file will automatically trigger its associated commands, effectively behaving as a "phony" target without explicit declaration.
- **Directory Handling**: If a target or source is a directory, it is treated as if it were a missing file, forcing its associated commands to run.
- **Automatic Directory Creation**: Before executing a rule, `make-lite` will automatically create any necessary parent directories for the target file if they do not already exist.
- **Dependency Resolution**: Automatically resolves and executes dependencies in the correct order.
- **File Freshness Checks**: Commands for a target are executed only if the target file does not exist or is older than any of its dependencies.
- **Circular Dependency Detection**: Prevents infinite loops by detecting and reporting circular dependencies.
- **Variable Management**: Supports `VARIABLE=value` assignments, `VARIABLE?=value` for conditional assignment (if not already set), and `$NAME` for variable substitution. Variables can also be read from the environment.
- **Shell Command Execution**: Allows `$(shell command)` for dynamic command output inclusion.
- **Makefile Inclusion**: Supports `include` directives to combine multiple makefiles into a single in-memory representation before processing. The default makefile name is `Makefile-lite`.
- **Comment Handling**: Ignores lines starting with `#` (unescaped).

## Usage

To run a target, use:

```bash
go run main.go <target_name>
```

If no target is specified, `make-lite` will attempt to execute the first rule defined in the `Makefile-lite`.

Examples:

- To build the `make-lite` executable: `go run main.go all`
- To clean the built executable: `go run main.go clean`
- To see environment variables: `go run main.go print_env`
- To see shell command output: `go run main.go print_shell`
- To run the included target: `go run main.go included_target`