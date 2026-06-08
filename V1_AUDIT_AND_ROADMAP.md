# dmg — v1 Audit & Roadmap (Wails v3 Integration)

_Audit date: 2026-06-08. Audited at commit `b6ee544` with uncommitted WIP (cross-platform
build tags + `IconPositions`)._

> **Update 2026-06-08 — pure-Go cross-platform DMG decided + UDIF container POC landed.**
> Following the audit we decided to pursue a **pure-Go, cross-compilable** DMG backend so
> Wails can build macOS DMGs on Linux/Windows CI. Because libdmg-hfsplus is **GPLv3**
> (incompatible with this MIT library and with embedding in MIT Wails), the implementation is
> **clean-room from public format specs** (Apple TN1150 for HFS+, documented UDIF/`koly` for the
> container) — no GPL source consulted.
>
> **Done:** the `udif/` package — a pure-Go UDZO (.dmg) container writer. It wraps a raw
> filesystem image, computes all three UDIF CRC-32 checksums, and produces a file that
> `hdiutil verify` accepts and macOS **mounts and reads** (verified by an oracle test). It
> cross-compiles cleanly to linux/amd64, linux/arm64, and windows/amd64. Key format facts
> nailed during the POC: ImageVariant must be `2` (whole-disk device), and the BLKX checksum
> covers only the **stored** (non-zero-fill) chunk data, not the full image.
>
> **Done — `hfsplus/` writer is functional.** `hfsplus.Build(*Volume)` writes a complete
> case-sensitive HFSX volume from an arbitrary directory tree with **no host OS calls**:
> volume header + alternate, allocation bitmap (multi-block aware), a **bulk-loaded multi-node
> catalog B-tree** (leaf + index levels), an empty extents B-tree, and data-fork block
> allocation. It supports **files** (incl. empty and multi-block), **folders** (nested), and
> **symlinks** (the `/Applications` drop target), plus per-entry POSIX mode and Finder flags
> (e.g. `kHasCustomIcon`). Arbitrary files such as `.DS_Store` and `.VolumeIcon.icns` are just
> `KindFile` entries.
>
> Oracle-verified by tests that mount the output and read it back: a mixed file/folder/symlink
> tree (contents + symlink target confirmed) and a 600-file volume (forces multiple leaf nodes
> + an index node; all files read back). Cross-compiles to linux/windows/arm64.
>
> **The full pure-Go DMG chain now works:** `hfsplus.Build` → `udif.Build` → a `.dmg` that
> `hdiutil verify` accepts and Finder mounts — built with zero macOS dependency.
>
> **Faithfulness gate passed + case mode resolved.** A real ad-hoc-signed **single-binary**
> `.app` packaged through the pure-Go chain passes `codesign --verify --strict` when mounted —
> proving every byte and the exec bit survive (a far stronger check than content round-trips).
> _(A framework-bearing bundle under `codesign --verify --deep` — exercising relative
> framework symlinks — is the first check in the integration phase; the symlink and byte-fidelity
> primitives it relies on are already independently verified.)_ The deep catalog B-tree
> (treeDepth ≥ 3, what a real multi-thousand-file `.app` needs) is exercised by a 3000-file
> mount+readback test. Volumes now
> default to **case-insensitive HFS+** (`H+`, the platform convention for distribution images);
> `Volume.CaseSensitive` opts into HFSX. The case-insensitive comparator is ASCII-correct
> (matches macOS fold exactly for ASCII, which covers app-bundle names) and **errors rather
> than corrupts** on non-ASCII names. Both modes are codesign-verified in CI tests.
>
> **The HFS+ writer is functionally complete.** Remaining work is integration, not the writer:
> wire it behind `dmg.Build()` as `nativeBackend` (walk source dir → `Volume`, generate
> `.DS_Store`/icons), and — tracked follow-ups — a full TN1150 Unicode fold table (for
> non-ASCII case-insensitive names) and resource-fork data (not needed by modern `.app`s).
> See §5 below.

