#!/usr/bin/env python3

import os
import sys
import json
import tempfile
import subprocess
import argparse
import glob
import shutil
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
    
    script_dir = Path(__file__).resolve().parent
    project_root = script_dir.parent
    source_dir = project_root / "cmd" / "make-lite"
    binary_path = script_dir / BINARY_NAME

    proc = subprocess.run(
        ["go", "build", "-o", str(binary_path), "."],
        cwd=str(source_dir),
        capture_output=True, text=True
    )
    if proc.returncode != 0:
        print(f"{COLOR_RED}COMPILATION FAILED:{COLOR_RESET}")
        print(proc.stderr)
        sys.exit(1)
    return binary_path

def run_test_case(binary_path, test_case_path, test_dir_base):
    """Runs a single test case from a JSON file."""
    with open(test_case_path, 'r') as f:
        case = json.load(f)

    case_name = case.get("name", Path(test_case_path).name)
    print(f"{COLOR_BLUE}Running Test:{COLOR_RESET} {case_name}", end=' ... ', flush=True)

    case_dir = Path(test_dir_base) / Path(test_case_path).stem
    case_dir.mkdir()

    for file_info in case.get("files", []):
        path = case_dir / file_info["path"]
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(file_info["content"] + "\n")

    command = [str(binary_path)]
    if case.get("command"):
        command.extend(case["command"].split())

    env = os.environ.copy()
    env.update(case.get("env_vars", {}))
    
    # Pass LOG_LEVEL from the runner's environment to the test environment
    if "LOG_LEVEL" in os.environ:
        env["LOG_LEVEL"] = os.environ["LOG_LEVEL"]
    
    proc = subprocess.run(
        command,
        cwd=case_dir,
        capture_output=True,
        text=True,
        env=env
    )
    
    checks = case.get("checks", {})
    errors = []

    if "exit_code" in checks and proc.returncode != checks["exit_code"]:
        errors.append(f"Expected exit code {checks['exit_code']}, got {proc.returncode}.")
    
    output = proc.stdout + proc.stderr
    for s in checks.get("stdout_contains", []):
        if s not in output:
            errors.append(f"Expected output to contain: '{s}'")
    for s in checks.get("stdout_not_contains", []):
        if s in output:
            errors.append(f"Expected output to NOT contain: '{s}'")
    
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
        print(f"  STDOUT:\n---\n{proc.stdout}\n---")
        print(f"  STDERR:\n---\n{proc.stderr}\n---")
        print(f"  Test artifacts are in: {case_dir}")
        return False
    else:
        print(f"{COLOR_GREEN}PASS{COLOR_RESET}")
        shutil.rmtree(case_dir)
        return True

def main():
    parser = argparse.ArgumentParser(description="Test runner for make-lite.")
    script_dir = Path(__file__).resolve().parent
    
    parser.add_argument('test_path', nargs='?', default=str(script_dir / DEFAULT_TEST_DIR),
                        help=f"Path to a specific test case JSON or a directory of tests (default: {DEFAULT_TEST_DIR})")
    args = parser.parse_args()

    binary_path = compile_binary()

    test_path = Path(args.test_path)
    if test_path.is_file():
        test_files = [test_path]
    else:
        test_files = sorted(test_path.glob("*.json"))
    
    if not test_files:
        print(f"{COLOR_RED}No test files found in '{test_path}'.{COLOR_RESET}")
        sys.exit(1)

    test_run_dir = tempfile.mkdtemp(prefix="make-lite-run-")
    print(f"{COLOR_YELLOW}--- Test run master directory: {test_run_dir} ---{COLOR_RESET}")
    
    results = [
        run_test_case(binary_path, test_file, test_run_dir)
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
        print(f"Failed test case directories were not deleted from {test_run_dir}")
        sys.exit(1)
    else:
        shutil.rmtree(test_run_dir)

if __name__ == "__main__":
    main()
