"""Deterministic pretty-printer for Baja Object Graph XML.

Niagara stores the station graph in ``file.xml`` inside the ``.bog`` zip.
Tag and document order are meaningful (component order, link order). Attribute
order and insignificant whitespace are not, so they are normalized. Values are
left as Niagara stored them, including reversible-encoding escapes.
"""

from __future__ import annotations

import re
from xml.etree import ElementTree as ET

# Refuse to expand DTD/entity declarations. Station XML does not need them,
# and expat entity expansion is the wrong tradeoff for a diff filter.
_DOCTYPE_OR_ENTITY = re.compile(br"<!DOCTYPE|<!ENTITY", re.IGNORECASE)


class XmlFormatError(Exception):
    """The bytes were XML-ish but not safe or not well-formed."""


def looks_like_xml(data: bytes) -> bool:
    head = data.lstrip(b"\xef\xbb\xbf \t\r\n")
    return head.startswith(b"<?xml") or head.startswith(b"<")


def pretty_xml(data: bytes) -> str:
    """Return canonical pretty XML ending with a single newline.

    Raises ``XmlFormatError`` when the document should be shown as text
    instead of parsed (DTD, or not well-formed).
    """
    if _DOCTYPE_OR_ENTITY.search(data[:8192]):
        raise XmlFormatError("DTD or entity declarations are not expanded")
    try:
        root = _parse(data)
    except ET.ParseError as exc:
        raise XmlFormatError(f"XML parse error: {exc}") from exc
    lines = ['<?xml version="1.0" encoding="UTF-8"?>', _serialize(root, 0)]
    return "\n".join(lines) + "\n"


def normalized_source(data: bytes, reason: str) -> str:
    """Newline-normalized source used when parsing is refused."""
    text = _decode(data).replace("\r\n", "\n").replace("\r", "\n")
    if text and not text.endswith("\n"):
        text += "\n"
    return f"# niagara-git: {reason}\n{text}"


def _parse(data: bytes) -> ET.Element:
    builder = ET.TreeBuilder(insert_comments=True, insert_pis=True)
    parser = ET.XMLParser(target=builder)
    parser.feed(data)
    return parser.close()


def _decode(data: bytes) -> str:
    if data.startswith(b"\xff\xfe") or data.startswith(b"\xfe\xff"):
        return data.decode("utf-16", errors="replace")
    if data.startswith(b"\xef\xbb\xbf"):
        return data.decode("utf-8-sig", errors="replace")
    match = re.match(
        br"""<\?xml[^>]*encoding=["']([A-Za-z0-9._-]+)["']""",
        data[:200],
    )
    encoding = match.group(1).decode("ascii") if match else "utf-8"
    try:
        return data.decode(encoding, errors="replace")
    except LookupError:
        return data.decode("utf-8", errors="replace")


def _serialize(elem: ET.Element, indent: int) -> str:
    pad = "  " * indent
    if elem.tag is ET.Comment:
        body = (elem.text or "").replace("--", "- -")
        return f"{pad}<!--{body}-->"
    if elem.tag is ET.ProcessingInstruction:
        body = (elem.text or "").replace("\r", " ").replace("\n", " ")
        return f"{pad}<?{body}?>"

    tag = str(elem.tag)
    attrs = "".join(
        f' {key}="{_escape_attr(value)}"'
        for key, value in sorted(elem.attrib.items(), key=lambda item: item[0])
    )
    children = list(elem)
    text = _significant(elem.text)
    if not children and text is None:
        line = f"{pad}<{tag}{attrs}/>"
    elif not children:
        line = f"{pad}<{tag}{attrs}>{_escape_text(text)}</{tag}>"
    else:
        body = [f"{pad}<{tag}{attrs}>"]
        if text is not None:
            body.append(f"{pad}  {_escape_text(text)}")
        for child in children:
            body.append(_serialize(child, indent + 1))
            tail = _significant(child.tail)
            if tail is not None:
                body.append(f"{pad}  {_escape_text(tail)}")
        body.append(f"{pad}</{tag}>")
        line = "\n".join(body)
    return line


def _significant(text: str | None) -> str | None:
    if text is None or text.strip() == "":
        return None
    return text.strip()


def _escape_text(text: str) -> str:
    return (
        text.replace("&", "&amp;")
        .replace("<", "&lt;")
        .replace(">", "&gt;")
        .replace("\r", "&#13;")
        .replace("\n", "&#10;")
    )


def _escape_attr(text: str) -> str:
    return (
        text.replace("&", "&amp;")
        .replace("<", "&lt;")
        .replace(">", "&gt;")
        .replace('"', "&quot;")
        .replace("\r", "&#13;")
        .replace("\n", "&#10;")
        .replace("\t", "&#9;")
    )
