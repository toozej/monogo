#!/usr/bin/env python3
"""Scaffold a new app from the golang-starter template."""

from __future__ import annotations

import argparse
import os
import re
import shutil
import stat
import subprocess
import sys
import textwrap
from pathlib import Path
from string import Template
from collections.abc import Iterable


ROOT = Path(__file__).resolve().parent.parent
STARTER_APP = ROOT / "apps" / "golang-starter"


class ScaffoldError(RuntimeError):
    """Raised when scaffolding cannot proceed."""


def parse_args(argv: list[str]) -> str:
    parser = argparse.ArgumentParser(
        prog="create-new-app",
        add_help=False,
        formatter_class=argparse.RawDescriptionHelpFormatter,
        description=textwrap.dedent(
            """
            Scaffolds a new minimal Go app under apps/<app-name>/ that follows the
            golang-starter conventions used by the `golang-starter` template app (cobra-based CLI, shared
            `pkg/config` loader, simple internal starter package, demo script).

            It then runs 'task generate:app APP=<app>' and 'go mod tidy' so the new app
            plugs into the existing CI/release flows.

            Example:
              task app:new APP=mytool
            """
        ).strip(),
    )
    parser.add_argument("app_name", nargs="?", help="Name for the new app")
    parser.add_argument(
        "-h", "--help", action="help", help="Show this help message and exit"
    )

    args = parser.parse_args(argv)

    env_app = os.getenv("APP", "")
    app_name = args.app_name or env_app
    if not app_name:
        parser.error("APP name is required (pass as argument or set APP env var)")
    return app_name


def validate_inputs(app_name: str) -> None:
    if app_name in {"", ".", ".."}:
        raise ScaffoldError(f"APP must be a single-segment name, got '{app_name}'")
    if "/" in app_name:
        raise ScaffoldError(f"APP must not contain '/', got '{app_name}'")
    if not re.fullmatch(r"[A-Za-z0-9._-]+", app_name):
        raise ScaffoldError(
            f"APP must be a single-segment name using [A-Za-z0-9._-], got: '{app_name}'"
        )
    if not (ROOT / "go.mod").is_file():
        raise ScaffoldError(f"missing go.mod at {ROOT / 'go.mod'}")
    apps_dir = ROOT / "apps"
    if not apps_dir.is_dir():
        raise ScaffoldError(f"missing apps/ directory at {apps_dir}")
    target_dir = apps_dir / app_name
    if target_dir.exists():
        raise ScaffoldError(f"apps/{app_name} already exists")
    if not STARTER_APP.is_dir():
        raise ScaffoldError(f"starter template missing at {STARTER_APP}")


def require_cmd(name: str) -> None:
    if shutil.which(name) is None:
        raise ScaffoldError(f"{name} is required")


def copy_starter(app_name: str) -> Path:
    target_dir = ROOT / "apps" / app_name
    shutil.copytree(STARTER_APP, target_dir)
    starter_cmd = target_dir / "cmd" / "golang-starter"
    if starter_cmd.exists():
        starter_cmd.rename(target_dir / "cmd" / app_name)
    return target_dir


def title_case(name: str) -> str:
    parts = re.split(r"[-_]+", name)
    cleaned = [part.capitalize() for part in parts if part]
    return " ".join(cleaned) if cleaned else name.capitalize()


def iter_text_files(root: Path) -> Iterable[tuple[Path, str]]:
    binary_suffixes = {".png", ".jpg", ".jpeg", ".gif", ".ico", ".bin", ".gz"}
    for path in root.rglob("*"):
        if not path.is_file():
            continue
        if path.suffix.lower() in binary_suffixes:
            continue
        try:
            text = path.read_text()
        except UnicodeDecodeError:
            continue
        yield path, text


def apply_replacements(
    target_dir: Path, app_name: str, description: str, short_description: str
) -> None:
    module_path = f"github.com/toozej/monogo/apps/{app_name}"
    replacements = {
        "github.com/toozej/monogo/apps/golang-starter": module_path,
        "apps/golang-starter": f"apps/{app_name}",
        "docs/diagrams/golang-starter": f"docs/diagrams/{app_name}",
        "dist/golang-starter": f"dist/{app_name}",
    }
    title = title_case(app_name)
    upper = app_name.upper()

    # The starter refers to itself by the lowercase slug "golang-starter" in
    # import/cmd paths, the binary name, and doc comments. Unlike the old
    # "monogo" app name, "golang-starter" never collides with the repo-root
    # module (github.com/toozej/monogo), so it is replaced unconditionally.
    # The title/upper forms cover any human-readable prose added to the starter.
    regex_rules = [
        (re.compile(r"Golang[ -]Starter"), title),
        (re.compile(r"GOLANG[ _-]STARTER"), upper),
        (re.compile(r"golang-starter"), app_name),
    ]

    for path, text in iter_text_files(target_dir):
        original = text
        for old, new in replacements.items():
            text = text.replace(old, new)
        for pattern, repl in regex_rules:
            text = pattern.sub(repl, text)

        if path.name == "app.yaml":
            text = re.sub(
                r"^description:.*$",
                f'description: "{description}"',
                text,
                flags=re.MULTILINE,
            )
            text = re.sub(
                r"^shortDescription:.*$",
                f'shortDescription: "{short_description}"',
                text,
                flags=re.MULTILINE,
            )

        if text != original:
            path.write_text(text)


