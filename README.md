# make-lite

A simple, predictable build tool that fixes the most common annoyances of GNU Make.

---

## Philosophy

`make-lite` is a macro and command automation tool born from the practical need to solve the most common and frustrating aspects of traditional Make. It is designed to be a powerful command runner that prioritizes simplicity, predictability, and a great developer experience over supporting every esoteric feature.

If you love the core dependency-graph concept of Make but are tired of `.PHONY`, tab errors, and confusing variable expansion rules, `make-lite` is for you.

## Core Features

-   **Intuitive Dependency Rules (The Core Fix)**  
    This is the most important feature. A rule runs if **any** of its target files are missing, or if **any** source file is newer than **any** target file. This elegant logic solves multiple GNU Make frustrations at once:
    -   **Multi-target rules just work.** A rule like `file_pb.go file_grpc.pb.go: file.proto` will correctly re-run if *either* generated file is deleted.
    -   **Code generators are handled perfectly.** `protoc` creating two files from one source is no longer a problem.

-   **Flexible Recipe Indentation**  
    While GNU Make's principle that recipes must be indented is sound, its implementation is famously brittle. `make-lite` fixes this:
    -   **Any indentation (spaces or tabs) is valid.** This completely eliminates the strict 'tab-only' requirement and the infamous 'missing separator' errors it causes, embracing the philosophy that indentation is for human readability, and humans should be free to choose their style.

-   **Implicit & Non-Infectious Phony Targets**  
    Any target that doesn't correspond to a file on disk is automatically treated as "phony" (it will always run). This has two major benefits:
    -   It completely eliminates the need for `.PHONY` boilerplate.
    -   It fixes the confusing GNU Make behavior where a phony target can cause its file-based dependencies to be rebuilt unnecessarily.

-   **Automatic Directory Creation**  
    If a rule's target is in a directory that doesn't exist (e.g., `bin/my_app`), `make-lite` will create the parent directory (`bin/`) automatically before running the recipe. No more `mkdir -p` boilerplate.

-   **Practical `.env` Parsing**  
    When using `load_env .env`, `make-lite` uses a practical parsing approach that automatically strips surrounding quotes (`"` or `'`) from values, which is the behavior users almost always want.

## Core Principles & Behavior

`make-lite` is designed to be simple and predictable. It achieves this by adhering to a few core principles that differ from GNU Make in key ways. Understanding these principles is essential for using the tool effectively.

### 1. Two-Pass Parser & "Last Definition Wins"

`make-lite` uses a strict **two-pass parser** to ensure predictable behavior.

*   **Pass 1: Variable & Rule Collection:** `make-lite` first reads *all* `include`d Makefiles from top to bottom and populates its variable store. If a variable is defined multiple times, the **last definition wins**. This process is completed before any rules are evaluated.
*   **Pass 2: Rule Expansion:** After the variable store is complete, `make-lite` expands the variables within the targets and dependencies of each rule.

This means the order of variable definitions matters, but the location of a rule relative to its variable definitions does not.

**Example:**
```makefile
# File: Makefile.mk-lite
VAR = first
include extra.mk
all:
	@echo "The value is: $(VAR)"
```
```makefile
# File: extra.mk
VAR = last
```
Running `make-lite` will output `The value is: last`, because the definition in `extra.mk` was the last one processed during the first pass.

### 2. Eager Variable Expansion (like `:=`)

`make-lite` uses **eager expansion** for all standard variable assignments (`=`). The right-hand side of an assignment is evaluated *once*, at the moment it is defined during the first parsing pass. The resulting literal string is then stored.

This is equivalent to GNU Make's `:=` operator and is a core part of `make-lite`'s "simple and predictable" philosophy. It means a variable's value is fixed before any recipes are run and will not change during execution.

**Example:**
```makefile
# The `shell date` command is run only once, during parsing.
TIMESTAMP = $(shell date)

all:
	@sleep 2
	@echo "Timestamp from parse time: $(TIMESTAMP)"
	@echo "Timestamp from execution time: $(shell date)"
```
The output will show two different timestamps, demonstrating that `$(TIMESTAMP)` stored the value from when the file was first parsed, not when the recipe was executed.

### 3. Clear Warnings and Precise Errors

To make debugging easy and prevent silent "action at a distance" errors, `make-lite` provides clear feedback:

*   **Variable Redefinition Warnings:** If you define a variable that has already been defined in another makefile context, `make-lite` will print a clear warning to `stderr`, showing you the exact file and line number of both the new and previous definitions. This helps you track down unintended overrides immediately.

    ```
    make-lite: Warning: variable 'VAR' redefined at extra.mk:2. Previous definition at Makefile.mk-lite:1. The last definition will be used.
    ```

