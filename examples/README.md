# Examples

Runnable example programs for the `dmg` library. Build them all with
`go build ./examples/...`, or run one directly with `go run`.

| Example | What it shows | Platform |
|---------|---------------|----------|
| [`purego-dmg`](./purego-dmg) | Build a mountable `.dmg` from a directory using **only Go** (`hfsplus` → `udif`) — no `hdiutil`, so it cross-compiles. | any |
| [`styled-dmg`](./styled-dmg) | Build a styled app `.dmg` with a gradient background and Applications drag-link via the high-level `dmg` package. | macOS |
| [`png-to-icns`](./png-to-icns) | Convert a PNG/JPEG image to an Apple `.icns` file with the `icns` package. | any |

```sh
# Pure-Go DMG from a directory (works on Linux/Windows/macOS):
go run ./examples/purego-dmg ./MyApp.app MyApp.dmg MyApp

# Styled DMG (macOS only):
go run ./examples/styled-dmg ./MyApp.app MyApp.dmg

# PNG -> ICNS:
go run ./examples/png-to-icns icon.png AppIcon.icns
```

For API-level snippets, see the runnable `Example` functions in each package's
test files (visible on pkg.go.dev), e.g. `hfsplus.ExampleBuild`,
`udif.ExampleBuild`, `icns.ExampleEncodeFile`, and `dmg.ExampleBuild`.
