# Product Requirements Document: `make-lite`

**Document Version:** 1.0
**Date:** July 29, 2025
**Author:** Alex Gaggin

---

### 1. Introduction & Philosophy

`make-lite` is a build automation tool born from the practical need to solve the most common and frustrating aspects of traditional Make. Its design is guided by a clear set of principles intended to create a predictable, powerful, and enjoyable developer experience.

-   **Simplicity and Predictability**: The tool prioritizes predictable behavior for the most common use cases over supporting every esoteric feature. If a syntax is ambiguous or can lead to surprising results, it is disallowed.

-   **Structure over Syntax**: The logical structure of the file is defined by human-readable cues like empty lines, not by invisible or special characters like tabs. **Indentation is for humans, not for the parser.**

-   **Eliminating Annoyances by Design**: `make-lite` is explicitly designed to solve the most frustrating parts of traditional Make by replacing boilerplate and configuration with sensible, automated conventions. This includes:
    -   **Intuitive Dependency Rules**: A single, simple freshness check applies to all rules: if *any* target is missing or older than *any* source, the recipe runs. This ensures code generators that create multiple files work as expected.
    -   **Automatic Directory Creation**: If a target's parent directory does not exist, `make-lite` creates it automatically before running the recipe.
    -   **Implicit Phony Targets**: Any target that is not an existing file (or is a directory) is automatically treated as "phony," removing the need for `.PHONY` declarations and their confusing side effects.
    -   **Practical `.env` Parsing**: When a `.env` file is loaded, values enclosed in quotes have those quotes stripped, which is the behavior users almost always want.
    -   **Implicit Variable Export**: All variables are available to recipe commands by default, removing the need for an `export` keyword.

-   **Transparent Execution Model**: The core logic of the build—what gets built and why—is always explicit in the makefile. At its core, `make-lite` is a powerful **macro and command runner**, not a complex build system with hidden behaviors.
    -   **Simple, Eager Expansion**: Variables are expanded in a single, predictable pass before recipe commands are run. There is no complex deferred expansion (`:=` vs. `=`) that can alter a variable's value unexpectedly during the build.
    -   **Explicit Conventions, Not Hidden Magic**: Features like automatic quote stripping in `.env` are considered explicit quality-of-life conventions. They simplify the syntax of the makefile but do not alter the fundamental command-and-dependency flow in a surprising way.

-   **Ease of Migration**: For common use cases, `make-lite` syntax is similar enough to GNU Make to facilitate easy migration of simple projects, allowing teams to adopt these fixes without a complete rewrite.

## 2. Makefile Parsing & Structure

### 2.1 File Pre-processing

The makefile is processed into a single, clean in-memory buffer before execution logic begins. This ensures consistency and simplifies the core parser. The steps are performed in this strict order:

