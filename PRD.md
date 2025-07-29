# Product Requirements Document: make-lite

**Document Version:** 1.4
**Date:** July 28, 2025
**Author:** Gemini (AI Assistant)

---

## 1. Introduction

### 1.1 Purpose
The purpose of this document is to define the requirements for `make-lite`, a simplified build automation tool. It aims to provide a lightweight, yet powerful, alternative to traditional build systems like GNU Make, focusing on core functionalities essential for common software development workflows.

### 1.2 Scope
`make-lite` will parse a custom `Makefile-lite` format, resolve dependencies, execute shell commands sequentially, and manage variables. It is designed for projects that require straightforward build automation without the complexity of larger build tools.

### 1.3 Target Audience
This PRD is intended for developers, project managers, and any stakeholders involved in the development, testing, or deployment of `make-lite`.

### 1.4 Design Philosophy: Avoiding Make's "Idiosyncrasies"
`make-lite` is designed to provide the power of a rule-based build system while eliminating common frustrations and non-intuitive behaviors found in GNU Make. It achieves this by adhering to the following principles:

-   **No-Tab-Required Syntax**: Command indentation is for human readability only and is ignored by the parser, eliminating errors from using spaces instead of tabs.
-   **Automatic Directory Creation**: `make-lite` will automatically create the full directory path for a target before executing its commands, removing the need for `mkdir -p` in recipes. This is specified as rule #15.
-   **Intuitive Multi-Target Rules**: A rule with multiple targets (`a b: c d`) is treated as a single unit. Its commands are executed **once** if **any** target is missing or if **any** target is older than **any** source. This fixes the common `make` issue where it fails to correctly handle rules that generate multiple files from one command (e.g., `protoc`).
-   **PHONY-less by Design**: Any target that does not correspond to a file on disk (like `clean`) is implicitly "phony" and its commands will always run. Any target or source that is a directory is also treated as a missing file, forcing a rebuild. This is specified as rule #14.

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

### 3.1 Makefile Parsing & Structure
- **As a developer, I want to define build rules based on a simple structure** so that my makefile is easy to read.
  - **Acceptance Criteria**:
    - A rule is defined by the first non-empty, non-comment line following an empty line. The beginning and end of the file are also treated as empty lines for this purpose.
    - The rule definition line contains one or more whitespace-separated targets on the left of a colon, and zero or more whitespace-separated sources (dependencies) on the right. Example: `target1 target2: source1 source2`
    - Commands for a rule are listed on the lines immediately following the rule definition, continuing until the next empty line or the end of the file.
