# niagara-git

Git diffs for Tridium Niagara 4 station files. A station database (`.bog`) and a station backup (`.dist`) are zip archives, so Git otherwise shows them as opaque binaries. niagara-git teaches Git to print what changed: components, points, links, and properties.

It is a view only. The files stored in the repository stay the original zip archives, which is what Workbench and the Distribution File Installer expect.

## Security

A real `config.bog` contains the station's users. Niagara stores those passwords as encrypted hashes inside the station database (Station Security Guide, "Encrypted hashed passwords"). A `.dist` backup is a zip of the station folder and often the platform user home, including the certificate stores `keystore.jceks`, `cacerts.jceks`, `exemptions.tes`, FIPS `*.bcfks` files, and exported keys under `certManagement`.

niagara-git does not strip hashes out of a `.bog` diff. A password or permission change is a configuration change, and hiding it would also hide the audit trail. The starter ignore list does keep keystores, exported keys, history, alarms, and logs out of `git add`.

**Keep repositories that hold real stations private.** The sample station in this repo is synthetic. Its user password is the literal placeholder `SYNTHETIC-NOT-A-REAL-HASH`.

Passphrase-protected `.bog` and `.dist` files are encrypted. niagara-git cannot decrypt them. It prints the file size and a SHA-256 of the ciphertext so Git still notices that the bytes changed.

## What you get

| File | In the station | What `git diff` shows |
| --- | --- | --- |
| `*.bog` | Zip of Baja Object Graph XML, usually the member `file.xml` | Stable, pretty-printed XML |
| `*.dist` | Zip backup of the station (and often `niagara_user_home` / `niagara_home`) | Sorted file listing (path, size, SHA-256), then the XML of each `.bog` that is not noise or a secret |
| `*.px`, `*.nav`, `*.palette` | XML already (graphics, nav trees, module palettes) | Normal text diff |

History databases, alarm databases, logs, timestamped `config.backup.*.bog` copies, and certificate material are left out of the `.dist` listing. Noisy directories are summarized as a count and a total size, so a trend log can grow without a thousand-line diff. Secret files are listed by path and size only, with no checksum and no contents.

The text is deterministic: zip entry order, compression, and timestamps do not change the output. Attribute order and insignificant whitespace inside the XML are normalized. Component order, link order, and property values are not.

## Install

You need Git and Python 3.8 or newer on `PATH`. No third-party packages. On Windows, use Git for Windows (the textconv command runs through its shell).

From the station repository, or by passing its path:

```bash
# Linux, macOS, or Git Bash
/path/to/niagara-git/install.sh
/path/to/niagara-git/install.sh /path/to/station-repo
```

```powershell
# Windows PowerShell
powershell -ExecutionPolicy Bypass -File C:\tools\niagara-git\install.ps1
powershell -ExecutionPolicy Bypass -File C:\tools\niagara-git\install.ps1 D:\stations\demoPlant
```

The installer does two things:

1. Merges the starter `.gitattributes` and `.gitignore` blocks into the repo. Rules you already have outside the `# niagara-git:begin` / `# niagara-git:end` markers are left in place. Re-running the installer replaces only that block.
2. Sets local git config (not `--global`) so `git diff` runs this program:

```text
diff.niagara-bog.textconv
diff.niagara-dist.textconv
```

Commit `.gitattributes` and `.gitignore`. They travel with the repo. The textconv command does not: **each clone has to run the installer once**. Until then, Git still reports the zip files as binary.

Then version the station the usual way:

```bash
git add config.bog shared
git commit -m "Import demoPlant station"
git diff
git log -p -- config.bog
```

`shared/` is the Niagara 4 station home (graphics, nav, palettes). `config.bog` is the live database. Leave `history/`, `alarm/`, `security/`, and the timestamped `config.backup.*.bog` files untracked; the ignore file already does.

If a diff still says `Binary files differ`, run the installer again in that clone and check `git check-attr diff -- config.bog`. It should print `diff: niagara-bog`.

After you pull a newer niagara-git, run the installer again. The textconv command includes a format version so Git drops its cached conversion.

## Sample station

`examples/demoPlant` is a made-up air-handler station. Regenerate it with `python3 scripts/generate_fixtures.py`. It is not a commissioned database and is not meant to boot in Workbench.

* `station/` is the before folder: `config.bog` plus a Px view, a nav file, and a palette.
* `station-v2/` changes the supply-temp fallback from 72.0 to 70.0, adds an `Occupied` point, and links that point to `SupplyFan`.
* `backups/demoPlant-v1.dist` and `demoPlant-v2.dist` are zip backups with a fake history, alarm, log, keystore, and timestamped bog copy so the skip rules have something to skip.

<!-- example-diff:start -->
`git diff` on those two databases looks like this (textconv output; Git also prints its usual `diff --git` header):