*   **Precise Error Locations:** All parsing errors point to the exact `file:line` where the error occurred, so you never have to guess.

This combination of a predictable parsing model and clear, precise feedback makes `make-lite` robust and easy to maintain.

## How It Works (The Specification)

#### 1. Makefile Structure

-   **Rules**: A non-indented line with a colon (`:`) defines a rule (e.g., `target: dep1 dep2`).
-   **Recipes**: A line is part of a rule's recipe **if and only if it is indented**. The recipe consists of the contiguous block of indented lines immediately following a rule. It is terminated by the first non-indented line or the end of the file.
-   **Comments**: A line is a comment if it starts with an unescaped `#`.

#### 2. Variables & Expansion

-   **Assignments**:
    -   `VAR = value`: Unconditional assignment.
    -   `VAR ?= value`: Conditional assignment (only sets if `VAR` is not already defined).
-   **Expansion Model: Eager by Default**:
    `make-lite` has a single, simple expansion model: all variable assignments are expanded **eagerly** at the time they are parsed. The right-hand side is fully resolved (including any `$(shell ...)` calls), and the resulting literal string is stored. This is equivalent to GNU Make's `:=` operator and ensures a variable's value is fixed and predictable throughout the build.
-   **Precedence (Highest to Lowest)**:
    1.  **Makefile Unconditional (`=`)**: Has the final say.
    2.  **Environment Variables**: Includes variables from `export` or command-line prefixes (e.g., `VAR=val make-lite`).
    3.  **Makefile Conditional (`?=`)**: Use this to provide a default that can be overridden by the environment.
-   **Expansion Syntax**:
    -   `$(...)`: The primary expansion form.
    -   `$VAR`: A shell-style convenience form for simple variables.
