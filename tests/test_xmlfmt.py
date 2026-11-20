"""Pretty-printer behavior for Baja Object Graph XML."""

from __future__ import annotations

import unittest

from tests.support import ROOT  # noqa: F401  (puts the repo on sys.path)

from niagara_git.xmlfmt import XmlFormatError, pretty_xml


class PrettyXmlTests(unittest.TestCase):
    def test_sorts_attributes_and_indents(self) -> None:
        raw = (
            b'<?xml version="1.0" encoding="UTF-8"?>'
            b'<bajaObjectGraph version="4.0">'
            b'<p t="b:Folder" n="AHU1" h="2">'
            b"<!-- supply air -->"
            b'<p n="SupplyTemp" t="control:NumericWritable" h="3">'
            b'<p v="72.0" n="fallback"/>'
            b"</p></p></bajaObjectGraph>"
        )
        expected = """<?xml version="1.0" encoding="UTF-8"?>
<bajaObjectGraph version="4.0">
  <p h="2" n="AHU1" t="b:Folder">
    <!-- supply air -->
    <p h="3" n="SupplyTemp" t="control:NumericWritable">
      <p n="fallback" v="72.0"/>
    </p>
  </p>
</bajaObjectGraph>
"""
        self.assertEqual(pretty_xml(raw), expected)

    def test_preserves_element_order(self) -> None:
        raw = (
            b"<root>"
            b'<p n="second"/>'
            b'<p n="first"/>'
            b"</root>"
        )
        text = pretty_xml(raw)
        self.assertLess(text.index('n="second"'), text.index('n="first"'))

    def test_escapes_attribute_text(self) -> None:
        raw = b'<root><p n="note" v="a&amp;b&quot;c&lt;d"/></root>'
        text = pretty_xml(raw)
        self.assertIn('v="a&amp;b&quot;c&lt;d"', text)

    def test_idempotent(self) -> None:
        raw = b'<root><p n="A" h="1"><p v="x" n="b"/></p></root>'
        once = pretty_xml(raw)
        twice = pretty_xml(once.encode("utf-8"))
        self.assertEqual(once, twice)

    def test_crlf_and_attribute_order_normalize(self) -> None:
        a = b'<root><p b="1" a="2"/></root>'
        b = b'<root>\r\n  <p a="2" b="1"/>\r\n</root>'
        self.assertEqual(pretty_xml(a), pretty_xml(b))
        self.assertNotIn("\r", pretty_xml(a))

    def test_refuses_doctype(self) -> None:
        raw = b'<!DOCTYPE bajaObjectGraph [<!ENTITY x "y">]><root>&x;</root>'
        with self.assertRaises(XmlFormatError):
            pretty_xml(raw)

    def test_newline_in_attribute_stays_on_one_line(self) -> None:
        raw = b'<root><p v="line1&#10;line2"/></root>'
        text = pretty_xml(raw)
        value_line = next(line for line in text.splitlines() if "line1" in line)
        self.assertIn("&#10;", value_line)
        self.assertNotIn("\nline2", text)


if __name__ == "__main__":
    unittest.main()
