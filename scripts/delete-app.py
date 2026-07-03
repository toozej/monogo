#!/usr/bin/env python3
"""Remove an app from the monogo repository and clean references."""

from __future__ import annotations

import argparse
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent


class DeleteAppError(RuntimeError):
    """Raised when application removal fails."""


def require_cmd(name: str) -> None:
    if shutil.which(name) is None:
        raise DeleteAppError(f"{name} is required")


def parse_args(argv: list[str]) -> str:
    parser = argparse.ArgumentParser(
        prog="delete-app",
        add_help=False,
        description=(
            "Remove an app directory under apps/<name> and update shared configs "
            "(CI matrix, Dependabot)."
        ),
    )
    parser.add_argument("app_name", nargs="?", help="Name of the app to delete")
    parser.add_argument(
        "-h", "--help", action="help", help="Show this help message and exit"
    )

    args = parser.parse_args(argv)
    app_name = args.app_name or os.getenv("APP", "")
    if not app_name:
        parser.error("APP name is required (pass as argument or set APP env var)")
    return app_name


def validate_app_name(app_name: str) -> None:
    if app_name in {"", ".", ".."}:
        raise DeleteAppError(f"APP must be a single-segment name, got '{app_name}'")
    if "/" in app_name:
        raise DeleteAppError(f"APP must not contain '/', got '{app_name}'")
    if not re.fullmatch(r"[A-Za-z0-9._-]+", app_name):
        raise DeleteAppError(
            f"APP must be a single-segment name using [A-Za-z0-9._-], got: '{app_name}'"
        )
    if app_name == "monogo":
        raise DeleteAppError("Refusing to delete the starter app 'monogo'")


def require_repo_layout(app_name: str) -> Path:
    apps_dir = ROOT / "apps"
    if not apps_dir.is_dir():
        raise DeleteAppError(f"missing apps/ directory at {apps_dir}")
    target = apps_dir / app_name
    if not target.exists():
        raise DeleteAppError(f"apps/{app_name} does not exist")
    if not target.is_dir():
        raise DeleteAppError(f"apps/{app_name} is not a directory")
    return target


def parse_app_binary(app_dir: Path) -> str | None:
    config = app_dir / "app.yaml"
    if not config.is_file():
        return None
    for raw_line in config.read_text().splitlines():
        line = raw_line.strip()
        if not line.startswith("binary:"):
            continue
        value = line.split(":", 1)[1].strip()
        value = value.strip('"')
        return value or None
    return None


def remove_path(path: Path) -> bool:
    if not path.exists():
        return False
    if path.is_dir() and not path.is_symlink():
        shutil.rmtree(path)
    else:
        path.unlink()
    return True


def remove_optional_artifacts(app_name: str, binary_name: str | None) -> None:
    candidates = [
        ROOT / "docs" / "diagrams" / app_name,
        ROOT / "dist" / app_name,
        ROOT / "profiles" / app_name,
    ]
    if binary_name:
        candidates.extend(
            [
                ROOT / "out" / binary_name,
                ROOT / "manpages" / f"{binary_name}.1.gz",
                ROOT / "completions" / f"{binary_name}.bash",
                ROOT / "completions" / f"{binary_name}.fish",
                ROOT / "completions" / f"{binary_name}.zsh",
            ]
        )
    for path in candidates:
        remove_path(path)


def remove_app_directory(app_dir: Path) -> None:
    shutil.rmtree(app_dir)


def remove_app_from_ci_matrix(app_name: str) -> None:
    ci_path = ROOT / ".github" / "workflows" / "ci.yaml"
    if not ci_path.exists():
        return
    lines = ci_path.read_text().splitlines()
    result: list[str] = []
    i = 0
    modified = False
    pattern = re.compile(r"^(\s*)app:\s*$")

    while i < len(lines):
        line = lines[i]
        match = pattern.match(line)
        if match:
            indent = match.group(1)
            list_indent = indent + "  "
            result.append(line)
            i += 1
            while i < len(lines):
                current = lines[i]
                if re.match(rf"^{re.escape(list_indent)}- ", current):
                    if current.strip() != f"- {app_name}":
                        result.append(current)
                    else:
                        modified = True
                    i += 1
                    continue
                if current.strip() == "":
                    result.append(current)
                    i += 1
                    continue
                break
            continue
        result.append(line)
        i += 1

    if modified:
        ci_path.write_text("\n".join(result).rstrip() + "\n")


def remove_dependabot_entry(app_name: str) -> None:
    dependabot_path = ROOT / ".github" / "dependabot.yml"
    if not dependabot_path.exists():
        return
    text = dependabot_path.read_text()
    marker = f'directory: "/apps/{app_name}"'
    if marker not in text:
        return

    lines = text.splitlines()
    start = None
    indent_pattern = None
    for idx, line in enumerate(lines):
        if marker in line:
            # Walk backwards to find the start of the block
            j = idx
            while j >= 0:
                match = re.match(r"^(\s*)-\s+package-ecosystem:\s+\"docker\"", lines[j])
                if match:
                    start = j
                    indent_pattern = (
                        rf"^{re.escape(match.group(1))}-\s+package-ecosystem:\s+\""
                    )
                    break
                if lines[j].startswith("  - "):
                    # Stop at preceding peer block even if not docker
                    if re.match(r"^\s*-\s+package-ecosystem:", lines[j]):
                        start = j
                        indent_pattern = rf"^{re.escape(re.match(r'^(\s*)', lines[j]).group(1))}-\s+package-ecosystem:\s+\""
                        break
                j -= 1
            if start is None:
                start = idx
            break

    if start is None:
        return

    end = len(lines)
    if indent_pattern:
        for k in range(start + 1, len(lines)):
            if re.match(indent_pattern, lines[k]):
                end = k
                break

    del lines[start:end]

    # Remove surplus blank lines
    while (
        start < len(lines)
        and start > 0
        and lines[start].strip() == ""
        and lines[start - 1].strip() == ""
    ):
        del lines[start]

    dependabot_path.write_text("\n".join(lines).rstrip() + "\n")


def run_go_mod_tidy() -> None:
    subprocess.run(["go", "mod", "tidy"], check=True, cwd=ROOT)


def main(argv: list[str]) -> int:
    try:
        app_name = parse_args(argv)
        validate_app_name(app_name)
        app_dir = require_repo_layout(app_name)
        binary_name = parse_app_binary(app_dir)
        require_cmd("go")

        remove_optional_artifacts(app_name, binary_name)
        remove_app_directory(app_dir)
        remove_app_from_ci_matrix(app_name)
        remove_dependabot_entry(app_name)
        run_go_mod_tidy()

        print(f"Removed apps/{app_name}/ and cleaned shared configs.")
        print()
        print("Next steps:")
        print("  - Prune any remaining references (README, docs, dashboards, secrets)")
        print("  - Run 'git status' to review deletions before committing")
        return 0
    except DeleteAppError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1
    except subprocess.CalledProcessError as exc:
        print(
            f"error: command {' '.join(exc.cmd)} failed with exit code {exc.returncode}",
            file=sys.stderr,
        )
        return exc.returncode


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
