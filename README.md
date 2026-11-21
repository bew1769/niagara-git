# niagara-git

Git diffs for Tridium Niagara 4 station files. A station database (`.bog`) and a station backup (`.dist`) are zip archives, so Git otherwise shows them as opaque binaries. niagara-git teaches Git to print what changed: components, points, links, and properties.

The program is one self-contained binary. A Workbench PC does not need Python, or any other runtime, installed. The diff is a view only. The files stored in the repository stay the original zip archives, which is what Workbench and the Distribution File Installer expect.

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

You need Git. On Windows, use Git for Windows: the textconv command runs through its shell. There is nothing else to install. The starter `.gitattributes` and `.gitignore` templates are embedded in the binary, so the download is a single file.

Download the binary for your machine from the latest [GitHub release](https://github.com/bew1769/niagara-git/releases/latest):

| Machine | File |
| --- | --- |
| Windows 64-bit | `niagara-git-windows-amd64.exe` |
| Linux 64-bit | `niagara-git-linux-amd64` |
| Linux ARM 64-bit | `niagara-git-linux-arm64` |
| macOS Apple silicon | `niagara-git-darwin-arm64` |
| macOS Intel | `niagara-git-darwin-amd64` |

Linux:

```bash
curl -fsSL -o niagara-git https://github.com/bew1769/niagara-git/releases/latest/download/niagara-git-linux-amd64
chmod +x niagara-git
./niagara-git install
./niagara-git install /path/to/station-repo
```

Windows PowerShell:

```powershell
Invoke-WebRequest https://github.com/bew1769/niagara-git/releases/latest/download/niagara-git-windows-amd64.exe -OutFile niagara-git.exe
.\niagara-git.exe install
.\niagara-git.exe install D:\stations\demoPlant
```

`install.sh` and `install.ps1` are thin wrappers around that same command. They use a `niagara-git` binary sitting next to the script or already on `PATH`. If they cannot find one, they download the release that matches this machine, then run `niagara-git install`.

```bash
./install.sh
./install.sh /path/to/station-repo
```

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1
powershell -ExecutionPolicy Bypass -File C:\tools\niagara-git\install.ps1 D:\stations\demoPlant
```

`niagara-git install` does two things:

1. Merges the starter `.gitattributes` and `.gitignore` blocks into the repo. Rules you already have outside the `# niagara-git:begin` / `# niagara-git:end` markers are left in place. Re-running the installer replaces only that block.
2. Sets local git config (not `--global`) so `git diff` runs this binary:

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

`examples/demoPlant` is a made-up air-handler station checked into this repository. It is not a commissioned database and is not meant to boot in Workbench.

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

* Textconv runs locally. GitHub and other hosts do not run the clean filter or textconv. With the default install, a pull request still shows `.bog` and `.dist` as binary. With `--store-xml`, the pull request shows the stored text container (the original `file.xml` bytes plus any other zip entries) instead of a zip.
* `--store-xml` is off unless you ask for it. Checkout rebuilds a `.bog` whose entry names, order, and uncompressed bytes match the station Niagara saved. The zip timestamp and compression are normalized so the git object stays stable.
* `.dist` backups are not stored with clean/smudge. A distribution archive contains history databases, alarm databases, and keystores. Putting that through a clean filter would commit those bytes and would change on every backup. Keep diffing `.dist` files with textconv. `snapshot` extracts `config.bog` (and, with `--shared`, the non-secret `shared/` files) instead of committing the `.dist` itself.
* Passphrase-encrypted bog and dist files are not decrypted.
* Baja reversible encoding is left as stored. Values are not decoded.
* Pretty-printing normalizes attribute order and insignificant whitespace, and it drops comments that sit outside the root element. A reorder of attributes alone does not show up. Element order is preserved.
* Documents with a DTD or entity declaration are not parsed (they are shown as normalized text) so a hostile file cannot expand entities inside the diff.
* A single zip member larger than 256 MiB is not inflated.
* Inside a `.dist`, Px, nav, and palette files are checksummed, not inlined. Diff those files from `shared/` if you need a line diff. Only `.bog` members are expanded.
* A `.dist` made while the station is running can contain a `config.backup.*.bog` that does not match the live `config.bog`. The timestamped copy is treated as noise; compare `config.bog`.
* The sample XML uses the public object-graph shape (`bajaObjectGraph`, `p` properties, `b:Link` slots). It is not a complete station.

## Storing a .bog as text

By default Git keeps the zip. `git diff` uses the textconv driver and Workbench opens `config.bog` directly.

`niagara-git install --store-xml` opts into a clean/smudge filter. The object in the repository is a small text container: every zip entry, in the original order, with its original bytes. `git checkout` runs `niagara-git smudge` and writes a `.bog` zip again. The same station produces the same container, including when Workbench only changed the zip timestamp or compression. `niagara-git install --no-store-xml` turns the filter off. A plain `niagara-git install` leaves the choice as it is.

`.dist` files are not included. They bundle histories, alarms, and keystores, and those do not belong in a clean filter. They stay on the textconv driver.

After you turn `--store-xml` on in a repo that already committed zip blobs, `config.bog` shows as modified once. Commit it to switch the stored form. Older commits still check out: smudge leaves an existing zip blob unchanged.

## Station snapshots

`niagara-git snapshot` copies a station into a git repository and commits only when the bytes changed.

```bash
niagara-git snapshot --source C:\ProgramData\Niagara4.14\stations\demoPlant --repo D:\git\demoPlant --shared
niagara-git snapshot --source /opt/niagara/stations/demoPlant --repo /srv/git/demoPlant --shared --push
niagara-git snapshot --source D:\backups --repo D:\git\demoPlant
```

`--source` may be a station directory (the folder that contains `config.bog`), a `.bog` file, a `.dist` file, or a folder of backups. A backup folder uses the newest `.bog` or `.dist` by modification time. From a `.dist`, snapshot commits `config.bog` and, with `--shared`, the `shared/` files. It does not commit the `.dist` itself, and it does not commit keystores, certificates, histories, alarms, or logs. The embedded ignore rules are installed into the snapshot repo if they are not already there.

The commit message is `snapshot` plus a UTC timestamp, for example `snapshot 2026-11-21T00:15:00Z`. `--push` pushes the current branch to `origin`, and sets the upstream on the first push.

Run `niagara-git install` once in that repo (snapshot does this) so `git diff` can read `.bog` files. Add `--store-xml` on that install if you want the text container.

Schedule it once an hour.

Windows Task Scheduler:

```text
schtasks /Create /F /TN "Niagara git snapshot" /SC HOURLY /ST 00:15 /TR "\"C:\Tools\niagara-git.exe\" snapshot --source \"C:\ProgramData\Niagara4.14\stations\demoPlant\" --repo \"D:\git\demoPlant\" --shared"
```

cron:

```text
15 * * * * /usr/local/bin/niagara-git snapshot --source /opt/niagara/stations/demoPlant --repo /srv/git/demoPlant --shared
```

systemd, in `/etc/systemd/system/niagara-snapshot.service`:

```text
[Service]
Type=oneshot
ExecStart=/usr/local/bin/niagara-git snapshot --source /opt/niagara/stations/demoPlant --repo /srv/git/demoPlant --shared
```

and `/etc/systemd/system/niagara-snapshot.timer`:

```text
[Timer]
OnCalendar=hourly
Persistent=true

[Install]
WantedBy=timers.target
```

Then `systemctl enable --now niagara-snapshot.timer`.

## Tests

```bash
go test ./...
```

GitHub Actions runs that on Linux and Windows. Pushing a `v*` tag also cross-compiles the five release binaries and attaches them to the GitHub release.

Building from a checkout, if you already have the Go toolchain:

```bash
go build -o niagara-git .
./niagara-git install
```

## File format notes

Checked against Tridium's Niagara 4 help, not only the summary in the project request:

* **Station database.** A station is defined by `config.bog` (Getting Started / Platform Guide). A `.bog` (Baja object graph) is a zip archive; the station XML is the `file.xml` member. The root element is `bajaObjectGraph`. Properties and components are `p` elements (`n` name, `t` type, `h` handle, `v` value); links are `b:Link` slots. Public module palettes are that same XML *without* the zip.
* **Graphics.** Px files and nav files are XML (Graphics Guide). Palettes shipped in modules are XML object graphs.
* **Backups.** A backup `.dist` is a zip of the station folder plus references the platform needs to restore it (Platform Guide, JACE backup guide). It can be protected with a file passphrase. History and alarms live beside `config.bog` and are not part of the shared graphics folder (AX to N4 Migration Guide).
* **Secrets.** User Key Store, User Trust Store, and the allowed-host list are `keystore.jceks`, `cacerts.jceks`, and `exemptions.tes` under the user home `security` folder. FIPS mode uses `*.bcfks`. Exported client certificates are `*.pem` (or `*.p12` / `*.pfx`) under `certManagement` (Station Security Guide).
