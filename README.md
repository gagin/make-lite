# make-lite

A simple, predictable build tool that fixes the most common annoyances of GNU Make.

---

## Philosophy

`make-lite` is a macro and command automation tool born from the practical need to solve the most common and frustrating aspects of traditional Make. It is designed to be a powerful command runner that prioritizes simplicity, predictability, and a great developer experience over supporting every esoteric feature.

If you love the core dependency-graph concept of Make but are tired of `.PHONY`, tab errors, and confusing variable expansion rules, `make-lite` is for you.

### Core Features

`make-lite` directly addresses the most common and frustrating aspects of GNU Make with a set of simple, predictable design choices.

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
    When using `load_env .env`, `make-lite` automatically strips surrounding quotes (`"` or `'`) from the values. This is the behavior users almost always want and expect, but have to handle manually in shell scripts.

## How It Works (The Specification)

#### 1. Makefile Structure

-   **Rules**: A non-indented line with a colon (`:`) defines a rule (e.g., `target: dep1 dep2`).
-   **Recipes**: A line is part of a rule's recipe **if and only if it is indented**. The recipe consists of the contiguous block of indented lines immediately following a rule. It is terminated by the first non-indented line or the end of the file.
-   **Comments**: A line is a comment if it starts with an unescaped `#`.

#### 2. Variables & Expansion

-   **Assignments**:
    -   `VAR = value`: Unconditional assignment.
    -   `VAR ?= value`: Conditional assignment (only sets if `VAR` is not already defined).
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

#### 3. Dependency Management

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
-   **Variable Precedence**: In `make-lite`, a Makefile assignment (`=`) **always** wins over an environment variable. Use `?=` in your Makefile to allow environment variables to take precedence.
-   **Unsupported Functions**: Complex GNU Make functions are not supported and will cause a fatal error. This includes `patsubst`, `foreach`, `if`, `call`, etc. These must be rewritten using `$(shell ...)` or simpler logic.
-   **No Automatic Variables**: Special variables like `$@` (target), `$<` (first dependency), and `$^` (all dependencies) are not supported. You must use the explicit names in your recipes.
-   **No Command-Line Variable Overrides**: The `make VAR=value` syntax is not supported. Use environment variables instead (`VAR=value make-lite`).
-   **No `-e` / `--environment-overrides` flag**.

## Automated Conversion with an LLM

You can use a capable LLM (like GPT-4, Claude 3, etc.) to automate much of the conversion process. Use the following prompt as a template.

---

**Prompt for LLM-based Makefile Conversion**

```
You are an expert build system engineer specializing in migrating projects from GNU Make to simpler, more modern alternatives. Your task is to analyze the provided GNU Makefile and convert it into the `make-lite` format.

First, understand the core principles of `make-lite`, which differ from GNU Make:
-   **Premise:** `make-lite` is a simple, predictable command runner that fixes common Make annoyances. It is a single-pass, sequential interpreter.
-   **What is Supported:** Basic rules (`target: deps`), `VAR = value`, `VAR ?= value`, `$(shell ...)` and implicit shell fallbacks `$(command)`, multi-target rules, `$$` for shell passthrough, `load_env`, `include`.
-   **What is NOT Supported:** Deferred assignment `:=`, `.DEFAULT_GOAL`, automatic variables (`$@`, `$<`, `$^`), complex functions (`patsubst`, `foreach`, `wildcard`, etc.), command-line variable overrides (`make VAR=value`).

Follow these conversion rules precisely:

**1. Sequential Parsing & File Structure:**
-   **CRITICAL: Reorder Variable Definitions.** Because `make-lite` is a single-pass interpreter, you must ensure that all variables are defined **before** they are used in rule targets or dependencies. If necessary, move variable definition blocks to the top of the file.
-   **Root Makefile:** The main file must be named `Makefile.mk-lite`.
-   **Default Target:** Remove any `.DEFAULT_GOAL` directive. The default target in `make-lite` is simply the first rule in the root `Makefile.mk-lite`. By convention, this should be `all: help` if a `help` target exists.

**2. Simplify Directives & Syntax:**
-   **Indentation:** Ensure every recipe line is indented. Any whitespace (tabs or spaces) is acceptable.
-   **Environment Files:** Replace conditional `include .env` logic (e.g., `ifneq (,$(wildcard ./.env))`) with a single `load_env .env` directive.
-   **Assignments:** Change `:=` to `=`.
-   **Recursive Calls:** Replace `$(MAKE)` or `make` with `make-lite`.

**3. Remove Boilerplate & GNU Make Workarounds:**
-   Aggressively simplify common workarounds for GNU Make's limitations, as `make-lite` often makes them obsolete.
-   **Unconditional Generation:** Find and remove any `-force` targets (e.g., `protos-force`). `make-lite`'s dependency tracking is robust enough to not need them.
-   **Stamp/Sentinel Files:** Find and remove rules that use empty "stamp" files (e.g., `touch .some_task_complete`) merely to trigger other rules.
-   **Merge Split Multi-Target Rules:** GNU Make sometimes struggles with multi-target rules, so users split them (e.g., one rule for `file.pb.go` and another for `file_grpc.pb.go`). Identify these and **merge them back into a single, clean multi-target rule**.
-   **Remove `.PHONY`**, `mkdir -p` (when creating a target's parent directory), and the `export` keyword.

**4. Convert Functions & Variables:**
-   **Automatic Variables**: Replace `$@` (target), `$<` (first dependency), and `$^` (all dependencies) with their explicit string values.
-   **Unsupported Functions**: Rewrite complex GNU Make functions (`patsubst`, `wildcard`, etc.) using `$(shell ...)` with common shell commands like `find` or `sed`. If a direct conversion is not possible, add a `# TODO:` comment explaining that the function needs manual review.

Convert the following GNU Makefile to `make-lite` format.

**GNU Makefile Input:**
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
-   **Debugging**: Set `LOG_LEVEL=DEBUG` to see verbose output, including the exact commands being sent to the shell.

```bash
LOG_LEVEL=DEBUG make-lite
```
