"""Synthetic Niagara 4 station used by tests and the README.

Nothing here is a customer station. Names, hashes, and certificate bytes are
placeholders. The XML follows the public Baja Object Graph shape (``bajaObjectGraph``
with ``p`` / ``a`` elements) and the station-backup layout documented by Tridium,
but it is not a commissioned database and is not expected to boot in Workbench.
"""

from __future__ import annotations

from pathlib import Path

from niagara_git.zipio import build_zip

STATION = "demoPlant"
_PREFIX = f"niagara_user_home/stations/{STATION}"


def config_xml(version: int) -> str:
    if version not in (1, 2):
        raise ValueError(version)
    fallback = "72.0" if version == 1 else "70.0"
    occupied = ""
    fan_link = ""
    if version == 2:
        occupied = """
      <p h="24" m="control=control" n="Occupied" t="control:BooleanWritable">
        <p f="s" n="out" t="b:StatusBoolean">
          <p n="value" v="true"/>
          <p n="status" v="0"/>
        </p>
        <p n="fallback" t="b:StatusBoolean">
          <p n="value" v="true"/>
        </p>
      </p>"""
        fan_link = """
        <p n="Link" t="b:Link">
          <p n="relationId" v="n:dataLink"/>
          <p n="sourceOrd" v="h:24"/>
          <p n="sourceSlotName" v="out"/>
          <p n="targetSlotName" v="in"/>
        </p>"""
    return f"""<?xml version="1.0" encoding="UTF-8"?>
<bajaObjectGraph FIPSEnabled="false" reversibleEncodingKeySource="none" reversibleEncodingValidator="[null.1]=" version="4.0">
  <p h="1" m="b=baja" n="demoPlant" t="b:Station">
    <p n="displayName" v="Demo Plant (synthetic)"/>
    <p h="10" n="Services" t="b:Folder">
      <p h="11" m="b=baja" n="UserService" t="b:UserService">
        <p h="12" n="admin" t="b:User">
          <p n="fullName" v="Demo Plant Operator"/>
          <p n="password" t="b:Password" v="SYNTHETIC-NOT-A-REAL-HASH"/>
          <p n="comment" v="Synthetic user for the niagara-git sample. Not a real password hash."/>
        </p>
      </p>
    </p>
    <p h="20" n="AHU1" t="b:Folder">
      <p h="21" m="control=control" n="SupplyTemp" t="control:NumericWritable">
        <p n="facets" t="b:Facets" v="units=u:fahrenheit;precision=i:1"/>
        <p f="s" n="out" t="b:StatusNumeric">
          <p n="value" v="{fallback}"/>
          <p n="status" v="0"/>
        </p>
        <p n="fallback" t="b:StatusNumeric">
          <p n="value" v="{fallback}"/>
        </p>
      </p>
      <p h="22" m="control=control" n="SupplyFan" t="control:BooleanWritable">
        <p f="s" n="out" t="b:StatusBoolean">
          <p n="value" v="false"/>
          <p n="status" v="0"/>
        </p>
        <p n="fallback" t="b:StatusBoolean">
          <p n="value" v="false"/>
        </p>{fan_link}
      </p>
      <p h="23" m="kitControl=kitControl" n="FanEnable" t="kitControl:GreaterThan">
        <p n="Link" t="b:Link">
          <p n="relationId" v="n:dataLink"/>
          <p n="sourceOrd" v="h:21"/>
          <p n="sourceSlotName" v="out"/>
          <p n="targetSlotName" v="inA"/>
        </p>
      </p>{occupied}
    </p>
  </p>
</bajaObjectGraph>
"""


def px_xml(version: int) -> str:
    title = "AHU-1 Supply Temp" if version == 1 else "AHU-1 Supply Temp (occupied)"
    return f"""<?xml version="1.0" encoding="UTF-8"?>
<!-- Synthetic Presentation XML. Not a commissioned Px view. -->
<px version="1.0">
  <canvas name="AHU1">
    <label name="title" text="{title}"/>
    <boundLabel name="supplyTemp" ord="station:|slot:/AHU1/SupplyTemp"/>
  </canvas>
</px>
"""


def nav_xml() -> str:
    return """<?xml version="1.0" encoding="UTF-8"?>
<nav version="1.0">
  <node name="Home" ord="station:|slot:/">
    <node name="AHU-1" ord="station:|slot:/AHU1" px="file:^px/AHU1.px"/>
  </node>
</nav>
"""


def palette_xml() -> str:
    # Module palettes ship as plain bajaObjectGraph XML (not zipped), same shape
    # as a .bog's file.xml. See public module.palette files.
    return """<?xml version="1.0" encoding="UTF-8"?>
<bajaObjectGraph version="4.0">
  <p m="b=baja" n="demoPalette" t="b:Folder">
    <p m="control=control" n="SupplyTemp" t="control:NumericWritable"/>
    <p m="control=control" n="SupplyFan" t="control:BooleanWritable"/>
  </p>
</bajaObjectGraph>
"""


