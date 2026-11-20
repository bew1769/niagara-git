"""Zip helpers shared by the converters and the sample generator.

Zip timestamps, entry order, and compression method are ignored on read so
two archives with the same members produce the same textconv output.
"""

from __future__ import annotations

import io
import zipfile
from dataclasses import dataclass

# One station XML larger than this is not a useful line diff. Refuse to
# inflate it rather than spend memory on a bad archive.
MAX_ENTRY_BYTES = 256 * 1024 * 1024

_FIXED_DATE = (1980, 1, 1, 0, 0, 0)


@dataclass(frozen=True)
class Member:
    """One non-directory zip member, in archive order."""

    name: str
    info: zipfile.ZipInfo
    source: zipfile.ZipFile

    @property
    def declared_size(self) -> int:
        return self.info.file_size

    def read(self) -> bytes:
        if self.info.file_size > MAX_ENTRY_BYTES:
            raise ValueError(
                f"entry larger than {MAX_ENTRY_BYTES} bytes ({self.info.file_size})"
            )
        data = self.source.read(self.info)
        if len(data) > MAX_ENTRY_BYTES:
            raise ValueError(
                f"entry larger than {MAX_ENTRY_BYTES} bytes ({len(data)})"
            )
        return data


def open_zip(data: bytes) -> tuple[zipfile.ZipFile, list[Member]]:
    """Open a zip from memory and list file members in archive order."""
    archive = zipfile.ZipFile(io.BytesIO(data))
    members = [
        Member(name=info.filename, info=info, source=archive)
        for info in archive.infolist()
        if not info.is_dir() and not info.filename.endswith("/")
    ]
    return archive, members


def build_zip(entries: list[tuple[str, bytes]]) -> bytes:
    """Build a deterministic zip (fixed timestamps, deflate level 9)."""
    buffer = io.BytesIO()
    with zipfile.ZipFile(
        buffer,
        "w",
        compression=zipfile.ZIP_DEFLATED,
        compresslevel=9,
    ) as archive:
        for name, payload in entries:
            info = zipfile.ZipInfo(filename=name, date_time=_FIXED_DATE)
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = 0o644 << 16
            info.create_system = 3
            archive.writestr(info, payload)
    return buffer.getvalue()
