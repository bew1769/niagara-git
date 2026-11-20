#!/usr/bin/env python3
"""Write the synthetic demo station under examples/demoPlant."""

from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))

from niagara_git.samples import write_examples


def main() -> None:
    dest = Path(sys.argv[1]) if len(sys.argv) > 1 else ROOT / "examples" / "demoPlant"
    write_examples(dest)
    print(f"wrote {dest}")


if __name__ == "__main__":
    main()
