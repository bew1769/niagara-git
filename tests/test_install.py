"""Installer, templates, sample fixtures, and a real git diff."""

from __future__ import annotations

import os
import shlex
import subprocess
import tempfile
import unittest
from pathlib import Path

from tests.support import ROOT

from niagara_git import FORMAT_VERSION
from niagara_git.convert import convert_bog, convert_dist
from niagara_git.install import install, merge_block, shell_quote, textconv_command
from niagara_git.samples import config_bog_bytes, dist_bytes, write_examples


def _git(repo: Path, *args: str, check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", "-C", str(repo), *args],
        check=check,
        capture_output=True,
        text=True,
    )


class InstallTests(unittest.TestCase):
    def test_shell_quote_and_textconv_command(self) -> None:
        self.assertEqual(shell_quote("a b"), "'a b'")
        self.assertEqual(shell_quote("it's"), "'it'\\''s'")
        command = textconv_command("bog")
        self.assertEqual(
            shlex.split(command)[-3:],
            ["--format", str(FORMAT_VERSION), "bog"],
        )
        self.assertIn("dist", textconv_command("dist"))

    def test_merge_block_is_idempotent_and_replaces(self) -> None:
        block = "# niagara-git:begin\n*.bog diff=niagara-bog\n# niagara-git:end\n"
        first = merge_block("# local\n", block)
        second = merge_block(first, block)
        self.assertEqual(first, second)
        self.assertEqual(first.count("# niagara-git:begin"), 1)
        updated = block.replace("niagara-bog", "niagara-bog-v2")
        third = merge_block(first, updated)
        self.assertIn("niagara-bog-v2", third)
        self.assertNotIn("diff=niagara-bog\n", third)
        self.assertTrue(third.startswith("# local\n"))

    def test_install_registers_drivers_and_ignore_rules(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            _git(repo, "init")
            _git(repo, "config", "user.email", "niagara-git@example.com")
            _git(repo, "config", "user.name", "niagara-git tests")
            install(repo)

            bog_cmd = _git(repo, "config", "--local", "--get", "diff.niagara-bog.textconv")
            dist_cmd = _git(repo, "config", "--local", "--get", "diff.niagara-dist.textconv")
            self.assertEqual(bog_cmd.stdout.strip(), textconv_command("bog"))
            self.assertEqual(dist_cmd.stdout.strip(), textconv_command("dist"))
            self.assertEqual(
                _git(repo, "config", "--local", "--get", "diff.niagara-bog.cachetextconv").stdout.strip(),
                "true",
            )

            # Re-running must not duplicate the template block.
            install(repo)
            attributes = (repo / ".gitattributes").read_text(encoding="utf-8")
            gitignore = (repo / ".gitignore").read_text(encoding="utf-8")
            self.assertEqual(attributes.count("# niagara-git:begin"), 1)
            self.assertEqual(gitignore.count("# niagara-git:begin"), 1)

            bog = repo / "config.bog"
            bog.write_bytes(config_bog_bytes(1))
            px = repo / "shared" / "px" / "AHU1.px"
            px.parent.mkdir(parents=True)
            px.write_text("<px/>\n", encoding="utf-8")
            (repo / "file.nav").write_text("<nav/>\n", encoding="utf-8")
            (repo / "module.palette").write_text("<bajaObjectGraph/>\n", encoding="utf-8")
            (repo / "station.dist").write_bytes(b"PK\x03\x04not-really")

            attr = _git(
                repo,
                "check-attr",
                "diff",
                "--",
                "config.bog",
                "station.dist",
                "shared/px/AHU1.px",
                "file.nav",
                "module.palette",
            ).stdout
            self.assertIn("config.bog: diff: niagara-bog", attr)
            self.assertIn("station.dist: diff: niagara-dist", attr)
            self.assertIn("shared/px/AHU1.px: diff: set", attr)
            self.assertIn("file.nav: diff: set", attr)
            self.assertIn("module.palette: diff: set", attr)

            ignored = [
                "history/zone.bin",
                "alarm/alarmData.bin",
                "logs/station.out",
                "security/keystore.jceks",
                "certManagement/client.pem",
                "dataRecovery/state.bin",
                "provisioningNiagara/job.bin",
                "config.backup.20200101.bog",
                "config_backup_200548_0042.bog",
                "trusted/extra.jceks",
            ]
            tracked = ["config.bog", "shared/px/AHU1.px"]
            for relative in ignored:
                path = repo / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(b"x")
                result = _git(repo, "check-ignore", "--", relative, check=False)
                self.assertEqual(result.returncode, 0, relative)
            for relative in tracked:
                result = _git(repo, "check-ignore", "--", relative, check=False)
                self.assertEqual(result.returncode, 1, relative)

    def test_git_diff_uses_textconv(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            _git(repo, "init")
            _git(repo, "config", "user.email", "niagara-git@example.com")
            _git(repo, "config", "user.name", "niagara-git tests")
            _git(repo, "config", "core.autocrlf", "false")
            _git(repo, "config", "color.ui", "false")
            install(repo)
            bog = repo / "config.bog"
            bog.write_bytes(config_bog_bytes(1))
            _git(repo, "add", "config.bog", ".gitattributes", ".gitignore")
            _git(repo, "commit", "-m", "station v1")
            bog.write_bytes(config_bog_bytes(2))
            env = os.environ.copy()
            env["GIT_PAGER"] = "cat"
            diff = subprocess.run(
                ["git", "-C", str(repo), "diff", "--", "config.bog"],
                check=True,
                capture_output=True,
                text=True,
                env=env,
            ).stdout
            again = subprocess.run(
                ["git", "-C", str(repo), "diff", "--", "config.bog"],
                check=True,
                capture_output=True,
                text=True,
                env=env,
            ).stdout
            self.assertEqual(diff, again)
            self.assertNotIn("Binary files", diff)
            self.assertIn('-          <p n="value" v="72.0"/>', diff)
            self.assertIn('+          <p n="value" v="70.0"/>', diff)
            self.assertIn('+      <p h="24" m="control=control" n="Occupied" t="control:BooleanWritable">', diff)
            self.assertIn('+          <p n="sourceOrd" v="h:24"/>', diff)


class SampleTests(unittest.TestCase):
    def test_generator_matches_committed_fixtures(self) -> None:
        committed = ROOT / "examples" / "demoPlant"
        with tempfile.TemporaryDirectory() as tmp:
            fresh = Path(tmp)
            write_examples(fresh)
            files = [
                Path("station/config.bog"),
                Path("station/shared/px/AHU1.px"),
                Path("station/shared/nav/demoPlant.nav"),
                Path("station/shared/palette/demoPlant.palette"),
                Path("station-v2/config.bog"),
                Path("station-v2/shared/px/AHU1.px"),
                Path("backups/demoPlant-v1.dist"),
                Path("backups/demoPlant-v2.dist"),
            ]
            for relative in files:
                self.assertEqual(
                    (committed / relative).read_bytes(),
                    (fresh / relative).read_bytes(),
                    str(relative),
                )

    def test_committed_bogs_match_converter(self) -> None:
        v1 = (ROOT / "examples" / "demoPlant" / "station" / "config.bog").read_bytes()
        v2 = (ROOT / "examples" / "demoPlant" / "station-v2" / "config.bog").read_bytes()
        self.assertEqual(v1, config_bog_bytes(1))
        self.assertEqual(convert_bog(v1), convert_bog(config_bog_bytes(1)))
        self.assertIn('v="70.0"', convert_bog(v2))

    def test_readme_diff_matches_textconv(self) -> None:
        import difflib

        before = convert_bog(config_bog_bytes(1)).splitlines(keepends=True)
        after = convert_bog(config_bog_bytes(2)).splitlines(keepends=True)
        diff = "".join(
            difflib.unified_diff(
                before,
                after,
                fromfile="config.bog (before)",
                tofile="config.bog (after)",
            )
        )
        readme = (ROOT / "README.md").read_text(encoding="utf-8")
        start = readme.index("<!-- example-diff:start -->")
        end = readme.index("<!-- example-diff:end -->")
        block = readme[start:end]
        self.assertIn('-          <p n="value" v="72.0"/>', block)
        self.assertIn('+          <p n="value" v="70.0"/>', block)
        self.assertIn("+      <p h=\"24\" m=\"control=control\" n=\"Occupied\"", block)
        for line in block.splitlines():
            if line.startswith(("+", "-", " ")) and not line.startswith(("+++", "---")):
                self.assertIn(line, diff, line)
            if line.startswith("@@"):
                self.assertIn(line, diff, line)

    def test_readme_dist_excerpt_matches_textconv(self) -> None:
        text = convert_dist(dist_bytes(1))
        readme = (ROOT / "README.md").read_text(encoding="utf-8")
        start = readme.index("<!-- example-dist:start -->")
        end = readme.index("<!-- example-dist:end -->")
        block = readme[start:end]
        for line in block.splitlines():
            stripped = line.strip()
            if not stripped or stripped.startswith("<!--") or stripped.startswith("```"):
                continue
            self.assertIn(line, text, line)


class TemplateDriftTests(unittest.TestCase):
    def test_repo_copies_contain_templates(self) -> None:
        attributes = (ROOT / "templates" / "gitattributes").read_text(encoding="utf-8").strip()
        gitignore = (ROOT / "templates" / "gitignore").read_text(encoding="utf-8").strip()
        self.assertIn(attributes, (ROOT / ".gitattributes").read_text(encoding="utf-8"))
        self.assertIn(gitignore, (ROOT / ".gitignore").read_text(encoding="utf-8"))


if __name__ == "__main__":
    unittest.main()