def backup_bog_bytes() -> bytes:
    xml = """<?xml version="1.0" encoding="UTF-8"?>
<bajaObjectGraph version="4.0">
  <p n="note" v="BACKUP-SHOULD-NOT-EXPAND"/>
</bajaObjectGraph>
"""
    return build_zip([("file.xml", xml.encode("utf-8"))])


def config_bog_bytes(version: int) -> bytes:
    # file.xml is the usual member name inside a .bog zip.
    return build_zip([("file.xml", config_xml(version).encode("utf-8"))])


def history_bytes(version: int) -> bytes:
    if version == 1:
        return b"synthetic-history-v1"
    return b"synthetic-history-v2-extra-sample"


def alarm_bytes() -> bytes:
    return b"synthetic-alarm-db"


def keystore_bytes() -> bytes:
    return b"synthetic-keystore-not-a-real-jceks"


def cacerts_bytes() -> bytes:
    return b"synthetic-cacerts-not-a-real-jceks"


def exemptions_bytes() -> bytes:
    return b"synthetic-exemptions-not-a-real-allow-list"


def pem_bytes() -> bytes:
    return (
        b"-----BEGIN CERTIFICATE-----\n"
        b"SYNTHETIC-NOT-A-REAL-CERTIFICATE\n"
        b"-----END CERTIFICATE-----\n"
    )


def log_bytes() -> bytes:
    return b"synthetic station log line\n"


def image_bytes() -> bytes:
    return b"\x89PNG synthetic mark"


def dist_entries(version: int) -> list[tuple[str, bytes]]:
    """Station backup members in a stable write order.

    Paths follow the platform backup layout: ``niagara_user_home/stations/<name>``
    plus the user-home security store and a platform log under ``niagara_home``.
    """
    return [
        (f"{_PREFIX}/config.bog", config_bog_bytes(version)),
        (f"{_PREFIX}/config.backup.20200101_0000.bog", backup_bog_bytes()),
        (f"{_PREFIX}/alarm/alarmData.bin", alarm_bytes()),
        (f"{_PREFIX}/history/AHU1_SupplyTemp.bin", history_bytes(version)),
        (f"{_PREFIX}/shared/px/AHU1.px", px_xml(version).encode("utf-8")),
        (f"{_PREFIX}/shared/nav/demoPlant.nav", nav_xml().encode("utf-8")),
        (f"{_PREFIX}/shared/palette/demoPlant.palette", palette_xml().encode("utf-8")),
        (f"{_PREFIX}/shared/images/mark.bin", image_bytes()),
        ("niagara_user_home/security/keystore.jceks", keystore_bytes()),
        ("niagara_user_home/security/cacerts.jceks", cacerts_bytes()),
        ("niagara_user_home/security/exemptions.tes", exemptions_bytes()),
        ("niagara_user_home/certManagement/demo-client.pem", pem_bytes()),
        ("niagara_home/logs/station.out", log_bytes()),
    ]


def dist_bytes(version: int) -> bytes:
    return build_zip(dist_entries(version))


def write_examples(dest: Path) -> None:
    """Write the sample station, the edited copy, and two ``.dist`` backups."""
    station = dest / "station"
    edited = dest / "station-v2"
    backups = dest / "backups"
    for path in (station, edited, backups):
        path.mkdir(parents=True, exist_ok=True)

    (station / "config.bog").write_bytes(config_bog_bytes(1))
    shared = station / "shared"
    (shared / "px").mkdir(parents=True, exist_ok=True)
    (shared / "nav").mkdir(parents=True, exist_ok=True)
    (shared / "palette").mkdir(parents=True, exist_ok=True)
    (shared / "px" / "AHU1.px").write_text(px_xml(1), encoding="utf-8", newline="\n")
    (shared / "nav" / "demoPlant.nav").write_text(nav_xml(), encoding="utf-8", newline="\n")
    (shared / "palette" / "demoPlant.palette").write_text(
        palette_xml(), encoding="utf-8", newline="\n"
    )

    (edited / "config.bog").write_bytes(config_bog_bytes(2))
    edited_shared = edited / "shared"
    (edited_shared / "px").mkdir(parents=True, exist_ok=True)
    (edited_shared / "px" / "AHU1.px").write_text(px_xml(2), encoding="utf-8", newline="\n")

    (backups / "demoPlant-v1.dist").write_bytes(dist_bytes(1))
    (backups / "demoPlant-v2.dist").write_bytes(dist_bytes(2))
