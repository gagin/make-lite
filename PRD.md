# Product Requirements Document: make-lite

**Document Version:** 1.0
**Date:** July 28, 2025
**Author:** Gemini (AI Assistant)

---

## 1. Introduction

### 1.1 Purpose
The purpose of this document is to define the requirements for `make-lite`, a simplified build automation tool. It aims to provide a lightweight, yet powerful, alternative to traditional build systems like GNU Make, focusing on core functionalities essential for common software development workflows.

### 1.2 Scope
`make-lite` will focus on parsing a custom Makefile format (`Makefile-lite`), resolving dependencies, executing shell commands, and managing variables. It is designed for projects that require straightforward build automation without the complexity and idiosyncrasies of larger build tools.

### 1.3 Target Audience
This PRD is intended for developers, project managers, and any stakeholders involved in the development, testing, or deployment of `make-lite`.

## 2. Goals

### 2.1 Business Goals
- To provide a simple, understandable, and efficient build automation tool for Go projects and other general-purpose scripting.
- To reduce the learning curve associated with complex build systems.
- To offer a portable solution that can be easily integrated into various development environments.

### 2.2 User Goals
- Users should be able to define build rules and dependencies clearly and concisely.
- Users should be able to execute specific targets or the default build process with ease.
- Users should receive clear feedback on build status, including errors and up-to-date messages.
- Users should be able to manage build-specific variables and leverage environment variables.

## 3. User Stories / Features

### 3.1 Core Build Automation
- **As a developer, I want to define build rules** so that I can specify how to build my project components.
  - **Acceptance Criteria**:
    - A rule is defined as the first non-empty, non-comment line after an empty line.
    - The rule line contains a target, a colon, and optionally sources (dependencies).
    - Example: `target: source1 source2`
