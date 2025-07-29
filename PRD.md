# Product Requirements Document: make-lite

**Document Version:** 1.0
**Date:** July 29, 2025
**Author:** Alex Gaggin

---

## 1. Introduction & Philosophy

`make-lite` is a simple build automation tool designed to provide the core power of GNU Make while eliminating its most common frustrations and non-intuitive behaviors. It achieves this through a small set of simple, universal parsing rules.

-   **Structure over Syntax**: Rules are defined by their position relative to empty lines, not by special characters like tabs. Indentation is for readability only and is ignored.
-   **Universal Escape Character**: A backslash (`\`) is a universal escape character. Any character immediately following it is treated as a literal.
-   **Predictable Expansion**: Variable expansion is handled recursively by `make-lite` before commands are executed. A special `$$` variable provides a clear, idiomatic way to pass a literal `$` to the shell.
-   **Command Echo Control**: Commands are printed by default for clarity. Prefixing a command with `@` suppresses this output, mimicking standard Make behavior for cleaner logs.
-   **Automatic Directory Creation**: `make-lite` automatically creates parent directories for targets.
-   **Intuitive Multi-Target Rules**: A rule like `a b: c d` is treated as a single unit, executing its commands if *any* target is missing or older than *any* source.
-   **PHONY-less by Design**: Any target that is not a file or is a directory is treated as "always rebuild," removing the need for `.PHONY` declarations.

## 2. Makefile Parsing & Structure

### 2.1 File Pre-processing
- **File Combination**: `make-lite` starts by reading the root `Makefile.mk-lite`. When it encounters an `include <filename>` directive, it reads that file and inserts its content. This process is recursive, resulting in a single, large in-memory buffer of text before any other parsing occurs.
- **Content Gluing**: The beginning of the file (BOF), the end of the file (EOF), and the boundary between each included file are treated as "empty lines." This is crucial for the rule parsing logic to work consistently across multiple files.
- **Line Continuations**: After combining files, any line ending in a backslash (`\`) is joined with the subsequent line (the backslash and newline are removed).

### 2.2 Rule and Recipe Structure
- **Rule Definition**: The **first non-empty, non-comment line after an empty line** that contains an unescaped colon (`:`) is a **rule definition**.
- **Rule Structure**: The rule definition line consists of one or more whitespace-separated **targets** to the left of the colon, and zero or more whitespace-separated **sources** (dependencies) to the right.
- **Recipe**: All subsequent non-empty lines are part of that rule's **recipe**. The recipe block ends at the next empty line OR at the next line that is itself a new rule definition.
- **Variable Blocks**: Any block that does not begin with a rule definition is treated as a block of variable assignments.

### 2.3 General Syntax
- **Comments**: Text from an unescaped `#` (one not inside a quoted string) to the end of a line is ignored.

## 3. Variable & Environment System

### 3.1 Declaration & Precedence
- `VARIABLE = value`: Unconditional assignment.
- `VARIABLE ?= value`: Conditional assignment.
- **Precedence (Highest to Lowest)**:
  1.  **Makefile Unconditional (`=`):** Has the highest precedence and overrides all other sources.
  2.  **Shell Environment**: Variables from the calling shell (`VAR=val make-lite ...`).
  3.  **`load_env` Files**: Variables loaded from files using the `load_env` directive.
  4.  **Makefile Conditional (`?=`):** The lowest priority; only sets if the variable is not defined by any of the above.

### 3.2 Expansion Logic
`make-lite` has a single, unified, recursive expansion engine that adheres to the following principles:

1.  **`make-lite` Expands First**: `$(VAR)` or `$VAR` triggers recursive expansion within `make-lite`. The shell receives the final, expanded value, not the variable name.
2.  **`$$` for Shell Passthrough**: `$$` is a special built-in variable that expands to a single, literal `$`. This is the correct way to pass variable syntax to the shell for it to interpret (e.g., `echo $$HOME` becomes `echo $HOME` for the shell).
3.  **`$(shell ...)` Expansion**: The content inside `$(shell ...)` is recursively expanded by `make-lite` *first*. The resulting command string is then executed by a sub-shell that inherits `make-lite`'s full variable environment.

### 3.3 Syntax Details
- **Variable Naming**: Variable names may consist of letters, digits, hyphens (`-`), and underscores (`_`).
- **`load_env <filename>`**: Reads a file in `.env` format and strips surrounding quotes from values.

## 4. Execution & Dependency Management

- **Execution Model**: The entire build process is **strictly sequential**.
- **Dependency Resolution**: For any source file, `make-lite` will first recursively execute the rule that claims it as a target.
- **Freshness Check**: A rule's commands will execute if:
    - **Any** of its target files do not exist.
    - OR if the modification time of **any** source file is newer than the modification time of **any** target file.
- **Command Echoing**: By default, `make-lite` prints each recipe command to standard output before executing it. If a command is prefixed with an `@` symbol, the command is executed, but it is not printed.
- **Implicit Phony / Directory Handling**:
    - Any target name that does not correspond to an existing file (e.g., `clean`) is treated as a missing file, causing its rule to always run.
    - Any target or source that is a directory is treated as a missing file for the purpose of freshness checks, forcing a rebuild.
- **Automatic Directory Creation**: Before executing a recipe, `make-lite` will create the full directory path for each of the rule's targets if they do not already exist.
- **Error Handling**:
    - **Circular Dependencies**: Detected during dependency resolution, causing `make-lite` to exit with a critical error.
    - **Circular Includes**: Detected during file pre-processing, causing `make-lite` to exit with a critical error.
    - **Recipe Failure**: If a command fails, `make-lite` provides a descriptive error indicating the target, the failed command, and the exit code.

## 5. Command Line Interface (CLI)

- **Default Makefile**: `Makefile.mk-lite` in the current directory.
- **Default Target**: The first rule defined in the makefile.
- **Usage**: `make-lite [target_name]`
- **Flags**:
    - `--help`, `-h`: Display help message.
    - `--version`, `-v`: Display program version.
