# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] - 2024-07-29

### Added

-   **Performance:** Implemented caching for the fully expanded environment used by sub-processes. This prevents redundant and expensive variable expansions for every recipe command and shell call, dramatically improving performance on Makefiles that use `$(shell ...)` in variables.

### Changed

-   **BREAKING CHANGE:** Variable assignment is now **eagerly expanded**, aligning with the "Simple, Eager Expansion" philosophy. The right-hand side of a variable assignment (e.g., `VAR = $(shell date)`) is expanded *once* at the moment it is defined, and the resulting literal string is stored. This is equivalent to GNU Make's `:=` operator and replaces the previous deferred (`=`) behavior.
-   **BREAKING CHANGE:** The environment variable to enable debug logging has been changed from `LOG_LEVEL=DEBUG` to `MAKE_LITE_LOG_LEVEL=DEBUG`. This prevents a project's own `LOG_LEVEL` setting in an `.env` file from unintentionally activating debug mode in recursive `make-lite` calls.
-   **Usability:** `make-lite` is now silent on success by default, following standard command-line tool philosophy. The "Building target..." and "Build finished successfully" messages will now only appear when `MAKE_LITE_LOG_LEVEL=DEBUG` is set.
-   **Usability:** The "Targets ... are up to date" message is now only shown in debug mode. It has also been improved to report on the entire group of targets in a multi-target rule, rather than just the first one.
-   **Debugging:** The debug output for shell cache hits has been removed to reduce noise. The absence of a "DEBUG: executing shell command" message is sufficient to indicate a cache hit.

### Fixed

-   Fixed a critical bug where variables with `$(shell ...)` were expanded "late" (deferred expansion). This caused the shell command to run when the variable was *used* in a recipe, not when it was *defined*. The expansion is now "eager," ensuring the shell command runs once at definition time. This allows variables to capture the state of the filesystem at a specific point in the parsing process, before any recipes have run.
-   Fixed a regression where eagerly expanded variables containing backslashes were being incorrectly processed for escapes a second time during recipe expansion, leading to corrupted shell commands.

## [1.0.0] - 2024-07-29

This marks the first stable, feature-complete release of `make-lite`. The core parsing and execution engines were rewritten from the ground up based on an improved specification to be more robust, predictable, and compatible with common Make patterns.

### Added

-   **Indentation-Based Recipes:** The parser now uses indentation (any mix of spaces or tabs) to define recipe blocks, completely fixing GNU Make's "tabs vs. spaces" problem.
-   **GNU Make Expansion Compatibility:** The `$(...)` operator now correctly implements the standard precedence: `$(shell ...)` is evaluated first, then defined variables, with a fallback to executing the content as an implicit shell command.
-   **Unsupported Function Detection:** The expander now detects common but unsupported GNU Make functions (e.g., `patsubst`, `foreach`) and exits with a clear error message.
-   **Missing Dependency Errors:** The engine will now fail immediately with a fatal error if a source file is missing and there is no rule to create it.
-   **Version Injection:** Added support for injecting the application version at build time using Go's `ldflags`.
-   **Comprehensive Test Suite:** Added numerous new test cases to cover indentation rules, implicit shell commands, dependency resolution, error handling, and debug output.

### Changed

-   **Core Engine Rewrite:** The parsing, variable expansion, and dependency-checking engines were completely rewritten for stability and to align with the refined specification.
-   **Error Reporting:** Error messages for failed recipes are now specific and follow the professional `recipe for target '...' failed` format, making debugging easier.
-   **Documentation:** The `PRD.md` and `README.md` were overhauled to reflect the final, stable design of the tool, including a robust LLM prompt for migrating from GNU Make.

### Fixed

-   Fixed a critical bug preventing multi-target rules from working correctly, ensuring all targets are checked and built as a single unit.
-   Fixed a bug where variables that expand to a space-separated list of files were not handled correctly in dependency lists.
-   Fixed a bug where variables used in rule definitions (e.g., `$(MY_TARGET): ...`) were not being expanded at parse time.
-   Fixed numerous subtle bugs related to backslash escaping in variables and shell commands.
