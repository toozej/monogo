#!/usr/bin/env python3
"""One-time, idempotent cutover release of every imported app from the monorepo.

Each app under ``apps/<app>/`` was imported from its own repo at
``github.com/toozej/<app>`` (see ``scripts/import-app.sh``). The importer brought
across the original repo's version tags as ``apps/<app>/vX.Y.Z`` and its GitHub
release metadata under ``apps/<app>/.monogo/releases/``. This script cuts those
apps over to the monorepo by publishing, for each app, a *minor version bump
above its most recent original-repo tag* as a monorepo release tag
(``apps/<app>/v<bumped>``). Pushing that tag triggers
``.github/workflows/release.yaml`` which builds, signs, and publishes the app.

It is safe to run repeatedly. "Most recent original tag" is derived only from
tags whose commit predates the monorepo layout (no ``apps/<app>/app.yaml`` at
that commit), so the monorepo release tags this script creates never shift the
computed target. An app is treated as already done when its target GitHub
release exists (or, if ``gh`` is unavailable, when its target tag exists), so
re-runs skip finished apps and act only on the rest -- which is what makes it
usable to iterate while fixing release issues.

Usage::

    # Dry run: print the per-app plan and exit (default; read-only).
    scripts/cutover-release-apps.py

    # Create and push the cutover tags for apps that still need them.
    scripts/cutover-release-apps.py --execute

    # Only certain apps.
    scripts/cutover-release-apps.py --execute --apps url2anki,go-listen

    # Re-trigger apps that were tagged but whose release did not finish
    # (deletes and re-pushes the target tag).
    scripts/cutover-release-apps.py --execute --retry-failed

    # Validate each app's GoReleaser config/snapshot before tagging it.
    scripts/cutover-release-apps.py --execute --preflight
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from shutil import which

ROOT = Path(__file__).resolve().parent.parent
APPS_DIR = ROOT / "apps"

SEMVER_RE = re.compile(r"^v(\d+)\.(\d+)\.(\d+)$")
Version = tuple[int, int, int]


class CutoverError(Exception):
    """Fatal, user-actionable error."""


# --- small subprocess helpers ------------------------------------------------


def git(*args: str, check: bool = True) -> subprocess.CompletedProcess:
    return subprocess.run(
        ["git", *args],
        cwd=ROOT,
        check=check,
        capture_output=True,
        text=True,
    )


def git_out(*args: str) -> str:
    return git(*args).stdout.strip()


# --- version helpers ---------------------------------------------------------


def parse_version(version_tag: str) -> Version | None:
    """Parse a ``vMAJOR.MINOR.PATCH`` string into a tuple, or None."""
    m = SEMVER_RE.match(version_tag)
    if not m:
        return None
    return int(m.group(1)), int(m.group(2)), int(m.group(3))


def fmt_version(v: Version) -> str:
    return f"v{v[0]}.{v[1]}.{v[2]}"


def minor_bump(v: Version) -> Version:
    """Increment the minor component and reset patch (v1.4.2 -> v1.5.0)."""
    major, minor, _patch = v
    return major, minor + 1, 0


# --- discovery ---------------------------------------------------------------


def discover_apps() -> list[str]:
    """Imported apps: have an app.yaml and a .monogo import-metadata dir.

    The .monogo dir is written by import-app.sh, so it distinguishes apps that
    came from their own repo (which we want to cut over) from the in-repo
    starter (``golang-starter``), which has no upstream repo to release from.
    """
    apps = []
    for entry in sorted(APPS_DIR.iterdir()):
        if not entry.is_dir():
            continue
        if not (entry / "app.yaml").is_file():
            continue
        if not (entry / ".monogo").is_dir():
            continue
        apps.append(entry.name)
    return apps


def app_yaml_present_at(ref: str, app: str) -> bool:
    """True if apps/<app>/app.yaml exists in the tree of ``ref``."""
    result = git("cat-file", "-e", f"{ref}:apps/{app}/app.yaml", check=False)
    return result.returncode == 0


def original_tags(app: str) -> list[str]:
    """apps/<app>/vX.Y.Z tags whose commit predates the monorepo layout.

    A monorepo release commit carries apps/<app>/app.yaml; an imported original
    tag's commit does not. Excluding the former keeps the computed base stable
    no matter how many monorepo tags we add.
    """
    out = git("tag", "--list", f"apps/{app}/v*", check=False).stdout.splitlines()
    tags = []
    for tag in (t.strip() for t in out if t.strip()):
        version = tag.rsplit("/", 1)[-1]
        if parse_version(version) is None:
            continue
        if app_yaml_present_at(tag, app):
            continue  # monorepo release tag, not an original one
        tags.append(tag)
    return tags


def latest_from_release_metadata(app: str) -> Version | None:
    """Fallback: highest vX.Y.Z tagName in the imported releases.json."""
    meta = APPS_DIR / app / ".monogo" / "releases" / "releases.json"
    if not meta.is_file():
        return None
    try:
        records = json.loads(meta.read_text())
    except (json.JSONDecodeError, OSError):
        return None
    versions = []
    for rec in records or []:
        v = parse_version(str(rec.get("tagName", "")))
        if v is not None:
            versions.append(v)
    return max(versions) if versions else None


@dataclass
class AppPlan:
    app: str
    base: Version | None  # latest original version (None => first release)
    base_source: str  # "tag" | "releases.json" | "none"
    target: Version
    target_tag: str
    tag_local: bool = False
    tag_remote: bool = False
    released: bool | None = None  # None => unknown (gh unavailable)
    status: str = "pending"  # pending | tagged | released
    result: str = ""


def build_plan(app: str, remote: str, gh_ok: bool) -> AppPlan:
    tags = original_tags(app)
    base: Version | None = None
    source = "none"
    if tags:
        parsed = (parse_version(t.rsplit("/", 1)[-1]) for t in tags)
        base = max(v for v in parsed if v is not None)
        source = "tag"
    else:
        meta_latest = latest_from_release_metadata(app)
        if meta_latest is not None:
            base, source = meta_latest, "releases.json"

    target = minor_bump(base) if base is not None else (0, 1, 0)
    target_tag = f"apps/{app}/{fmt_version(target)}"

    plan = AppPlan(
        app=app,
        base=base,
        base_source=source,
        target=target,
        target_tag=target_tag,
    )

    ref = f"refs/tags/{target_tag}"
    local = git("rev-parse", "-q", "--verify", ref, check=False)
    plan.tag_local = local.returncode == 0
    plan.tag_remote = bool(
        git("ls-remote", "--tags", remote, ref, check=False).stdout.strip()
    )
    if gh_ok:
        view = subprocess.run(
            ["gh", "release", "view", target_tag],
            cwd=ROOT,
            capture_output=True,
            text=True,
        )
        plan.released = view.returncode == 0

    if plan.released:
        plan.status = "released"
    elif plan.tag_local or plan.tag_remote:
        # Pushed, but the release is not confirmed (in progress or failed).
        plan.status = "tagged"
    else:
        plan.status = "pending"
    return plan


# --- execution ---------------------------------------------------------------


def preflight(app: str) -> bool:
    print(f"    running preflight: make release-test APP={app}")
    completed = subprocess.run(["make", "release-test", f"APP={app}"], cwd=ROOT)
    return completed.returncode == 0


def push_tag(plan: AppPlan, release_sha: str, remote: str) -> None:
    message = f"{plan.app} {fmt_version(plan.target)} (monorepo cutover)"
    git("tag", "-a", "-f", plan.target_tag, "-m", message, release_sha)
    git("push", remote, f"refs/tags/{plan.target_tag}")


def delete_tag(plan: AppPlan, remote: str) -> None:
    if plan.tag_remote:
        git("push", remote, f":refs/tags/{plan.target_tag}", check=False)
    git("tag", "-d", plan.target_tag, check=False)


def execute_plan(
    plans: list[AppPlan],
    release_sha: str,
    remote: str,
    retry_failed: bool,
    do_preflight: bool,
) -> int:
    failures = 0
    for plan in plans:
        if plan.status == "released":
            plan.result = "skip (already released)"
            continue
        if plan.status == "tagged" and not retry_failed:
            plan.result = "skip (tag exists; use --retry-failed to re-trigger)"
            continue

        print(f"  {plan.app} -> {plan.target_tag}")
        if do_preflight and not preflight(plan.app):
            plan.result = "FAILED preflight; not tagged"
            failures += 1
            continue

        try:
            if plan.status == "tagged":  # retry_failed path
                print("    re-triggering: deleting and re-pushing existing tag")
                delete_tag(plan, remote)
            push_tag(plan, release_sha, remote)
            plan.result = "tag pushed (release workflow triggered)"
        except subprocess.CalledProcessError as exc:
            plan.result = f"FAILED: {exc.stderr.strip() or exc}"
            failures += 1
    return failures


# --- preconditions -----------------------------------------------------------


def ensure_preconditions(allow_dirty: bool, allow_branch: bool) -> str:
    if not (ROOT / ".git").exists():
        raise CutoverError(f"{ROOT} is not a git repository")

    branch = git_out("rev-parse", "--abbrev-ref", "HEAD")
    if branch != "main" and not allow_branch:
        raise CutoverError(
            f"on branch '{branch}', not 'main'. Cutover tags should be cut from "
            "main; pass --allow-branch to override."
        )
    if not allow_dirty and git_out("status", "--porcelain"):
        raise CutoverError(
            "working tree is not clean. Commit or stash changes, or pass "
            "--allow-dirty (the tag is created at HEAD regardless)."
        )
    return git_out("rev-parse", "HEAD")


# --- reporting ---------------------------------------------------------------

BASE_NOTE = {
    "tag": "",
    "releases.json": "(from releases.json)",
    "none": "(no prior tag; first release)",
}


def print_plan(plans: list[AppPlan], release_sha: str, execute: bool) -> None:
    print(f"\nCutover target commit: {release_sha}\n")
    header = (
        f"{'APP':<24} {'ORIGINAL':<12} {'->':<2} {'TARGET':<10} {'STATUS':<9} DETAIL"
    )
    print(header)
    print("-" * len(header))
    for p in plans:
        base = fmt_version(p.base) if p.base else "<none>"
        detail = p.result if execute else BASE_NOTE[p.base_source]
        print(
            f"{p.app:<24} {base:<12} {'->':<2} "
            f"{fmt_version(p.target):<10} {p.status:<9} {detail}"
        )


def summarize(plans: list[AppPlan]) -> None:
    counts: dict[str, int] = {}
    for p in plans:
        counts[p.status] = counts.get(p.status, 0) + 1
    parts = [f"{n} {status}" for status, n in sorted(counts.items())]
    print("\nSummary: " + (", ".join(parts) if parts else "no apps"))


# --- CLI ---------------------------------------------------------------------


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Idempotent monorepo cutover release for all imported apps.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument(
        "--execute",
        action="store_true",
        help="Create and push tags. Without this flag only the plan is printed.",
    )
    parser.add_argument(
        "--apps",
        default="",
        help="Comma-separated subset of apps to act on (default: all imported).",
    )
    parser.add_argument(
        "--remote",
        default="origin",
        help="Git remote to push tags to (default: origin).",
    )
    parser.add_argument(
        "--retry-failed",
        action="store_true",
        help="For apps tagged but not released, delete and re-push the tag to "
        "re-trigger the release workflow.",
    )
    parser.add_argument(
        "--preflight",
        action="store_true",
        help="Run 'make release-test APP=<app>' before tagging; skip on failure.",
    )
    parser.add_argument(
        "--allow-dirty",
        action="store_true",
        help="Do not require a clean working tree.",
    )
    parser.add_argument(
        "--allow-branch",
        action="store_true",
        help="Allow running from a branch other than main.",
    )
    parser.add_argument(
        "--no-fetch",
        action="store_true",
        help="Skip 'git fetch --tags' before planning.",
    )
    parser.add_argument(
        "--yes",
        action="store_true",
        help="Skip the confirmation prompt before pushing tags.",
    )
    return parser.parse_args(argv)


def confirm(to_act: list[AppPlan], remote: str) -> bool:
    print(f"About to push {len(to_act)} release tag(s) to '{remote}':")
    for p in to_act:
        print(f"  {p.target_tag}")
    try:
        reply = input("\nProceed? This triggers real release workflows. [y/N] ")
    except EOFError:
        reply = ""
    return reply.strip().lower() in ("y", "yes")


def select_actionable(plans: list[AppPlan], retry_failed: bool) -> list[AppPlan]:
    actionable = [p for p in plans if p.status == "pending"]
    if retry_failed:
        actionable += [p for p in plans if p.status == "tagged"]
    return actionable


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    try:
        release_sha = ensure_preconditions(args.allow_dirty, args.allow_branch)

        if not args.no_fetch:
            # Best effort: land any tags that exist on the remote so the plan
            # reflects reality. Failures (offline, no remote) are non-fatal.
            fetch = git("fetch", "--tags", "--force", args.remote, check=False)
            if fetch.returncode != 0:
                print(
                    f"warning: 'git fetch --tags {args.remote}' failed; planning "
                    "from local state only.",
                    file=sys.stderr,
                )

        gh_ok = which("gh") is not None
        if not gh_ok:
            print(
                "note: 'gh' not found; cannot confirm published releases. "
                "Idempotency will fall back to tag existence.",
                file=sys.stderr,
            )

        apps = discover_apps()
        if args.apps:
            wanted = [a.strip() for a in args.apps.split(",") if a.strip()]
            unknown = [a for a in wanted if a not in apps]
            if unknown:
                raise CutoverError(
                    f"unknown or non-importable app(s): {', '.join(unknown)}. "
                    f"Known apps: {', '.join(apps)}"
                )
            apps = wanted
        if not apps:
            raise CutoverError(
                "no imported apps found under apps/*/ (need app.yaml + .monogo/)"
            )

        plans = [build_plan(app, args.remote, gh_ok) for app in apps]

        if not args.execute:
            print_plan(plans, release_sha, execute=False)
            summarize(plans)
            actionable = select_actionable(plans, args.retry_failed)
            print(
                f"\nDry run. {len(actionable)} app(s) would be tagged. "
                "Re-run with --execute to create and push tags."
            )
            return 0

        to_act = select_actionable(plans, args.retry_failed)
        if not to_act:
            print_plan(plans, release_sha, execute=False)
            summarize(plans)
            print("\nNothing to do: all selected apps are already released.")
            return 0

        print(f"\nCutover target commit: {release_sha}")
        if not args.yes and not confirm(to_act, args.remote):
            print("Aborted.")
            return 1

        failures = execute_plan(
            plans,
            release_sha,
            args.remote,
            args.retry_failed,
            args.preflight,
        )
        print()
        print_plan(plans, release_sha, execute=True)
        summarize(plans)
        if failures:
            print(f"\n{failures} app(s) failed. Fix the issue and re-run (idempotent).")
            return 1
        print(
            "\nDone. Watch the Release workflow runs; re-run this script to "
            "retry any that fail."
        )
        return 0

    except CutoverError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        print("\nInterrupted.", file=sys.stderr)
        return 130


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
