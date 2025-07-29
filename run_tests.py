#!/usr/bin/env python3

import os
import sys
import json
import tempfile
import subprocess
import argparse
import glob
from pathlib import Path

# --- Configuration ---
BINARY_NAME = "make-lite-test"
DEFAULT_TEST_DIR = "test_cases"

# --- Colors ---
COLOR_GREEN = '\033[92m'
COLOR_RED = '\033[91m'
COLOR_YELLOW = '\033[93m'
COLOR_BLUE = '\033[94m'
COLOR_RESET = '\033[0m'

def compile_binary():
    """Compiles the Go source code."""
    print(f"{COLOR_YELLOW}--- Building make-lite for testing... ---{COLOR_RESET}")
    proc = subprocess.run(
        ["go", "build", "-o", BINARY_NAME, "./main.go"],
        capture_output=True, text=True
    )
    if proc.returncode != 0:
        print(f"{COLOR_RED}COMPILATION FAILED:{COLOR_RESET}")
        print(proc.stderr)
        sys.exit(1)
    return Path(BINARY_NAME).resolve()

def run_test_case(binary_path, test_case_path, test_dir_base):
    """Runs a single test case from a JSON file."""
    with open(test_case_path, 'r') as f:
        case = json.load(f)

    case_name = case.get("name", Path(test_case_path).name)
    print(f"{COLOR_BLUE}Running Test:{COLOR_RESET} {case_name}", end=' ... ', flush=True)

    # --- Setup test environment for this case ---
    case_dir = Path(test_dir_base) / Path(test_case_path).stem
    case_dir.mkdir()

    for file_info in case.get("files", []):
        path = case_dir / file_info["path"]
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(file_info["content"] + "\n")

    # --- Prepare and run command ---
    command = [str(binary_path)]
    if case.get("command"):
        command.extend(case["command"].split())

    env = os.environ.copy()
    env.update(case.get("env_vars", {}))
    
    proc = subprocess.run(
        command,
        cwd=case_dir,
        capture_output=True,
        text=True,
        env=env
    )
    
    # --- Check results ---
    checks = case.get("checks", {})
    errors = []

    # Check 1: Exit code
    if "exit_code" in checks and proc.returncode != checks["exit_code"]:
        errors.append(f"Expected exit code {checks['exit_code']}, got {proc.returncode}.")
        errors.append(f"  STDOUT:\n{proc.stdout}")
        errors.append(f"  STDERR:\n{proc.stderr}")

    # Check 2: stdout/stderr contents
    output = proc.stdout + proc.stderr
    for s in checks.get("stdout_contains", []):
        if s not in output:
            errors.append(f"Expected output to contain: '{s}'")
    for s in checks.get("stdout_not_contains", []):
        if s in output:
            errors.append(f"Expected output to NOT contain: '{s}'")
    
    # Check 3: File existence
    for f in checks.get("files_exist", []):
        if not (case_dir / f).exists():
            errors.append(f"Expected file to exist: {f}")
    for f in checks.get("files_not_exist", []):
        if (case_dir / f).exists():
            errors.append(f"Expected file to NOT exist: {f}")

    if errors:
        print(f"{COLOR_RED}FAIL{COLOR_RESET}")
        for error in errors:
            print(f"  - {error}")
        return False
    else:
        print(f"{COLOR_GREEN}PASS{COLOR_RESET}")
        return True

def main():
    parser = argparse.ArgumentParser(description="Test runner for make-lite.")
    parser.add_argument('test_path', nargs='?', default=DEFAULT_TEST_DIR,
                        help=f"Path to a specific test case JSON or a directory of tests (default: {DEFAULT_TEST_DIR})")
    args = parser.parse_args()

    binary_path = compile_binary()

    if Path(args.test_path).is_file():
        test_files = [args.test_path]
    else:
        test_files = sorted(glob.glob(f"{args.test_path}/*.json"))
    
    if not test_files:
        print(f"{COLOR_RED}No test files found in '{args.test_path}'.{COLOR_RESET}")
        sys.exit(1)

    with tempfile.TemporaryDirectory(prefix="make-lite-tests-") as temp_dir:
        print(f"{COLOR_YELLOW}--- Test artifacts will be in {temp_dir} ---{COLOR_RESET}")
        
        results = [
            run_test_case(binary_path, test_file, temp_dir)
            for test_file in test_files
        ]

        passed = sum(1 for r in results if r)
        failed = len(results) - passed
        
        print("\n==================== TEST SUMMARY ====================")
        print(f"Total Tests: {len(results)}")
        print(f"{COLOR_GREEN}Passed: {passed}{COLOR_RESET}")
        print(f"{COLOR_RED}Failed: {failed}{COLOR_RESET}")
        print("======================================================")

        if failed > 0:
            sys.exit(1)

if __name__ == "__main__":
    main()
