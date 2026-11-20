"""Decide which members of a station backup are worth diffing.

Niagara 4 layout, from Tridium's AX-to-N4 migration guide and platform guide:

* Protected station home holds ``config.bog``, timestamped ``config.backup.*``
  copies, and the ``alarm``, ``history``, ``dataRecovery``,
  ``provisioningNiagara``, and ``niagaraDriver_nVirtual`` folders.
* ``shared/`` (station home) holds graphics and nav files modules may write.
* Certificate stores live under the user home ``security`` folder
  (``keystore.jceks``, ``cacerts.jceks``, ``exemptions.tes``, ``*.bcfks``)
  and exported keys under ``certManagement`` (Station Security Guide).

History, alarms, and logs churn on every backup. Keystores are secret. Both
are omitted from the per-file listing.
"""

from __future__ import annotations

import re
from pathlib import PurePosixPath

Kind = str  # "show" | "noisy" | "secret" | "unsafe"

# Directory names matched case-insensitively anywhere in the path.
NOISY_DIR_NAMES = frozenset(
    {
        "alarm",
        "history",
        "logs",
        "logging",
        "datarecovery",
        "provisioningniagara",
        "niagaradriver_nvirtual",
        "__macosx",
    }
)

SECRET_DIR_NAMES = frozenset(
    {
        "security",
        "certmanagement",
        "certificates",
        "keystores",
    }
)

SECRET_FILENAMES = frozenset(
    {
        "keystore.jceks",
        "cacerts.jceks",
        "exemptions.tes",
    }
)

SECRET_EXTENSIONS = frozenset(
    {
        ".jceks",
        ".bcfks",
        ".pem",
        ".key",
        ".p12",
        ".pfx",
        ".jks",
        ".keystore",
        ".cer",
        ".crt",
        ".der",
        ".priv",
        ".tes",
    }
)

NOISY_FILENAMES = frozenset(
    {
        "station.out",
        "security.out",
        "audit.out",
        ".ds_store",
        "thumbs.db",
    }
)

NOISY_EXTENSIONS = frozenset({".log", ".hdb", ".adb"})

_CONFIG_BACKUP = re.compile(
    r"^config(?:\.backup\.|_backup_).+\.bog$",
    re.IGNORECASE,
)
_DRIVE = re.compile(r"^[A-Za-z]:")


def normalize_zip_path(name: str) -> tuple[str, str]:
    """Return ``(kind_or_empty, display_path)``.

    ``kind_or_empty`` is ``"unsafe"`` when the member should not be read,
    otherwise ``""`` and the caller classifies the display path.
    """
    raw = name.replace("\\", "/")
    absolute = raw.startswith("/") or bool(_DRIVE.match(raw))
    parts = [part for part in raw.split("/") if part not in ("", ".")]
    display = "/".join(parts)
    if absolute or not parts or any(part == ".." for part in parts):
        return "unsafe", display or raw
    return "", display


def noisy_category(display_path: str) -> str:
    """Stable bucket name for a path already classified as noisy."""
    parts = display_path.split("/")
    for part in parts[:-1]:
        folded = part.casefold()
        if folded in NOISY_DIR_NAMES:
            return folded
    name = parts[-1].casefold()
    if _CONFIG_BACKUP.match(name):
        return "config.backup"
    if name in NOISY_FILENAMES:
        if name in {"station.out", "security.out", "audit.out"}:
            return "*.out"
        return name
    suffix = PurePosixPath(name).suffix.casefold()
    if suffix in NOISY_EXTENSIONS:
        return f"*{suffix}"
    return "noisy"


def classify(name: str) -> tuple[Kind, str]:
    """Classify a zip member name.

    Returns ``(kind, display_path)`` where ``kind`` is ``show``, ``noisy``,
    ``secret``, or ``unsafe``. Secret wins over noisy so a key left under
    ``logs/`` is still treated as secret.
    """
    unsafe, display = normalize_zip_path(name)
    if unsafe:
        return "unsafe", display

    parts = display.split("/")
    directories = [part.casefold() for part in parts[:-1]]
    filename = parts[-1].casefold()
    suffix = PurePosixPath(filename).suffix.casefold()

    if (
        any(part in SECRET_DIR_NAMES for part in directories)
        or filename in SECRET_FILENAMES
        or suffix in SECRET_EXTENSIONS
    ):
        return "secret", display

    if (
        any(part in NOISY_DIR_NAMES for part in directories)
        or filename in NOISY_FILENAMES
        or suffix in NOISY_EXTENSIONS
        or _CONFIG_BACKUP.match(filename)
    ):
        return "noisy", display

    return "show", display
