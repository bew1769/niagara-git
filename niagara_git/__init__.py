"""Git textconv support for Tridium Niagara 4 station files.

``.bog`` station databases and ``.dist`` backups are zip archives. This
package turns them into a deterministic text view for ``git diff``.
"""

__version__ = "0.1.0"

# Bumped when textconv output changes. The installer embeds this in the
# git textconv command so a re-install invalidates Git's textconv cache.
FORMAT_VERSION = 1