1.  **Comment Removal**:
    -   **Rule**: The parser first scans the raw file(s) and removes any text from an unescaped `#` to the end of the line.
    -   **Reasoning**: Processing comments first is critical. It allows users to comment out any line, including `include` directives, which is a common and essential debugging technique.
    -   **Use-Case**: A user wants to temporarily disable a section of the build (`# include feature.mk-lite`) without deleting the line.
    -   **Ambiguity Rule**: If a comment line ends in a backslash (`# ... \`), `make-lite` will exit with a fatal error.
    -   **Reasoning**: Allowing line continuation within a comment is highly ambiguous and can lead to silently ignoring a line of code that was intended to be joined with the comment. Forcing an error makes the user's intent explicit.

2.  **File Inclusion**:
    -   **Rule**: After comments are removed, `include <filename>` directives are processed. The contents of the specified file replace the directive. This process is recursive.
    -   **Syntax**: The directive is `include`, followed by whitespace, followed by a filename. If the filename is enclosed in matching `'` or `"`, the quotes are stripped.
    -   **Search Path**: File paths are resolved **relative to the directory of the file containing the `include` directive**.
    -   **Reasoning**: Relative paths promote modular and portable makefile components. A feature's build logic can be self-contained and included from a root makefile without path manipulation.
    -   **Use-Case**: A project has a `docs/` subdirectory with its own `build.mk-lite`. The root `Makefile.mk-lite` can simply `include docs/build.mk-lite` to integrate it.

3.  **Line Continuations**:
    -   **Rule**: After inclusion, any line ending in a backslash (`\`) is joined with the subsequent line. The backslash and newline are removed.
    -   **Reasoning**: This is a standard convenience for improving the readability of very long rule definitions or commands.

### 2.2 Global Structure

`make-lite` processes the makefile as a simple, top-to-bottom script.

-   **Sequential Evaluation**: Lines are parsed and evaluated in the order they appear. Variable assignments take effect immediately and are available to subsequent lines.
-   **Rule Definition**: The **first line in a section that contains an unescaped colon (`:`)** is a **rule definition**. It consists of one or more whitespace-separated **targets** to the left of the colon, and zero or more **sources** to the right.
-   **Recipe**: All subsequent non-empty, non-comment lines are part of that rule's **recipe**.
-   **Recipe Termination**: A recipe is terminated by the **first empty line** or by the end of the file. An empty line's only function is to signal the end of a recipe.
-   **Reasoning**: This model is simple and requires no complex look-ahead or block analysis. A line is either an assignment, a rule definition, or part of a recipe. The empty line provides a clear, visual delimiter between a rule's recipe and whatever follows, enhancing readability.

## 3. Variable & Environment System

### 3.1 Declaration & Precedence

`make-lite` has a layered variable system, allowing for flexible configuration.

-   **Assignment Syntax**:
    -   `VARIABLE = value`: Unconditional assignment. Overwrites any previous value.
    -   `VARIABLE ?= value`: Conditional assignment. Only sets if `VARIABLE` is not yet defined.
-   **Parsing Rule**: An assignment is a line containing an unescaped `=` or `?=`. The token to the left is the variable name. The value is everything to the right. Leading/trailing whitespace is trimmed from both the name and the value. Any text on the line before the variable name (e.g., `export`) is ignored.
    -   **Reasoning**: This makes the parser robust and user-friendly. It correctly handles common copy-paste scenarios from shell scripts (`export VAR=val`) and forgives minor formatting inconsistencies.
-   **Precedence (Highest to Lowest)**:
    1.  **Makefile Unconditional (`=`):** Allows the makefile author to have the final say, guaranteeing a build's configuration.
    2.  **Shell Environment**: Allows a user to provide temporary, one-off overrides from the command line (e.g., `CC=clang make-lite`).
    3.  **`load_env` Files**: For project-level environment configuration that shouldn't be checked into the main makefile (e.g., secrets, local paths). See 3.3.
    4.  **Makefile Conditional (`?=`):** Provides sensible defaults that can be easily overridden by any other method.

### 3.2 Expansion Logic

-   **Unified Expansion**: `make-lite` has a single, recursive expansion engine that runs *before* a command is passed to the shell.
-   **Syntax**:
    -   `$(VAR)`: The primary, explicit form. The variable name is everything between the parentheses. This is the **required** form for variables containing hyphens.
    -   `$VAR`: A shell-style convenience form. The variable name is the longest subsequent string of letters, digits, and underscores.
    -   **Reasoning**: Supporting both forms provides the best of both worlds. `$VAR` is familiar and fast for simple variables. `$(VAR)` is unambiguous and necessary for more complex names (e.g., `$(GO-LDFLAGS)`), preventing parsing errors.
-   **Shell Passthrough (`$$`)**: The `$$` sequence expands to a single, literal `$`.
    -   **Reasoning**: This provides a clear, explicit way to pass a variable to the shell for its own expansion, distinguishing it from variables meant to be expanded by `make-lite`.
    -   **Use-Case**: A recipe needs to access a shell environment variable: `echo "Home is $$HOME"` becomes `echo "Home is $HOME"` for the shell to execute.
-   **Shell Execution (`$(shell ...)`):** The command inside `$(shell ...)` is expanded by `make-lite` first. The resulting string is executed by a sub-shell, and its standard output becomes the value of the expansion.
    -   **Reasoning**: This is a powerful feature for dynamic variable creation, consistent with standard Make functionality. All `make-lite` variables are passed to this sub-shell's environment.

### 3.3 Environment Loading

-   **`load_env <filename>`**: Reads a file in `.env` format.
    -   **Parsing Rules**: Lines are parsed as `KEY=VALUE`. Comment lines (`#...`) and blank lines are ignored. The `export` prefix is ignored. Crucially, if the `VALUE` is enclosed in a matching pair of `'` or `"`, those quotes are stripped.
    -   **Reasoning**: This provides a standard, idiomatic way to manage project-specific environment variables without polluting the user's global shell environment. Stripping quotes is a major quality-of-life feature, as `.env` files often quote values containing spaces, and failing to strip them is a common and frustrating bug.
    -   **Use-Case**: A `config.env` file contains `API_KEY="key with spaces"`. The makefile uses `load_env config.env` to make `$(API_KEY)` available to the build script with the value `key with spaces`.

## 4. Execution & Dependency Management

-   **Sequential Execution**: The build process is strictly sequential. If a target has multiple dependencies, they are resolved and built one at a time in the order they are listed.
-   **Fail-Fast**: If any command in a recipe fails (returns a non-zero exit code), `make-lite` stops immediately.
    -   **Reasoning**: This prevents a cascade of subsequent errors that depend on the failed step, providing a clear, single point of failure to the user. It is the most common and expected behavior.
-   **Command Echoing & Suppression (`@`)** **[NEW]**:
    -   **Rule**: By default, `make-lite` prints each recipe command to standard output before executing it. If a command line is prefixed with an `@` symbol (and optional whitespace), the command is executed, but it is not printed.
    -   **Reasoning**: Default echoing makes builds transparent and easy to debug. The `@` prefix provides a standard, Make-compatible mechanism to suppress output for cleaner logs, typically used for `echo` statements or other non-critical commands.
-   **Automatic Variable Export** **[NEW]**:
    -   **Rule**: All variables known to `make-lite` (from the makefile, the environment, and `load_env` files) are automatically exported to the environment of any sub-shell it creates (for recipes and `$(shell ...)` expansion).
    -   **Reasoning**: This behavior is simple and predictable. The user does not need to manage an `export` keyword. What you define is what you get in the shell, eliminating a common source of confusion from other build systems.
-   **Freshness Check**: A rule's recipe will execute if:
    1.  **Any** of its target files do not exist.
    2.  OR the modification time of **any** source file is newer than the modification time of **any** target file.
-   **Automatic Directory Creation**: Before executing a recipe, `make-lite` will create the full directory path for each of the rule's targets.
    -   **Reasoning**: This is a major quality-of-life improvement over GNU Make, eliminating the need for boilerplate `mkdir -p $(@D)` commands in nearly every rule.
-   **Refined Directory & Phony Handling**:
    -   **Rule**: A target that is a directory, or a target name that does not correspond to a file (e.g., `clean`, `all`), is treated as "always out of date," causing its rule to always run. A source that is a directory has its modification time (`mtime`) checked.
    -   **Reasoning**: This elegantly solves two problems. It removes the need for `.PHONY` declarations, simplifying the makefile. It also provides an intuitive way to handle source directories: if you add, remove, or rename a file in a source directory, the directory's `mtime` is updated, correctly triggering a rebuild of anything that depends on it.
    -   **Use-Case 1 (Phony)**: A `clean:` rule will always run when `make-lite clean` is invoked.
    -   **Use-Case 2 (Source Directory)**: A rule `app: src/` will rebuild `app` whenever the contents of the `src/` directory change.

## 5. Command Line Interface (CLI)

-   **Default Makefile**: `Makefile.mk-lite`
-   **Default Target**: The first rule defined in the makefile.
-   **Usage**: `make-lite [options] [target_name]`
-   **Flags**:
    -   `--help`, `-h`: Display help message.
    -   `--version`, `-v`: Display program version.