- **As a developer, I want to write multi-line statements** for better formatting.
  - **Acceptance Criteria**:
    - Any line ending in a backslash (`\`) is joined with the subsequent line (the backslash and newline are removed). This is a pre-processing step performed when files are first read and combined, before any rule or syntax parsing occurs.
- **As a developer, I want to add comments to my makefile.**
  - **Acceptance Criteria**:
    - Any text from an unescaped hash symbol (`#`) to the end of the line is a comment, unless it is inside a `$(shell ...)` block.

### 3.2 Variable Handling
- **As a developer, I want to define and use variables** to avoid repetition.
  - **Acceptance Criteria**:
    - `VARIABLE=value` is an unconditional assignment that overrides any previous value, including environment variables.
    - Variable names may consist of uppercase letters, digits, underscores (`_`), and hyphens (`-`).
    - Variables can be substituted using either `$VARIABLE` or `$(VARIABLE)` syntax.
- **As a developer, I want to conditionally set variables** to provide default values.
  - **Acceptance Criteria**:
    - `VARIABLE?=value` only assigns the value if `VARIABLE` has not been set previously (either from the environment or an earlier assignment).
- **As a developer, I want to use environment variables** so that I can configure builds externally.
  - **Acceptance Criteria**:
    - Environment variables are loaded at startup. They can be overridden by an unconditional assignment (`=`) within the `Makefile-lite`.
- **As a developer, I want to execute shell commands and use their output as variable values.**
  - **Acceptance Criteria**:
    - The `$(shell command)` syntax executes a command and substitutes its trimmed standard output as the value.
    - The parser distinguishes `$(shell ...)` from `$(VARIABLE)` by checking for the literal `shell` keyword followed by a space.
    - The parser identifies the command content by scanning from `$(shell ` to the final, matching closing parenthesis `)`, correctly handling nested parentheses.
    - Within a `$(shell ...)` block, comment characters (`#`) are treated as literal characters for the shell.
    - Before the command is executed, the following escape sequences within the extracted command string are processed: `\(` is unescaped to `(`, `\)` is unescaped to `)`, and `\\` is unescaped to `\`. No other backslash sequences are interpreted.

### 3.3 Dependency Management & Execution Flow
- **As a developer, I want `make-lite` to resolve dependencies recursively and sequentially** so that all prerequisites are built in a predictable order.
  - **Acceptance Criteria**:
    - For any source file, `make-lite` checks if it is a target of another rule and executes that rule first.
    - The entire build process is strictly sequential; no commands are run in parallel.
- **As a developer, I want `make-lite` to only rebuild targets when necessary** so that build times are optimized.
  - **Acceptance Criteria**:
    - A rule's commands will be executed if **any** of the rule's target files do not exist, OR if the modification time of **any** source file is newer than the modification time of **any** target file.
    - If a target or source path points to a directory, it is treated as a missing file.
- **As a developer, I want `make-lite` to automatically create target directories.**
  - **Acceptance Criteria**:
    - Before executing a rule's commands, `make-lite` will create the full directory path for each target if it does not already exist.
- **As a developer, I want `make-lite` to detect circular dependencies.**
  - **Acceptance Criteria**:
    - If a circular dependency is detected, `make-lite` will exit with a critical error message.

### 3.4 Makefile Inclusion
- **As a developer, I want to include other `Makefile-lite` files** so that I can modularize my build definitions.
  - **Acceptance Criteria**:
    - The `include <filename>` directive will read the content of the specified file.
    - All included content is combined into a single in-memory representation before processing, with newlines separating the content of each file.
    - Circular includes will be detected and result in a critical error.

### 3.5 Command Line Interface (CLI)
- **As a user, I want to specify a target to build.**
  - **Acceptance Criteria**:
    - `make-lite <target_name>` will execute the specified target.
- **As a user, if no target is specified, I want `make-lite` to build the first rule defined.**
  - **Acceptance Criteria**:
    - Running `make-lite` without arguments will execute the first rule encountered in the `Makefile-lite`.
- **As a user, I want to see usage information and version.**
  - **Acceptance Criteria**:
    - `make-lite --help` or `make-lite -h` displays a help message.
    - `make-lite --version` or `make-lite -v` displays the program's version.

## 4. Functional Requirements

- **FR1: Makefile Parsing**: The system shall parse `Makefile-lite` files by first combining all included files, processing line continuations, and then identifying rules, variables, and commands. It must correctly handle special parsing contexts like `$(shell ...)`.
- **FR2: Variable Resolution**: The system shall load environment variables, then process makefile assignments. `VARIABLE=value` always overrides. `VARIABLE?=value` only sets if the variable is not already defined.
- **FR3: Shell Command Execution**: The system shall execute shell commands defined in rules and within `$(shell ...)` constructs.
- **FR4: Dependency Graph Construction**: The system shall build an internal dependency graph based on parsed rules.
- **FR5: Build Execution Logic**: The system shall traverse the dependency graph and execute rules sequentially based on a clear freshness check: rebuild if any target is missing, or if any source is newer than any target.
- **FR6: Error Handling**: The system shall provide clear error messages for parsing errors, circular dependencies, command failures, and file system errors.
- **FR7: File System Interaction**: The system shall interact with the file system to check file modification times, create directories for targets, and execute commands. It must treat directories as non-existent files for dependency-checking.

## 5. Non-Functional Requirements

- **NFR1: Performance**: `make-lite` shall execute builds efficiently, especially for incremental builds.
- **NFR2: Reliability**: `make-lite` shall consistently produce correct build outputs given valid inputs.
- **NFR3: Usability**: The `Makefile-lite` syntax shall be straightforward and easy to understand.
- **NFR4: Portability**: `make-lite` shall be runnable on common operating systems where Go is supported.

## 6. Technical Requirements / Constraints

- **TR1: Language**: Implemented in Go.
- **TR2: Makefile Format**: Custom `Makefile-lite` format.
- **TR3: Shell Execution**: Commands will be executed via `bash -c` (or equivalent on Windows).
- **TR4: File Naming**: Default makefile name is `Makefile-lite`.
- **TR5: Execution Model**: Execution is strictly sequential. Parallel execution is explicitly out of scope.

## 7. Future Considerations (Out of Scope for V1.4)

- **`foreach` Constructs**: Support for iterating over lists of values to generate rules or commands.
- **Pattern Rules**: Support for generic rules based on file patterns (e.g., `%.o: %.c`).
- **Built-in Functions**: Additional built-in functions beyond `$(shell ...)`.