> **Update 2026-06-08 — native backend wired into `dmg.Build()`.** `Options.Backend` selects
> the image-creation backend: `BackendAuto` (default) uses hdiutil on macOS and the pure-Go
> backend everywhere else; `BackendNative` forces pure-Go; `BackendHdiutil` forces macOS. The
> `dmg` package was split by build tags so the hdiutil/syscall path is darwin-only and the
> native path + shared helpers (`newStyledStore`) **cross-compile to linux/windows** — so the
> whole package now builds off-macOS, and `dmg.Build` produces a styled DMG (background,
> Applications link, icon layout, `.DS_Store`) entirely in Go. Verified by `TestBuildNative_EndToEnd`:
> a signed `.app` built through the native backend passes `codesign --verify` when mounted.
> The native backend produces UDZO/HFS+ only (other formats/APFS error and point to hdiutil).

## 1. Executive summary

The library is in good shape: it builds, all tests pass, and it does something genuinely
useful that nothing else in pure Go does well — **styled** DMGs (background image, icon
positions, window geometry, volume/file icons) via pure-Go `.DS_Store` and ICNS generation.

It is **not yet v1-quality for a public release**, for three reasons:

1. **The "pure Go" claim is misleading.** `.DS_Store` and ICNS generation _are_ pure Go, but
   the actual disk-image creation shells out to `hdiutil` (create/attach/detach/convert/resize)
   and `/usr/bin/ditto` (file copy), plus `syscall.Sync`. The library only produces a DMG on
   macOS. This needs to be stated honestly before it goes public.
2. **No CI, no `doc.go`, no examples, no CHANGELOG/CONTRIBUTING**, and the repo carries 6 `cmd/`
   dev tools plus Python scratch scripts that should not ship in a v1 public release.
3. **The public API is not locked.** `Build` vs `BuildWithContents` overlap with surprising
   merge semantics, and the `dsstore`/`icns` sub-packages export a large internal surface that
   was probably never meant to be public API.

### The one decision that defines v1 scope

> **Does v1 keep `hdiutil` (Option A) or write the UDIF/HFS+ image format in pure Go (Option B)?**

- **Option A — Pragmatic v1 (RECOMMENDED).** Keep `hdiutil`. Make the docs honest, lock the
  API, add CI, clean up the repo. Ships in ~1–2 weeks of focused work. This is correct for
  Wails because macOS app bundles are built and signed on macOS runners anyway, where
  `hdiutil` is always present.
- **Option B — True pure Go.** Write UDIF + HFS+ directly in Go, no `hdiutil`. This is a large,
  high-risk effort (months) and is **not** a v1 gate. It belongs as a post-v1 feasibility spike.

**Recommendation: ship v1 on Option A.** Everything below assumes A unless noted. If you want
B in v1, say so and the roadmap changes substantially. (Background job — recommending A and
proceeding rather than blocking; veto if you disagree.)

---

## 2. Audit findings

### ✅ Present and working
- `go build ./...`, `GOOS=windows`/`GOOS=linux go build ./...` all succeed (WIP build tags work).
- `go test ./...` passes: `dmg` 69.2%, `dsstore` 62.0%, `icns` 83.0% coverage.
- Real, non-trivial functionality: pure-Go `.DS_Store` (buddy allocator + B-tree + alias
  records), pure-Go ICNS encoding, `.icon` (Tahoe "Liquid Glass") package compositing,
  volume + file icons via `com.apple.FinderInfo` / `com.apple.ResourceFork` xattrs.
- `TestBuildIntegration` exercises the full `Build()` path end-to-end (darwin-gated).
- The WIP correctly abstracts platform-specific bits (`syncFilesystem`, `fileID`,
  `setCustomIconFlag`, `setResourceFork`) behind `platform_unix.go` / `platform_windows.go`.

### ⚠️ Broken / incorrect
- **`go vet` failure:** `dsstore/store.go:235` — `WriteTo(w io.Writer) error` collides with the
  `io.WriterTo` convention (`WriteTo(io.Writer) (int64, error)`). Rename to `Encode`/`WriteStore`
  or make it conform. This is the only vet failure and should be fixed before tagging v1.
