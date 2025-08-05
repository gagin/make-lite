# Product Requirements Document: `make-lite`

**Document Version:** 1.2
**Date:** August 4, 2025
**Author:** Alex Gaggin

---

### 1. Introduction & Philosophy

`make-lite` is a macro and command automation tool born from the practical need to solve the most common and frustrating aspects of traditional Make. Its design is guided by a clear set of principles intended to create a predictable, powerful, and enjoyable developer experience.

-   **Simplicity and Predictability**: The tool prioritizes predictable behavior for the most common use cases over supporting every esoteric feature. If a syntax is ambiguous or can lead to surprising results, it is disallowed.

-   **Structure over Syntax**: The logical structure of the file is defined by clear, human-readable cues. **Indentation is used predictably to define recipe blocks,** avoiding the strict and often invisible tab requirements of traditional Make.

-   **Eliminating Annoyances by Design**: `make-lite` is explicitly designed to solve the most frustrating parts of traditional Make by replacing boilerplate and configuration with sensible, automated conventions. This includes:
    -   **Intuitive Dependency Rules**: A single, simple freshness check applies to all rules: if *any* target is missing or older than *any* source, the recipe runs.
    -   **Automatic Directory Creation**: If a target's parent directory does not exist, `make-lite` creates it automatically before running the recipe.
    -   **Implicit Phony Targets**: Any target that does not correspond to an existing file on disk is automatically treated as "phony," removing the need for `.PHONY` declarations.
    -   **Practical `.env` Parsing**: When a `.env` file is loaded, values enclosed in quotes have those quotes stripped, which is the behavior users almost always want.
    -   **Proactive Error Handling**: Common but unsupported GNU Make functions (e.g., `patsubst`, `foreach`) are detected and result in a clear error message, preventing silent failures.
    -   **Precise, Actionable Feedback**: All error and warning messages must report the exact file and line number of the issue. Warnings for common pitfalls, like variable redefinition, must be provided to help users debug their Makefiles.

-   **Transparent Execution Model**: The core logic of the build—what gets built and why—is always explicit in the makefile. At its core, `make-lite` is a powerful **macro and command runner**, not a complex build system with hidden behaviors.
    -   **Simple, Eager Expansion**: Variables are expanded in a single, predictable pass before a recipe command is run. There is no complex deferred expansion (`:=` vs. `=`) that can alter a variable's value unexpectedly during the build.
    -   **Clear Precedence**: The order in which variables are evaluated is simple and predictable, with Makefile assignments taking precedence over environment variables.

-   **Ease of Migration**: For common use cases, `make-lite` syntax is similar enough to GNU Make to facilitate easy migration of simple projects, allowing teams to adopt these fixes without a complete rewrite.

## 2. Makefile Parsing & Structure

### 2.1 File Pre-processing

The makefile is processed into a single, clean in-memory buffer before execution logic begins. This ensures consistency and simplifies the core parser. The steps are performed in this strict order:

1.  **Backslash Escaping**: As a foundational rule, a backslash (`\`) escapes the immediately following character, stripping it of any special meaning to the parser. This applies to characters like `#`, `:`, `=`, `(`, `)`, and `\` itself. For example, `\#` becomes a literal `#` and is not a comment, while `\\` becomes a literal `\`.

2.  **Comment Removal**:
    -   **Rule**: The parser first scans the raw file(s) and removes any text from an unescaped `#` to the end of the line.
    -   **Ambiguity Rule**: If a comment line ends in a backslash (`# ... \`), `make-lite` will exit with a fatal error.

3.  **File Inclusion**:
    -   **Rule**: After comments are removed, `include <filename>` directives are processed. The contents of the specified file replace the directive. This process is recursive.
    -   **Syntax**: The directive is `include`, followed by whitespace, followed by a filename. If the filename is enclosed in matching `'` or `"`, the quotes are stripped.
    -   **Search Path**: File paths are resolved **relative to the directory of the file containing the `include` directive**.
    -   **Error Condition**: Circular includes are detected and result in a fatal error.

4.  **Line Continuations**:
    -   **Rule**: After inclusion, any line ending in an unescaped backslash (`\`) is joined with the subsequent line. The backslash and newline are removed.

### 2.2 Global Structure & Two-Pass Parsing

`make-lite` employs a **two-pass parser** to ensure predictable behavior and eliminate ordering problems.

-   **Pass 1: Variable and Rule Collection**: The parser iterates through the entire pre-processed file content.
    -   **Variable assignments** are processed immediately. The right-hand side is eagerly expanded, and the variable is set in the store. If a variable is assigned multiple times, the **last definition wins**.
    -   **`load_env` directives** are processed immediately.
    -   **Rule definitions** and their associated recipes are collected in a raw, unexpanded form.
-   **Pass 2: Rule Expansion**: After the first pass is complete and the variable store is fully populated, the parser iterates over the collected raw rules. It now expands the variables in the target and source lists of each rule.
-   **Recipe**: A line is part of a rule's recipe **if and only if it is indented** (with one or more spaces or tabs). The recipe consists of the contiguous block of indented lines immediately following a rule definition.
-   **Recipe Termination**: A recipe block is terminated by the **first non-indented line** or by the end of the file. Empty lines that are not indented also terminate the recipe.

## 3. Variable & Environment System

### 3.1 Declaration & Precedence

-   **Assignment Syntax**:
    -   `VARIABLE = value`: Unconditional assignment. Overwrites any previous value.
    -   `VARIABLE ?= value`: Conditional assignment. Only sets if `VARIABLE` is not yet defined.
-   **Parsing Rule**: An assignment is a non-indented line containing an unescaped `=` or `?=`. The token to the left is the variable name. The value is everything to the right. Leading/trailing whitespace is trimmed from both the name and the value.
-   **Precedence (Highest to Lowest)**:
    1.  **Makefile Unconditional (`=`):** Allows the makefile author to have the final say.
    2.  **Shell Environment**: Variables from the command line or parent environment.
    3.  **`load_env` Files**: For project-level environment configuration.
    4.  **Makefile Conditional (`?=`):** Provides sensible defaults. This is the primary mechanism for allowing environment variables to override Makefile defaults.
-   **Redefinition Warning**: If an unconditional Makefile assignment (`=`) overwrites a previous unconditional Makefile assignment, a warning must be issued to `stderr`. This warning must include the variable name and the file and line number of both the new and previous definitions.

### 3.2 Expansion Logic

-   **Unified Expansion**: `make-lite` has a single, recursive expansion engine that processes backslash escapes and variable references before a command is passed to the shell.
-   **Syntax**:
    -   `$(...)`: The primary expansion form.
    -   `$VAR`: A shell-style convenience form for simple variables.
-   **Shell Passthrough (`$$`)**: The `$$` sequence expands to a single, literal `$`, which is then passed to the shell.
-   **Expansion Precedence within `$(...)`**:
    1.  **Explicit Shell (`$(shell ...)`):** The command inside `$(shell ...)` is expanded by `make-lite` first. The resulting string is executed by a sub-shell, and its standard output becomes the value of the expansion.
    2.  **Unsupported Function Error**: `make-lite` checks for common GNU Make functions (e.g., `patsubst`, `foreach`) and exits with a fatal "not supported" error to prevent unexpected behavior.
    3.  **Variable Expansion (`$(VAR)`)**: If the content is a defined `make-lite` variable, it is expanded.
    4.  **Implicit Shell Fallback**: If the content is not a defined variable and does not match a disallowed function, it is treated as an implicit shell command. The content is expanded and then executed in a sub-shell, with its output substituted.
-   **Error Condition**: Circular variable references (e.g., `A=$(B)`, `B=$(A)`) are detected and result in a fatal error during expansion.

### 3.3 Environment Loading

-   **`load_env <filename>`**: Reads a file in `.env` format.
-   **Parsing Rules**: Lines are parsed as `KEY=VALUE`. Comment lines and blank lines are ignored. Anything preceding the last token before the assignment operator (including `export`) is ignored. The `VALUE` is processed as follows:
    1.  Leading and trailing whitespace is trimmed.
    2.  Then, if the resulting string is enclosed in a matching pair of `'` or `"`, those outer quotes are stripped.

## 4. Execution & Dependency Management

-   **Circular Dependency Detection**: If a target depends on itself through a chain of rules, `make-lite` will detect this cycle and exit with a fatal error.
-   **Sequential Execution**: If a target has multiple dependencies, they are resolved and built one at a time in the order they are listed.
-   **Fail-Fast**: If any command in a recipe fails (returns a non-zero exit code), `make-lite` stops immediately and reports that the recipe for that target failed. If a required dependency is missing and there is no rule to create it, `make-lite` stops with a fatal error.
-   **Command Echoing & Suppression (`@`)**: By default, recipe commands are printed after expansion and before execution. A command prefixed with `@` is executed silently.
-   **Automatic Variable Export**: All `make-lite` variables are automatically expanded and exported to the environment of any sub-shell.
-   **Freshness Check**: A rule's recipe will execute if:
    1.  **Any** of its target files do not exist.
    2.  OR the modification time of **any** source file is newer than the modification time of **any** target file.
-   **Automatic Directory Creation**: Before executing a recipe, `make-lite` will create the full directory path for each of the rule's targets.
-   **Refined Directory & Phony Handling**:
    -   A target that corresponds to a directory on disk, or a target name that does not correspond to a file and has no sources, is treated as "always out of date," causing its rule to always run. A source that is a directory has its modification time (`mtime`) checked like a regular file.

## 5. Command Line Interface (CLI)

-   **Default Makefile**: `Makefile.mk-lite`
-   **Default Target**: The first rule defined in the makefile.
-   **Usage**: `make-lite [options] [target_name]`
-   **Flags**:
    -   `--help`, `-h`: Display help message.
    -   `--version`, `-v`: Display program version.
-   **Debugging**: Set the environment variable `MAKE_LITE_LOG_LEVEL=DEBUG` to enable verbose output, including the exact commands being sent to the shell.

## 6. Coding quality
    -   **Centralized Configuration**: Internal application configuration, such as user-facing strings and version numbers, are centralized for maintainability and consistency.
    -   **Lintable code**: Make lint happy, specifically in checking return error codes on all operations.
