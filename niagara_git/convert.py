"""Textconv views for ``.bog`` and ``.dist`` files.

Output depends only on archive bytes. The path Git passes to a textconv
filter is often a temporary checkout, so it is never included.
"""

from __future__ import annotations

import hashlib
import zipfile
from collections import defaultdict

from niagara_git import FORMAT_VERSION
from niagara_git.classify import classify, noisy_category
from niagara_git.xmlfmt import (
    XmlFormatError,
    looks_like_xml,
    normalized_source,
    pretty_xml,
)
from niagara_git.zipio import MAX_ENTRY_BYTES, Member, open_zip

_XML_SUFFIX = ".xml"


def sha256_hex(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def convert_bog(data: bytes) -> str:
    """Pretty-print the XML stored in a ``.bog`` (zip or raw XML)."""
    if data.startswith(b"PK"):
        return _convert_bog_zip(data)
    if looks_like_xml(data):
        return _render_xml(data)
    return _opaque(
        "bog",
        data,
        "not a zip archive or XML document; passphrase-protected bog files are encrypted",
    )


def convert_dist(data: bytes) -> str:
    """List a ``.dist`` backup and inline every non-skipped ``.bog``."""
    if data.startswith(b"PK"):
        return _convert_dist_zip(data)
    if looks_like_xml(data):
        return _render_xml(data)
    return _opaque(
        "dist",
        data,
        "not a zip archive; passphrase-protected distribution files are encrypted",
    )


def _convert_bog_zip(data: bytes) -> str:
    try:
        archive, members = open_zip(data)
    except zipfile.BadZipFile as exc:
        return _opaque("bog", data, f"not a readable zip archive: {exc}")
    try:
        rendered = _render_members(members)
    finally:
        archive.close()
    return rendered


def _convert_dist_zip(data: bytes) -> str:
    try:
        archive, members = open_zip(data)
    except zipfile.BadZipFile as exc:
        return _opaque("dist", data, f"not a readable zip archive: {exc}")
    try:
        return _render_dist(members)
    finally:
        archive.close()


def _render_dist(members: list[Member]) -> str:
    shown: list[tuple[str, int, str]] = []
    secrets: list[tuple[str, int]] = []
    others: list[tuple[str, str, int]] = []
    noisy_totals: dict[str, list[int]] = defaultdict(lambda: [0, 0])
    bogs: list[tuple[str, str]] = []

    ordered = sorted(members, key=lambda member: _sort_key(member.name))
    for member in ordered:
        kind, display = classify(member.name)
        if kind == "noisy":
            bucket = noisy_totals[noisy_category(display)]
            bucket[0] += 1
            bucket[1] += member.declared_size
            continue
        if kind == "secret":
            secrets.append((display, member.declared_size))
            continue
        if kind == "unsafe":
            others.append(("unsafe", display, member.declared_size))
            continue

        if member.declared_size > MAX_ENTRY_BYTES:
            others.append(("too-large", display, member.declared_size))
            continue
        try:
            payload = member.read()
        except (RuntimeError, ValueError, zipfile.BadZipFile) as exc:
            others.append(
                (f"unreadable ({exc.__class__.__name__})", display, member.declared_size)
            )
            continue

        shown.append((display, len(payload), sha256_hex(payload)))
        if display.casefold().endswith(".bog"):
            bogs.append((display, convert_bog(payload).rstrip("\n")))

    lines = [
        f"# niagara-git dist format {FORMAT_VERSION}",
        f"# files {len(shown)}",
        f"# skipped-noisy {sum(item[0] for item in noisy_totals.values())}",
        f"# skipped-secret {len(secrets)}",
    ]
    if others:
        lines.append(f"# skipped-other {len(others)}")
    lines.append("")
    lines.append("## files")
    for path, size, digest in sorted(shown, key=lambda item: _path_key(item[0])):
        lines.append(f"{size} {digest} {path}")

    if noisy_totals:
        lines.append("")
        lines.append("## skipped-noisy")
        for category in sorted(noisy_totals):
            count, size = noisy_totals[category]
            lines.append(f"{category} count={count} size={size}")

    if secrets:
        lines.append("")
        lines.append("## skipped-secret")
        for path, size in sorted(secrets, key=lambda item: _path_key(item[0])):
            lines.append(f"{size} {path}")

    if others:
        lines.append("")
        lines.append("## skipped-other")
        for reason, path, size in sorted(others, key=lambda item: _path_key(item[1])):
            lines.append(f"{reason} {size} {path}")

    for path, xml in sorted(bogs, key=lambda item: _path_key(item[0])):
        lines.append("")
        lines.append(f"## bog {path}")
        lines.append(xml)

    return "\n".join(lines) + "\n"


def _render_members(members: list[Member]) -> str:
    """Render a bog zip. A single XML member is returned as pure XML."""
    rendered: list[tuple[str, str]] = []
    binaries: list[tuple[str, int, str]] = []
    problems: list[str] = []

    ordered = sorted(members, key=lambda member: _sort_key(member.name))
    for member in ordered:
        _kind, display = classify(member.name)
        if _kind != "show":
            problems.append(f"# skipped {_kind} {display}")
            continue
        if member.declared_size > MAX_ENTRY_BYTES:
            problems.append(f"# skipped too-large {member.declared_size} {display}")
            continue
        try:
            payload = member.read()
        except (RuntimeError, ValueError, zipfile.BadZipFile) as exc:
            problems.append(f"# unreadable {display} ({exc.__class__.__name__})")
            continue
        if _is_xml_member(display, payload):
            rendered.append((display, _render_xml(payload).rstrip("\n")))
        else:
            binaries.append((display, len(payload), sha256_hex(payload)))

    if len(rendered) == 1 and not binaries and not problems:
        return rendered[0][1] + "\n"

    lines: list[str] = []
    if not rendered and not binaries and not problems:
        lines.append("# niagara-git: bog zip contains no file entries")
    for path, size, digest in binaries:
        if lines:
            lines.append("")
        lines.append(f"# binary {path}")
        lines.append(f"# size {size}")
        lines.append(f"# sha256 {digest}")
    for path, xml in rendered:
        if lines:
            lines.append("")
        lines.append(f"# entry {path}")
        lines.append(xml)
    for note in problems:
        if lines:
            lines.append("")
        lines.append(note)
    return "\n".join(lines) + "\n"


def _render_xml(data: bytes) -> str:
    try:
        return pretty_xml(data)
    except XmlFormatError as exc:
        return normalized_source(data, str(exc))


def _is_xml_member(display: str, payload: bytes) -> bool:
    if display.casefold().endswith(_XML_SUFFIX):
        return True
    # A bog is normally just file.xml. If the member was named something else
    # but is still XML, show it as XML rather than a checksum.
    return looks_like_xml(payload)


def _opaque(kind: str, data: bytes, reason: str) -> str:
    return (
        f"# niagara-git: {kind} not decoded ({reason})\n"
        f"# size {len(data)}\n"
        f"# sha256 {sha256_hex(data)}\n"
    )


def _sort_key(name: str) -> tuple[str, str]:
    display = name.replace("\\", "/")
    return (display.casefold(), display)


def _path_key(path: str) -> tuple[str, str]:
    return (path.casefold(), path)
