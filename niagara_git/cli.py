"""Command line entry point used by Git textconv and the installers."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from niagara_git import FORMAT_VERSION, __version__
from niagara_git.convert import convert_bog, convert_dist
from niagara_git.install import install


def main(argv: list[str] | None = None) -> int:
    if sys.version_info < (3, 8):
        sys.stderr.write("niagara-git requires Python 3.8 or newer\n")
        return 1

    parser = argparse.ArgumentParser(
        prog="niagara-git",
        description="Git textconv filters for Niagara 4 .bog and .dist files.",
    )
    parser.add_argument("--version", action="version", version=f"niagara-git {__version__}")
    parser.add_argument(
        "--format",
        type=int,
        default=None,
        help=argparse.SUPPRESS,
    )
    sub = parser.add_subparsers(dest="command", required=True)

    bog = sub.add_parser("bog", help="pretty-print a .bog file to stdout")
    bog.add_argument("file")

    dist = sub.add_parser("dist", help="list a .dist backup and inline its .bog files")
    dist.add_argument("file")

    setup = sub.add_parser(
        "install",
        help="register diff drivers and merge the station templates into a repo",
    )
    setup.add_argument(
        "repo",
        nargs="?",
        help="station repository (default: the current git work tree)",
    )

    args = parser.parse_args(argv)
    if args.format is not None and args.format != FORMAT_VERSION:
        sys.stderr.write(
            f"niagara-git: textconv format {args.format} requested; "
            f"this copy emits format {FORMAT_VERSION}. Re-run the installer.\n"
        )

    if args.command == "install":
        repo = Path(args.repo) if args.repo else None
        install(repo)
        return 0

    path = Path(args.file)
    if not path.is_file():
        sys.stderr.write(f"niagara-git: file not found: {path}\n")
        return 1
    data = path.read_bytes()
    text = convert_bog(data) if args.command == "bog" else convert_dist(data)
    sys.stdout.buffer.write(text.encode("utf-8"))
    return 0
