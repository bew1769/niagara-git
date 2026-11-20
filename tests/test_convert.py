"""Textconv output for .bog and .dist bytes."""

from __future__ import annotations

import hashlib
import io
import unittest
import zipfile

from tests.support import ROOT  # noqa: F401

from niagara_git.convert import convert_bog, convert_dist, sha256_hex
from niagara_git.samples import (
    alarm_bytes,
    backup_bog_bytes,
    config_bog_bytes,
    config_xml,
    dist_bytes,
    history_bytes,
    keystore_bytes,
    pem_bytes,
)
from niagara_git.xmlfmt import pretty_xml
from niagara_git.zipio import build_zip


def _zip(entries: list[tuple[str, bytes]], date: tuple[int, int, int, int, int, int]) -> bytes:
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w", compression=zipfile.ZIP_STORED) as archive:
        for name, payload in entries:
            info = zipfile.ZipInfo(filename=name, date_time=date)
            info.compress_type = zipfile.ZIP_STORED
            archive.writestr(info, payload)
    return buffer.getvalue()


class BogTests(unittest.TestCase):
    def test_single_file_xml_is_pure_pretty_xml(self) -> None:
        xml = (
            b'<bajaObjectGraph version="4.0">'
            b'<p t="b:Folder" n="AHU1" h="2"/>'
            b"</bajaObjectGraph>"
        )
        text = convert_bog(build_zip([("file.xml", xml)]))
        self.assertTrue(text.startswith("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"))
        self.assertIn('<p h="2" n="AHU1" t="b:Folder"/>', text)
        self.assertNotIn("file.xml", text)
        self.assertEqual(text, pretty_xml(xml))

    def test_raw_xml_bog(self) -> None:
        xml = b'<bajaObjectGraph version="4.0"><p n="AHU1" h="2" t="b:Folder"/></bajaObjectGraph>'
        self.assertEqual(convert_bog(xml), pretty_xml(xml))

    def test_zip_metadata_does_not_affect_output(self) -> None:
        xml = b'<root><p n="A" v="1"/></root>'
        first = _zip(
            [("notes.txt", b"side"), ("file.xml", xml)],
            (2020, 1, 2, 3, 4, 5),
        )
        second = _zip(
            [("file.xml", xml), ("notes.txt", b"side")],
            (1999, 12, 31, 23, 59, 58),
        )
        self.assertNotEqual(first, second)
        self.assertEqual(convert_bog(first), convert_bog(second))
        text = convert_bog(first)
        self.assertIn("# entry file.xml", text)
        self.assertIn("# binary notes.txt", text)
        self.assertLess(text.index("# binary notes.txt"), text.index("# entry file.xml"))
        self.assertEqual(convert_bog(first), convert_bog(second))

    def test_sample_station_shows_points_links_and_hash_placeholder(self) -> None:
        text = convert_bog(config_bog_bytes(1))
        self.assertIn('n="demoPlant"', text)
        self.assertIn('n="SupplyTemp"', text)
        self.assertIn('n="SupplyFan"', text)
        self.assertIn('t="kitControl:GreaterThan"', text)
        self.assertIn('v="h:21"', text)
        self.assertIn('v="72.0"', text)
        self.assertIn("SYNTHETIC-NOT-A-REAL-HASH", text)
        self.assertNotIn('v="70.0"', text)
        self.assertNotIn('n="Occupied"', text)
        self.assertEqual(text, pretty_xml(config_xml(1).encode("utf-8")))
        self.assertEqual(text, convert_bog(config_bog_bytes(1)))

    def test_version_two_changes_setpoint_and_adds_link(self) -> None:
        before = convert_bog(config_bog_bytes(1))
        after = convert_bog(config_bog_bytes(2))
        self.assertIn('v="70.0"', after)
        self.assertIn('n="Occupied"', after)
        self.assertIn('v="h:24"', after)
        self.assertNotEqual(before, after)

    def test_opaque_bog_is_deterministic_and_has_no_payload_decode(self) -> None:
        data = b"not a bog \x00 encrypted-looking"
        first = convert_bog(data)
        self.assertEqual(first, convert_bog(data))
        self.assertIn("passphrase-protected", first)
        self.assertIn(sha256_hex(data), first)
        self.assertNotIn("\x00", first)