- **As a developer, I want to specify dependencies for my targets** so that `make-lite` knows the order in which to build components.
  - **Acceptance Criteria**:
    - Dependencies are listed after the colon on the rule line, separated by whitespace.
    - Dependencies can span multiple lines using a backslash (`\`) for continuation.
- **As a developer, I want to define commands to execute for a rule** so that `make-lite` can perform the necessary build steps.
  - **Acceptance Criteria**:
    - Commands are listed on subsequent lines after the rule definition.
    - Commands are executed sequentially.
    - Indentation is allowed but ignored (for readability).
- **As a developer, I want `make-lite` to automatically create target directories** so that I don't have to manually create them in my build scripts.
  - **Acceptance Criteria**:
    - If a target path includes directories that do not exist, `make-lite` will create the full directory path before executing the rule's commands.
    - If an existing directory in the path is not writable, `make-lite` will exit with a critical error.

### 3.2 Dependency Management & Execution Flow
- **As a developer, I want `make-lite` to resolve dependencies recursively** so that all prerequisites are built before their dependents.
  - **Acceptance Criteria**:
    - For any source file on the right side of a rule, `make-lite` will check if it is a target of another rule and execute that rule first.
- **As a developer, I want `make-lite` to only rebuild targets when necessary** so that build times are optimized.
  - **Acceptance Criteria**:
    - `make-lite` will check if the target file exists and if it's newer than all its source files.
    - If the target file does not exist, or if any source file is newer than the target file, the commands for that rule will be executed.
    - If a target name does not correspond to an existing file (e.g., a "phony" target like `clean`), its commands will always be executed.
    - If a target or source is a directory, it will be treated as a missing file, forcing its associated commands to run.
- **As a developer, I want `make-lite` to detect circular dependencies** so that I can avoid infinite build loops.
  - **Acceptance Criteria**:
    - If a circular dependency is detected, `make-lite` will exit with a critical error message.

### 3.3 Variable Handling
- **As a developer, I want to define variables within my `Makefile-lite`** so that I can reuse values.
  - **Acceptance Criteria**:
    - Variables are declared using `VARIABLE=value`.
    - Variables can be substituted using `$VARIABLE`.
- **As a developer, I want to conditionally set variables** so that I can provide default values.
  - **Acceptance Criteria**:
    - Variables can be conditionally set using `VARIABLE?=value`. If `VARIABLE` is already set (either in the `Makefile-lite` or from the environment), its value will not be overwritten.
- **As a developer, I want to use environment variables** so that I can configure builds externally.
  - **Acceptance Criteria**:
    - Environment variables are loaded and available for substitution.
    - Environment variables take precedence over `VARIABLE?=value` declarations but can be overridden by `VARIABLE=value` declarations within the `Makefile-lite`.
- **As a developer, I want to execute shell commands and use their output as variable values** so that I can dynamically generate build parameters.
  - **Acceptance Criteria**:
    - `$(shell command)` syntax will execute the command and substitute its standard output (trimmed) into the variable.
    - Variables within `$(shell command)` will be expanded before execution.

### 3.4 Makefile Inclusion
- **As a developer, I want to include other `Makefile-lite` files** so that I can modularize my build definitions.
  - **Acceptance Criteria**:
    - The `include <filename>` directive will read the content of the specified file.
    - All included content will be combined into a single in-memory representation before processing.
    - Circular includes will be detected and result in a critical error.

### 3.5 Command Line Interface (CLI)
- **As a user, I want to specify a target to build** so that I can control which part of the project is built.
  - **Acceptance Criteria**:
    - `make-lite <target_name>` will execute the specified target.
- **As a user, if no target is specified, I want `make-lite` to build the first rule defined** so that I have a default build action.
  - **Acceptance Criteria**:
    - Running `make-lite` without arguments will execute the first rule encountered in the `Makefile-lite`.

## 4. Functional Requirements

- **FR1: Makefile Parsing**: The system shall parse `Makefile-lite` files, identifying rules, dependencies, commands, and variable declarations.
- **FR2: Variable Resolution**: The system shall resolve variables, respecting precedence (environment > `?=` > `=`).
- **FR3: Shell Command Execution**: The system shall execute shell commands defined in rules and within `$(shell ...)` constructs.
- **FR4: Dependency Graph Construction**: The system shall build an internal dependency graph based on parsed rules.
- **FR5: Build Execution Logic**: The system shall traverse the dependency graph, executing rules based on freshness checks and dependency order.
- **FR6: Error Handling**: The system shall provide clear error messages for parsing errors, circular dependencies, command failures, and missing targets/dependencies.
- **FR7: File System Interaction**: The system shall interact with the file system to check file modification times, create directories, and execute commands.

## 5. Non-Functional Requirements

- **NFR1: Performance**: `make-lite` shall execute builds efficiently, especially for incremental builds.
- **NFR2: Reliability**: `make-lite` shall consistently produce correct build outputs given valid inputs.
- **NFR3: Usability**: The `Makefile-lite` syntax shall be straightforward and easy to understand for developers familiar with basic build concepts.
- **NFR4: Portability**: `make-lite` shall be runnable on common operating systems (Linux, macOS, Windows) where Go is supported.
- **NFR5: Maintainability**: The codebase shall be well-structured, commented, and easy to extend or debug.

## 6. Technical Requirements / Constraints

- **TR1: Language**: Implemented in Go.
- **TR2: Makefile Format**: Custom `Makefile-lite` format (not fully GNU Make compatible).
- **TR3: Shell Execution**: Commands will be executed via `bash -c` (or equivalent on Windows).
- **TR4: File Naming**: Default makefile name is `Makefile-lite`.

## 7. Future Considerations (Out of Scope for V1.0)

- **Pattern Rules**: Support for generic rules based on file patterns (e.g., `%.o: %.c`).
- **Phony Target Declaration**: Explicit `.PHONY` declaration for targets that do not produce files. (Currently handled implicitly).
- **Wildcard Expansion**: Support for wildcard characters in dependencies.
- **Built-in Functions**: Additional built-in functions beyond `$(shell ...)`.
- **Parallel Execution**: Running independent commands in parallel.
- **Error Handling Improvements**: More granular error reporting and recovery options.
- **Cross-Platform Shell Compatibility**: Enhanced handling for different shell environments (e.g., PowerShell, cmd.exe).
