"""Register the textconv drivers and drop the station templates."""

from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

from niagara_git import FORMAT_VERSION

BEGIN = "# niagara-git:begin"
END = "# niagara-git:end"

_BLOCK = re.compile(re.escape(BEGIN) + r".*?" + re.escape(END) + r"\n?", re.DOTALL)


def templates_dir() -> Path:
    return Path(__file__).resolve().parent.parent / "templates"


def shell_quote(value: str) -> str:
    """POSIX single quotes. Git runs textconv through a shell, including Git for Windows."""
    return "'" + value.replace("'", "'\\''") + "'"


def textconv_command(mode: str) -> str:
    if mode not in {"bog", "dist"}:
        raise ValueError(mode)
    executable = Path(sys.executable).resolve().as_posix()
    script = Path(__file__).resolve().parent.joinpath("__main__.py").as_posix()
    return (
        f"{shell_quote(executable)} {shell_quote(script)} "
        f"--format {FORMAT_VERSION} {mode}"
    )


def install(repo: Path | None = None) -> None:
    """Configure ``repo`` (default: current git work tree) for niagara-git."""
    root = _resolve_repo(repo)
    _merge_template(root / ".gitattributes", templates_dir() / "gitattributes")
    _merge_template(root / ".gitignore", templates_dir() / "gitignore")
    _git(root, "diff.niagara-bog.textconv", textconv_command("bog"))
    _git(root, "diff.niagara-bog.cachetextconv", "true")
    _git(root, "diff.niagara-dist.textconv", textconv_command("dist"))
    _git(root, "diff.niagara-dist.cachetextconv", "true")
    sys.stderr.write(f"niagara-git: configured {root}\n")
    sys.stderr.write(
        "niagara-git: commit .gitattributes and .gitignore. "
        "Each clone must run the installer; the diff command is local git config.\n"
    )


def merge_block(existing: str, block: str) -> str:
    """Insert or replace the marked niagara-git block. Idempotent."""
    text = existing.replace("\r\n", "\n").replace("\r", "\n")
    snippet = block.strip("\n") + "\n"
    if text and not text.endswith("\n"):
        text += "\n"
    if _BLOCK.search(text):
        replaced = _BLOCK.sub("\0", text, count=0)
        # Drop extra copies if a file was merged twice by hand.
        replaced = replaced.replace("\0", snippet, 1)
        replaced = replaced.replace("\0", "")
        return replaced
    if not text:
        return snippet
    if not text.endswith("\n\n"):
        return text + "\n" + snippet
    return text + snippet


def _merge_template(dest: Path, template: Path) -> None:
    block = template.read_text(encoding="utf-8")
    current = dest.read_text(encoding="utf-8") if dest.exists() else ""
    dest.write_text(merge_block(current, block), encoding="utf-8", newline="\n")


def _resolve_repo(repo: Path | None) -> Path:
    if repo is None:
        completed = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            check=False,
            capture_output=True,
            text=True,
        )
        if completed.returncode != 0:
            raise SystemExit(
                "niagara-git: not a git repository. Pass the station repo path, "
                "or run git init first.\n"
            )
        return Path(completed.stdout.strip()).resolve()

    root = repo.expanduser().resolve()
    if not root.is_dir():
        raise SystemExit(f"niagara-git: not a directory: {root}\n")
    completed = subprocess.run(
        ["git", "-C", str(root), "rev-parse", "--show-toplevel"],
        check=False,
        capture_output=True,
        text=True,
    )
    if completed.returncode != 0:
        raise SystemExit(
            f"niagara-git: {root} is not a git repository. Run git init there first.\n"
        )
    return Path(completed.stdout.strip()).resolve()


def _git(repo: Path, key: str, value: str) -> None:
    subprocess.run(
        ["git", "-C", str(repo), "config", "--local", key, value],
        check=True,
    )