-   **Escaping Special Characters**:
    -   **Backslash (`\`):** Use a backslash to escape the next character from `make-lite`'s parser. This is for passing literal characters like `$`, `#`, `(`, `)`, `:`, `=`, or `\` itself to the value of a variable or a recipe. Example: `GREETING = echo Hello \#world` sets the variable's value to `echo Hello #world`.
    -   **Double Dollar (`$$`):** Use a double dollar sign to pass a single literal `$` to the shell. This is the primary mechanism for using shell variables (`$$PATH`) or shell command substitution (`LATEST_COMMIT=$$(git rev-parse HEAD)`) inside a recipe.
-   **Expansion Precedence within `$(...)`**:
    1.  **`$(shell command)`**: Explicitly runs `command` in a sub-shell and substitutes its output.
    2.  **`$(VAR)`**: If `VAR` is a defined `make-lite` variable, it is expanded.
    3.  **`$(command)`**: If `command` is *not* a defined `make-lite` variable, it is treated as an implicit shell command, executed, and its output is substituted.

#### 3. Recursive Calls & The Environment

When `make-lite` is called from within a recipe (e.g., `make-lite clean`), it is a new process. This new process inherits its environment from the recipe's shell, **not** from the original `make-lite` process that launched the recipe.

This means that any variables modified within the recipe's command line will be seen by the recursive call.

**Example:**
```makefile
VAR = original
all:
	@echo "Top level sees: $(VAR)"
	VAR=changed make-lite inner

inner:
	@echo "Inner level sees: $(VAR)"
```
Output:
```
Top level sees: original
Inner level sees: changed
```
This is the correct and expected behavior, but it's important to be aware of when writing complex recursive Makefiles.

#### 4. Dependency Management

-   A rule's recipe runs if **any** of its targets don't exist, or if **any** of its sources are newer than the **oldest** target.
-   If a dependency is missing from the filesystem and there is no rule to create it, `make-lite` exits with a fatal error.

## Example: Building `make-lite` with `make-lite`

This project "eats its own dog food." Here is the actual `Makefile.mk-lite` used to build this tool, which serves as a great example of best practices.

```makefile
# ==============================================================================
# Main Makefile for the make-lite project.
#
# This file uses make-lite itself to build, test, and install the tool.
# ==============================================================================

# --- Variables ---
# The name of the final executable.
BINARY_NAME = make-lite
# The path to the main Go package.
CMD_PATH = ./cmd/make-lite
# The directory where the binary will be installed.
INSTALL_DIR ?= $(shell echo $HOME)/.local/bin

# Versioning
# Get the version string from git tags. Fallback to "dev" if not in a git repo.
APP_VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
# Go linker flags to inject the version string into the binary.
LDFLAGS = -ldflags="-X main.AppVersion=$(APP_VERSION)"

# Test suite runner
TEST_RUNNER = ./test_suite/run_tests.py
# Find all Go source files to use as dependencies for the build.
GO_SOURCES = $(shell find $(CMD_PATH) -name '*.go')

# --- Default Target ---
# The default target is `build`. Running `make` or `make-lite` will build the binary.
all: build

# --- Main Targets ---

# Build the make-lite binary with the version injected.
build: $(GO_SOURCES)
	@echo "Tidying modules..."
	go mod tidy
	@echo "Building $(BINARY_NAME) version $(APP_VERSION)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) $(CMD_PATH)

# Install the make-lite binary to the user's local bin directory.
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY_NAME) $(INSTALL_DIR)/

# Run the Python test suite.
test:
	@echo "Running test suite..."
	python3 $(TEST_RUNNER)

# Clean build artifacts and Go caches.
clean:
	@echo "Cleaning artifacts and caches..."
	rm -f $(BINARY_NAME)
	rm -f ./test_suite/make-lite-test
	go clean -cache -modcache -testcache
```

## Seamless Workflow with `direnv`

You can use `direnv` to automatically shadow the system's `make` command with `make-lite` whenever you are in this project's directory. This provides a completely seamless workflow.

**1. Create the `.envrc` file:**
Create a file named `.envrc` in the project root with the following content. This tells `direnv` to add a local `mk-lite` directory to the front of your `PATH`.
```sh
# .envrc
#
# Use this with `direnv` to shadow the system 'make' with 'make-lite'.
# This prepends the ./mk-lite directory to your PATH.
PATH_add ./mk-lite
```

**2. Create the symbolic link:**
Create a symbolic link named `make` inside the `mk-lite` directory that points to your `make-lite` executable. This is the "executable" that `direnv` will find in the path.
```bash
# Ensure the directory exists
mkdir -p mk-lite

# Create the symlink (adjust path if you installed make-lite elsewhere)
ln -sf ~/.local/bin/make-lite mk-lite/make
```

**3. Allow `direnv`:**
Finally, run `direnv allow` in your terminal. Now, any time you type `make`, the shell will find and execute your local `make-lite` symlink instead of the system `make`.

## Migrating from GNU Make

Migrating simple Makefiles is straightforward.

#### What Works as Expected

-   Basic `target: dependency` rules.
-   `VAR = value` and `VAR ?= value` assignments.
-   Recipe commands prefixed with `@` for suppression.
-   `$(shell command)` and implicit `$(command)` expansion.
-   `$$` for passing a literal `$` to the shell.

#### Key Differences & Unsupported Features

-   **Flexible Recipe Indentation**: Like GNU Make, recipe lines must be indented to be part of a recipe. The crucial difference is that `make-lite` allows **any whitespace indentation (one or more spaces or tabs)**. This completely eliminates GNU Make's strict 'tab-only' rule and the infamous 'missing separator' errors it causes.
-   **No Deferred Expansion (`=`):** `make-lite`'s `=` operator is always **eagerly expanded** (like GNU Make's `:=`). There is no equivalent to GNU Make's deferred (`=`) operator or target-specific variables. This is a deliberate design choice to eliminate the complexity and "workaround" feel of variables that change their value depending on the context. Recipes should be explicit and self-contained.
-   **Variable Precedence**: In `make-lite`, a Makefile assignment (`=`) **always** wins over an environment variable. Use `?=` in your Makefile to allow environment variables to take precedence.
-   **Unsupported Functions**: Complex GNU Make functions are not supported and will cause a fatal error. This includes `patsubst`, `foreach`, `if`, `call`, etc. These must be rewritten using `$(shell ...)` or simpler logic.
-   **No Automatic Variables**: Special variables like `$@` (target), `$<` (first dependency), and `$^` (all dependencies) are not supported. You must use the explicit names in your recipes.
-   **No Command-Line Variable Overrides**: The `make VAR=value` syntax is not supported. Use environment variables instead (`VAR=value make-lite`).
-   **No `-e` / `--environment-overrides` flag**.

## Automated Conversion with an LLM

You can use a capable LLM to automate much of the conversion process.

### Prompt for Migrating from GNU Make