```diff
--- config.bog (before)
+++ config.bog (after)
@@ -15,11 +15,11 @@
       <p h="21" m="control=control" n="SupplyTemp" t="control:NumericWritable">
         <p n="facets" t="b:Facets" v="units=u:fahrenheit;precision=i:1"/>
         <p f="s" n="out" t="b:StatusNumeric">
-          <p n="value" v="72.0"/>
+          <p n="value" v="70.0"/>
           <p n="status" v="0"/>
         </p>
         <p n="fallback" t="b:StatusNumeric">
-          <p n="value" v="72.0"/>
+          <p n="value" v="70.0"/>
         </p>
       </p>
       <p h="22" m="control=control" n="SupplyFan" t="control:BooleanWritable">
@@ -30,6 +30,12 @@
         <p n="fallback" t="b:StatusBoolean">
           <p n="value" v="false"/>
         </p>
+        <p n="Link" t="b:Link">
+          <p n="relationId" v="n:dataLink"/>
+          <p n="sourceOrd" v="h:24"/>
+          <p n="sourceSlotName" v="out"/>
+          <p n="targetSlotName" v="in"/>
+        </p>
       </p>
       <p h="23" m="kitControl=kitControl" n="FanEnable" t="kitControl:GreaterThan">
         <p n="Link" t="b:Link">
@@ -39,6 +45,15 @@
           <p n="targetSlotName" v="inA"/>
         </p>
       </p>
+      <p h="24" m="control=control" n="Occupied" t="control:BooleanWritable">
+        <p f="s" n="out" t="b:StatusBoolean">
+          <p n="value" v="true"/>
+          <p n="status" v="0"/>
+        </p>
+        <p n="fallback" t="b:StatusBoolean">
+          <p n="value" v="true"/>
+        </p>
+      </p>
     </p>
   </p>
 </bajaObjectGraph>
```
<!-- example-diff:end -->

A `.dist` of that station lists every kept file with size and SHA-256, then inlines `config.bog`. This is the skip summary. History, alarms, logs, and the timestamped bog copy are counts and sizes. Keystore lines are a size and a path, with no checksum.

<!-- example-dist:start -->
```text
# niagara-git dist format 1
# files 5
# skipped-noisy 4
# skipped-secret 4

## skipped-noisy
alarm count=1 size=18
config.backup count=1 size=227
history count=1 size=20
logs count=1 size=27

## skipped-secret
87 niagara_user_home/certManagement/demo-client.pem
34 niagara_user_home/security/cacerts.jceks
42 niagara_user_home/security/exemptions.tes
35 niagara_user_home/security/keystore.jceks
```
<!-- example-dist:end -->

## How a station folder maps

Niagara 4 splits the station directory (AX to N4 Migration Guide, Platform Guide):

* Protected station home: `config.bog`, `config.backup.<timestamp>.bog`, `alarm/`, `history/`, `dataRecovery/`, `provisioningNiagara/`, `niagaraDriver_nVirtual/`.
* Station home `shared/` (the `^` file ORD): Px, nav, images, and other files modules are allowed to write.

A platform backup `.dist` is a zip. Workbench names it `backup_<station>_<YYMMDD>_<HHMM>.dist` and stores it under the user home `backups` folder. Inside, station files sit under `niagara_user_home/stations/<name>/`. Some backups also carry `niagara_home`. niagara-git does not assume a single prefix; it classifies each member by its path.

## Limitations

* Textconv runs locally. GitHub and other hosts still show `.bog` and `.dist` as binary files in a pull request. Review those diffs on a machine where niagara-git is installed, or read the textconv output there.
* Blobs are still zips. This version does not use clean/smudge filters, so a checkout is unchanged and Workbench can open `config.bog` directly.
* Passphrase-encrypted bog and dist files are not decrypted.
* Baja reversible encoding is left as stored. Values are not decoded.
* Pretty-printing normalizes attribute order and insignificant whitespace, and it drops comments that sit outside the root element. A reorder of attributes alone does not show up. Element order is preserved.
* Documents with a DTD or entity declaration are not parsed (they are shown as normalized text) so a hostile file cannot expand entities inside the diff.
* A single zip member larger than 256 MiB is not inflated.
* Inside a `.dist`, Px, nav, and palette files are checksummed, not inlined. Diff those files from `shared/` if you need a line diff. Only `.bog` members are expanded.
* A `.dist` made while the station is running can contain a `config.backup.*.bog` that does not match the live `config.bog`. The timestamped copy is treated as noise; compare `config.bog`.
* The sample XML uses the public object-graph shape (`bajaObjectGraph`, `p` properties, `b:Link` slots). It is not a complete station.

## Roadmap

Not in this version:

* **Clean/smudge filters** that store a `.bog` as expanded XML in the repository and rebuild a lossless zip on checkout, so hosts that cannot run textconv still show a text diff.
* **A station snapshot / auto-commit script** that copies `config.bog` (and `shared/`) on a schedule or from a backup folder and commits it.

## Tests

```bash
python3 -m unittest discover -s tests -v
```

## File format notes

Checked against Tridium's Niagara 4 help, not only the summary in the project request:

* **Station database.** A station is defined by `config.bog` (Getting Started / Platform Guide). A `.bog` (Baja object graph) is a zip archive; the station XML is the `file.xml` member. The root element is `bajaObjectGraph`. Properties and components are `p` elements (`n` name, `t` type, `h` handle, `v` value); links are `b:Link` slots. Public module palettes are that same XML *without* the zip.
* **Graphics.** Px files and nav files are XML (Graphics Guide). Palettes shipped in modules are XML object graphs.
* **Backups.** A backup `.dist` is a zip of the station folder plus references the platform needs to restore it (Platform Guide, JACE backup guide). It can be protected with a file passphrase. History and alarms live beside `config.bog` and are not part of the shared graphics folder (AX to N4 Migration Guide).
* **Secrets.** User Key Store, User Trust Store, and the allowed-host list are `keystore.jceks`, `cacerts.jceks`, and `exemptions.tes` under the user home `security` folder. FIPS mode uses `*.bcfks`. Exported client certificates are `*.pem` (or `*.p12` / `*.pfx`) under `certManagement` (Station Security Guide).