class DistTests(unittest.TestCase):
    def test_sample_dist_lists_shows_and_inlines_only_live_bog(self) -> None:
        data = dist_bytes(1)
        text = convert_dist(data)
        self.assertEqual(text, convert_dist(data))
        self.assertNotIn("\r", text)

        bog_path = "niagara_user_home/stations/demoPlant/config.bog"
        bog_bytes = config_bog_bytes(1)
        from niagara_git.samples import image_bytes, px_xml

        px_bytes = px_xml(1).encode("utf-8")
        image = image_bytes()
        self.assertIn(
            f"{len(bog_bytes)} {sha256_hex(bog_bytes)} {bog_path}",
            text,
        )
        self.assertIn(
            f"{len(px_bytes)} {sha256_hex(px_bytes)} niagara_user_home/stations/demoPlant/shared/px/AHU1.px",
            text,
        )
        self.assertIn(
            f"{len(image)} {sha256_hex(image)} niagara_user_home/stations/demoPlant/shared/images/mark.bin",
            text,
        )
        px_at = text.index("shared/px/AHU1.px")
        bog_at = text.index("config.bog")
        self.assertLess(bog_at, px_at)

        self.assertIn("## bog " + bog_path, text)
        self.assertIn('v="72.0"', text)
        self.assertNotIn("BACKUP-SHOULD-NOT-EXPAND", text)
        self.assertNotIn(hashlib.sha256(backup_bog_bytes()).hexdigest(), text)

        self.assertIn("history count=1 size=" + str(len(history_bytes(1))), text)
        self.assertIn("alarm count=1 size=" + str(len(alarm_bytes())), text)
        self.assertNotIn(b"synthetic-history-v1".decode(), text)
        self.assertNotIn(b"synthetic-alarm-db".decode(), text)

        self.assertIn(
            f"{len(keystore_bytes())} niagara_user_home/security/keystore.jceks",
            text,
        )
        self.assertNotIn(keystore_bytes().decode(), text)
        self.assertNotIn(hashlib.sha256(keystore_bytes()).hexdigest(), text)
        self.assertNotIn(pem_bytes().decode(), text)
        self.assertIn("niagara_user_home/certManagement/demo-client.pem", text)
        self.assertNotIn("## bog niagara_user_home/stations/demoPlant/config.backup", text)

    def test_entry_order_and_timestamps_do_not_change_dist_output(self) -> None:
        entries = [
            ("b/config.bog", config_bog_bytes(1)),
            ("a/shared/note.txt", b"hello"),
            ("history/h.bin", b"noise"),
            ("security/keystore.jceks", b"SECRET"),
        ]
        first = _zip(entries, (2018, 5, 1, 0, 0, 0))
        second = _zip(list(reversed(entries)), (2024, 8, 9, 10, 11, 12))
        self.assertEqual(convert_dist(first), convert_dist(second))

    def test_version_two_dist_shows_setpoint_px_and_history_size(self) -> None:
        before = convert_dist(dist_bytes(1))
        after = convert_dist(dist_bytes(2))
        self.assertIn('v="72.0"', before)
        self.assertIn('v="70.0"', after)
        self.assertIn('n="Occupied"', after)
        self.assertIn("size=" + str(len(history_bytes(1))), before)
        self.assertIn("size=" + str(len(history_bytes(2))), after)
        self.assertNotEqual(before, after)

    def test_unsafe_member_is_not_read(self) -> None:
        payload = b"SECRET-OUTSIDE"
        data = build_zip([("../outside.txt", payload), ("config.bog", config_bog_bytes(1))])
        text = convert_dist(data)
        self.assertNotIn(payload.decode(), text)
        self.assertNotIn(sha256_hex(payload), text)
        self.assertIn("unsafe", text)

    def test_encrypted_dist_is_opaque(self) -> None:
        data = b"this is not a zip"
        text = convert_dist(data)
        self.assertEqual(text, convert_dist(data))
        self.assertIn("passphrase-protected", text)
        self.assertIn(sha256_hex(data), text)
        self.assertTrue(text.endswith("\n"))
        self.assertFalse(text.endswith("\n\n"))


if __name__ == "__main__":
    unittest.main()