```
You are an expert build system engineer specializing in migrating projects from GNU Make to simpler, more modern alternatives. Your task is to analyze the provided GNU Makefile and convert it into the `make-lite` format.

First, understand the core principles of `make-lite`, which differ from GNU Make:
-   **Premise:** `make-lite` is a simple, predictable command runner that fixes common Make annoyances.
-   **Parsing Model:** `make-lite` uses a two-pass parser. In the first pass, it reads all files and populates all variables. If a variable is defined multiple times, the last definition wins. In the second pass, it expands variables in rules. This means you do not need to manually reorder variable definitions to appear before their use.
-   **What is Supported:** Basic rules (`target: deps`), `VAR = value`, `VAR ?= value`, `$(shell ...)` and implicit shell fallbacks `$(command)`, multi-target rules, `$$` for shell passthrough, `load_env`, `include`.
-   **What is NOT Supported:** Deferred assignment (the `=` operator is always eagerly expanded like `:=`), `.DEFAULT_GOAL`, automatic variables (`$@`, `$<`, `$^`), complex functions (`patsubst`, `foreach`, `wildcard`, etc.), command-line variable overrides (`make VAR=value`).

Follow these conversion rules precisely:

**1. File Structure & Simplification:**
-   **Root Makefile:** The main file must be named `Makefile.mk-lite`.
-   **Default Target:** Remove any `.DEFAULT_GOAL` directive. The default target in `make-lite` is simply the first rule in the root `Makefile.mk-lite`. By convention, this should be `all: help` if a `help` target exists.
-   **Indentation:** Ensure every recipe line is indented. Any whitespace (tabs or spaces) is acceptable.
-   **Environment Files:** Replace conditional `include .env` logic (e.g., `ifneq (,$(wildcard ./.env))`) with a single `load_env .env` directive.
-   **Assignments:** Convert both GNU Make's simple `:=` and deferred `=` assignments to `make-lite`'s standard `=` operator. Because `make-lite` uses eager expansion, you may need to refactor rules that depend on deferred expansion.
-   **Recursive Calls:** Replace `$(MAKE)` or `make` with `make-lite`.

**2. Remove Boilerplate & GNU Make Workarounds:**
-   Aggressively simplify common workarounds for GNU Make's limitations, as `make-lite` often makes them obsolete.
-   **Unconditional Generation:** Find and remove any `-force` targets (e.g., `protos-force`). `make-lite`'s dependency tracking is robust enough to not need them.
-   **Stamp/Sentinel Files:** Find and remove rules that use empty "stamp" files (e.g., `touch .some_task_complete`) merely to trigger other rules.
-   **Merge Split Multi-Target Rules:** GNU Make sometimes struggles with multi-target rules, so users split them (e.g., one rule for `file.pb.go` and another for `file_grpc.pb.go`). Identify these and **merge them back into a single, clean multi-target rule**.
-   **Remove `.PHONY`**, `mkdir -p` (when creating a target's parent directory), and the `export` keyword.

**3. Convert Functions & Variables:**
-   **Automatic Variables**: Replace `$@` (target), `$<` (first dependency), and `$^` (all dependencies) with their explicit string values.
-   **Unsupported Functions**: Rewrite complex GNU Make functions (`patsubst`, `wildcard`, etc.) using `$(shell ...)` with common shell commands like `find` or `sed`. If a direct conversion is not possible, add a `# TODO:` comment explaining that the function needs manual review.

Convert the following GNU Makefile to `make-lite` format.

**GNU Makefile Input:**
```

### Prompt for Generating New Makefiles

```
If you need to generate a Makefile, use the `make-lite` format. The file must be named `Makefile.mk-lite`. Follow these rules:

*   **Syntax:** Use `target: dependency` for rules. Recipe commands must be indented with one or more spaces or tabs.

*   **Variables:** All variables (`VAR = value`) are expanded **eagerly** before any rules run. A `$(shell ...)` command in a variable assignment runs only once, during parsing. Use `VAR ?= value` to set a default that can be overridden by the environment.

*   **CRITICAL: No Automatic Variables:** Do not use automatic variables like `$@`, `$<`, or `$^`, or commands like `foreach`. You must write out the explicit target and dependency names in your recipes.

*   **No Boilerplate:** Do not add GNU Make boilerplate or workarounds. `make-lite` handles them automatically:
    *   Do not use `.PHONY` targets.
    *   Do not use `mkdir -p` for a target's parent directory.
    *   Do not use stamp files. A rule with multiple targets runs if *any* target is missing or outdated.

*   **File Lists:** Use `VAR = $(shell find ...)` to gather source file lists.
```

---

## Installation

You can build from source or, once available, download a pre-compiled binary from the releases page.

```bash
# Build from source
make-lite build

# Install to a local directory (e.g., ~/.local/bin)
make-lite install
```

## Usage

```
Usage: make-lite [options] [target]

A simple, predictable build tool inspired by Make.

Options:
  -h, --help      Display help message.
  -v, --version   Display program version.
```

-   **Default Makefile**: `Makefile.mk-lite`
-   **Default Target**: The first rule defined in the Makefile.
-   **Debugging**: Set the environment variable `MAKE_LITE_LOG_LEVEL=DEBUG` to see verbose output, including the exact commands being sent to the shell.

```bash
MAKE_LITE_LOG_LEVEL=DEBUG make-lite
```
