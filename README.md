# make-lite

`make-lite` is a simple, modern build automation tool written in Go. It aims to provide the power and core concepts of GNU Make while eliminating its most common frustrations and idio-syn-crazies.

## Features

- **Simple, Indentation-Insensitive Syntax**: Rules are separated by empty lines, and commands don't require tabs.
- **Intuitive Dependency Management**:
    - `a b: c d`: A single rule that correctly rebuilds if *any* target is missing or older than *any* source. Ideal for modern tools that generate multiple files (`protoc`, etc.).
    - Implicit "phony" targets: Any target that isn't a file (like `clean`) just works.
    - Directories as dependencies are treated as always out-of-date, forcing rebuilds.
- **Automatic Directory Creation**: Automatically runs `mkdir -p` for your targets so you don't have to.
- **Powerful Variable System**:
    - `VAR = value` (unconditional) and `VAR ?= value` (conditional).
    - Precedence: Makefile (`=`) > Shell environment >  `load_env` files > Makefile (`?=`).
    - `$(shell ...)` for dynamic command output, which can access the parent shell's environment (`$HOME`, etc.).
- **Smart Environment Handling**:
    - `load_env .env`: A dedicated directive to load environment files (and strip quotes from values!).
    - Makefile variables are automatically available to rule commands.
- **Line Continuations (`\`)** and **Comments (`#`)** are supported.
- **Command Echoing Control**: Commands are printed by default. Prefix with `@` to suppress printing for cleaner output.

## Dogfooding: Building `make-lite` with `make-lite`

This project uses its own `Makefile.mk-lite` for its development lifecycle.

**Common Commands:**

-   `make-lite build`: Compiles the `make-lite` binary from source.
-   `make-lite install`: Builds and copies the binary to `~/.local/bin`.
-   `make-lite test`: Runs the automated test suite.
-   `make-lite clean`: Removes build artifacts and cleans Go caches.

### `direnv` Setup for Development

For a seamless development experience where you can type `make` instead of `make-lite`:

1.  [Install `direnv`](https://direnv.net/docs/installation.html).
2.  Create a `bin` directory in the project root.
3.  Install `make-lite` to your local bin: `make-lite install`
4.  Symlink it into the project's `bin` dir: `ln -s ~/.local/bin/make-lite bin/make`
5.  Create a `.envrc` file in the project root with the content: `PATH_add ./bin`
6.  Run `direnv allow`.

Now, `direnv` will automatically add `./bin` to your `PATH` when you are in the project directory, and any calls to `make` will execute your compiled `make-lite`.

## Testing

The project includes a comprehensive test suite written in Python.

-   **Location**: `test_suite/`
-   **Runner**: `test_suite/run_tests.py`
-   **Test Cases**: `test_suite/test_cases/*.json`

To run the tests, simply execute `make-lite test` or `python3 test_suite/run_tests.py` from the project root. The runner will compile a fresh test binary and execute all test cases defined in the `test_cases` directory.