def write_app_readme(target_dir: Path, app_name: str, description: str) -> None:
    template = Template(
        textwrap.dedent(
            """
            # ${app_name}

            ${description}

            Scaffolded with `task app:new APP=${app_name}`. See the repo root README and
            `scripts/create-new-app.py` for the conventions in use.

            ## Layout

            ```
            apps/${app_name}/
              app.yaml                 # build/release metadata consumed by Task + GoReleaser
              main.go                  # entrypoint that defers to cmd/${app_name}
              cmd/${app_name}/         # cobra CLI (root command + version subcommand)
              internal/starter/        # core hello-world logic (replace with your own)
              internal/config/         # app-specific config struct backed by pkg/config
              demo.sh                  # smoke-test script run via `task demo APP=${app_name}`
            ```

            ## Common workflows

            ```sh
            task test        APP=${app_name}        # run unit tests
            task local:build APP=${app_name}        # produce ./out/${app_name}
            task local:run   APP=${app_name}        # run the binary against ./apps/${app_name}/.env
            task demo        APP=${app_name}        # exercise the freshly built binary
            task release:test APP=${app_name}       # goreleaser snapshot build
            task release     APP=${app_name} TYPE=patch  # tag and push a release
            ```
            """
        ).strip()
    )
    readme_path = target_dir / "README.md"
    readme_path.write_text(
        template.substitute(app_name=app_name, description=description)
    )


def write_gitignore(target_dir: Path) -> None:
    content = (
        textwrap.dedent(
            """
        # generated by Task; do not commit
        c.out
        # demo artefacts
        demo-output/
        """
        ).strip()
        + "\n"
    )
    (target_dir / ".gitignore").write_text(content)


def ensure_executable_demo(target_dir: Path) -> None:
    demo = target_dir / "demo.sh"
    if demo.exists():
        mode = demo.stat().st_mode
        demo.chmod(mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)


def add_app_to_ci_matrix(app_name: str) -> None:
    ci_path = ROOT / ".github" / "workflows" / "ci.yaml"
    if not ci_path.exists():
        return
    lines = ci_path.read_text().splitlines()

    result: list[str] = []
    i = 0
    added = False
    while i < len(lines):
        line = lines[i]
        result.append(line)
        match = re.match(r"^(\s*)app:\s*$", line)
        if match:
            indent = match.group(1)
            list_indent = indent + "  "
            collected: list[str] = []
            found = False
            i += 1
            while i < len(lines):
                next_line = lines[i]
                if re.match(rf"^{re.escape(list_indent)}- ", next_line):
                    collected.append(next_line)
                    if next_line.strip() == f"- {app_name}":
                        found = True
                    i += 1
                    continue
                break
            if not found:
                collected.append(f"{list_indent}- {app_name}")
                added = True
            result.extend(collected)
            continue
        i += 1

    if added:
        ci_path.write_text("\n".join(result) + "\n")


def run_command(command: list[str], cwd: Path) -> None:
    subprocess.run(command, check=True, cwd=cwd)


def try_run_task_generate(app_name: str) -> None:
    try:
        run_command(["task", "generate", f"APP={app_name}"], ROOT)
        print(f"Generated Docker/Air/Compose/GoReleaser configs for {app_name}")
    except subprocess.CalledProcessError:
        print(
            f"warning: 'task generate:app APP={app_name}' failed; review apps/{app_name}/ before continuing",
            file=sys.stderr,
        )


def scaffold(app_name: str) -> None:
    validate_inputs(app_name)

    for tool in ("task", "go"):
        require_cmd(tool)

    description = os.getenv("DESCRIPTION", f"{app_name} scaffolded via task app:new")
    short_description = os.getenv("SHORT_DESCRIPTION", f"{app_name} scaffolded app")

    target_dir = copy_starter(app_name)
    apply_replacements(target_dir, app_name, description, short_description)
    ensure_executable_demo(target_dir)
    write_app_readme(target_dir, app_name, description)
    write_gitignore(target_dir)

    add_app_to_ci_matrix(app_name)

    run_command(["go", "mod", "tidy"], ROOT)
    try_run_task_generate(app_name)

    print()
    print(f"Scaffolded apps/{app_name}/.")
    print()
    print("Next steps:")
    print(
        f"  - Edit apps/{app_name}/app.yaml (description, port, runtimeImage, cgoEnabled, ...)"
    )
    print("  - Replace internal/starter with your app's real logic")
    print(f"  - Add commands under cmd/{app_name}/ as needed")
    print(f"  - Run 'task test APP={app_name}' and 'task local:build APP={app_name}'")
    print(f"  - Commit the new files; {app_name} is now part of CI workflow")


def main(argv: list[str]) -> int:
    try:
        app_name = parse_args(argv)
        scaffold(app_name)
        return 0
    except ScaffoldError as exc:
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