- **README "pure Go" overclaim** (see §1). The headline and the `hdiutil`/`ditto`/`cp`
  dependency contradict each other.
- **WIP `getAliasFileInfo` regression (minor, macOS-correctness):** the diff replaced
  `syscall.Stat(...).Birthtimespec` (true HFS creation date) with `os.Stat(...).ModTime()`
  (modification time) for the alias record's creation dates. Functionally Finder still resolves
  the alias, but the dates are now less accurate than before. Worth restoring the birthtime on
  the unix path (it's available via `Stat_t.Birthtimespec`) while keeping the portable fallback.

### ❌ Missing (for v1 quality)
- **No CI.** No `.github/workflows`. The only test of the main `Build()` path is darwin-gated,
  so CI must include a **macOS runner** or the core path is never exercised in CI.
- **No `doc.go` / package-level godoc** for `dsstore` and `icns`; `dmg` has a short package
  comment only. No runnable `Example` functions, no `examples/` dir.
- **No CHANGELOG.md, CONTRIBUTING.md, or SECURITY.md.**
- **No honest cross-platform story.** On Windows/Linux the package _compiles_ but `Build()`
  fails at runtime (`hdiutil` absent; `setCustomIconFlag` returns an error). This must be
  documented and ideally surfaced as a clear, early, typed error (e.g. `ErrUnsupportedPlatform`)
  rather than a deep `hdiutil: executable file not found`.
- **No `context.Context` support.** `Build()` can run for many seconds (compression, retries);
  Wails should be able to cancel it. Add `BuildContext(ctx, opts)`.
- **No structured error types / progress callback.** Wails' UX benefits from progress
  ("creating image… compressing…") and from distinguishing "no hdiutil" from "copy failed".
- **Distribution gaps that Wails users will ask for** (scope per §4): code-signing the DMG
  (`codesign`), notarization/stapling hooks, and an SLA/EULA "license agreement" resource.
  None are present. These are likely **post-v1**, but the API should not paint itself into a
  corner that blocks adding them.

### 🧹 Repo hygiene (must address before public)
- `cmd/` contains 6 dev/debug tools (`dsanalyze`, `dscompare`, `mkdmg`, `refdmg`, `single_test`,
  `testdmgs`). Decide: ship `mkdmg` as the one real CLI, move the rest to `internal/` or delete.
- `test-dmgs/` has Python scratch scripts (`decode_dsstore.py`, `compare_aliases.py`, etc.) and
  golden artifacts. Keep golden fixtures used by tests; move/delete the ad-hoc Python.
- `go.mod` is `go 1.24.0`; sibling `dmg-builder` is `go 1.25`. Pick a supported floor.

---

## 3. The Wails v3 integration contract (what v1 must satisfy)

Wails v3 already has its own DMG creator at
`wails/v3/internal/commands/dmg/dmg.go`. It is a thin `hdiutil create -srcfolder` wrapper with a
notable bug: **it copies the background image into the volume but never generates a `.DS_Store`,
so the background and the stored icon positions are never actually displayed.** This library is
the styled replacement. Its current public API is:

```go
c, _ := dmg.New(sourcePath, outputPath, appName) // sourcePath is the .app bundle
c.SetBackgroundImage(path)
c.AddIconPosition(filename, x, y)
c.Create()
```

Triggered from `v3/internal/commands/tool_package.go` (DMG path = `<out>/<ExecutableName>.dmg`)
behind the `--create-dmg` flag in `v3/internal/flags/package.go`.

**v1 implication:** the library should offer an entry point that maps 1:1 onto this so the
Wails-side `Creator` becomes a thin adapter (or is deleted in favor of calling the library
directly). The existing `Options` + `BuildWithContents` already cover everything Wails needs
(app bundle, Applications symlink, background, per-icon positions) — the gap is **API stability
and a clean, documented entry point**, not missing features.

---

## 4. What "v1" is (scope)

**In scope for v1 (Option A):**
1. Honest docs: "pure-Go `.DS_Store` + ICNS generation; uses `hdiutil` for image creation
   (macOS only)."
2. A **locked, documented public API** — one obvious way to build a styled app DMG.
3. `BuildContext(ctx, Options)` + a typed `ErrUnsupportedPlatform` returned early off-macOS.
4. CI with a **macOS runner** (build + vet + test), plus linux/windows compile-only jobs.
5. `go vet` clean (fix `WriteTo`).
6. Repo cleanup: one CLI, no Python scratch, sub-package surface decided.
7. godoc + at least one runnable `Example`, README rewrite, CHANGELOG, CONTRIBUTING, LICENSE
   (LICENSE already present — MIT).
8. A Wails v3 adapter PR (or issue) demonstrating the drop-in replacement.

**Explicitly post-v1 (document as roadmap, don't build now):**
- Option B: pure-Go UDIF/HFS+ image writer (feasibility spike first).
- Code signing (`codesign`) + notarization/stapling helpers.
- SLA/EULA license-agreement resource.
- APFS-specific tuning, retina/`@2x` background handling, multi-window layouts.

---

## 5. Roadmap

### Phase 0 — Decide (blocking, you)
- [ ] Confirm **Option A** for v1 (or choose B and re-plan).
- [ ] Confirm `dsstore` and `icns` are **public** sub-packages or should move under `internal/`.

### Phase 1 — Correctness & honesty (0.5 wk)
- [ ] Fix `go vet`: rename/realign `dsstore.Store.WriteTo`.
- [ ] Restore HFS birthtime on the unix path in `getAliasFileInfo` (keep portable fallback).
- [ ] Rewrite README "pure Go" framing; add a "Platform support" section (macOS-only runtime).
- [ ] Add `ErrUnsupportedPlatform`; return it early from `Build` when `hdiutil` is unavailable.

### Phase 2 — API lock (0.5 wk)
- [ ] Decide `Build` vs `BuildWithContents`: keep one as primary, document the other as
      convenience, and fix `BuildWithContents`'s surprising "copy only non-zero fields" merge.
- [ ] Add `BuildContext(ctx, Options)`; thread `ctx` into `exec.CommandContext` calls.
- [ ] Optional `Progress func(stage string)` hook on `Options`.
- [ ] Audit exported identifiers in `dsstore`/`icns`; unexport anything not intended as API.
- [ ] Tag the surface in `doc.go` and freeze it.

### Phase 3 — Quality gates (0.5 wk)
- [ ] `.github/workflows/ci.yml`: `macos-latest` (build, `go vet`, `go test -race ./...`),
      plus `GOOS=windows`/`GOOS=linux go build` compile checks on `ubuntu-latest`.
- [ ] Raise coverage on the main `Build()` path; add a golden-file test for `.DS_Store` bytes.
- [ ] Add `golangci-lint` config and a CI lint job.

### Phase 4 — Docs & cleanup (0.5 wk)
- [ ] `doc.go` for each package + ≥1 runnable `Example`.
- [ ] Move 5 of 6 `cmd/` tools to `internal/` or delete; keep one documented CLI.
- [ ] Remove Python scratch from `test-dmgs/`; keep only fixtures referenced by tests.
- [ ] CHANGELOG.md, CONTRIBUTING.md, SECURITY.md.

### Phase 5 — Wails integration (0.5 wk)
- [ ] Adapter in `wails/v3/internal/commands/dmg` that calls this library, replacing the
      hand-rolled `hdiutil` path (fixing the missing-`.DS_Store` bug).
- [ ] Manual verify: build a styled DMG, mount, confirm background + icon positions render.
- [ ] Tag `v1.0.0`, make repo public.

### Post-v1 backlog
- Pure-Go UDIF/HFS+ writer spike (Option B).
- `codesign` + notarization/stapling helpers.
- SLA/EULA resource; retina backgrounds; APFS tuning.

**Estimated effort to v1 (Option A): ~2.5 focused weeks**, front-loaded on API decisions
(Phase 0–2) since everything downstream depends on the locked surface.
