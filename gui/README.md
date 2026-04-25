# Forge GUI (Stage 11)

Electron Forge-based GUI for the Forge CLI. The CLI is the source of truth:
every read in this app is just `forge ... --json` invoked from the main process
and piped back to the renderer over `contextBridge`.

## Architecture

```
gui/
  forge.config.js Electron Forge packaging config
  src/
    main/
      main.js      Electron main process; registers IPC handlers
      forge.js     child_process.execFile wrapper around the forge binary
    preload/
      preload.js   contextBridge surface (window.forgeAPI)
    renderer/
      index.html   Static layout (no bundler)
      app.js       Vanilla JS controller; calls forgeAPI for every fetch
      style.css    Dark theme
  resources/
    bin/           OS-specific Forge core binary staged by Make
```

## Forge binary resolution

`forge.js` looks up the binary in this order:

1. `FORGE_BIN` environment variable
2. packaged Electron resource: `resources/bin/forge`
3. `gui/resources/bin/forge` staged by `make gui-stage-core`
4. repo `./bin/forge` produced by `make build`
5. PATH fallback

## Running

From the repo root:

```bash
make build         # builds ./bin/forge
make gui-install   # one-time: npm install for Electron Forge
make gui-start     # launches the Electron app
# or both at once:
make gui
```

## Packaging

The packaged app embeds the OS-specific Forge core binary as an Electron
resource. Use `OS` and `ARCH` variables instead of Make goal suffixes.

```bash
make core OS=linux ARCH=amd64          # dist/core/forge-...
make gui-stage-core OS=linux ARCH=amd64
make gui-dist OS=linux ARCH=amd64

# complete flow: core -> resources/bin -> packaged Electron app
make app OS=linux ARCH=amd64
```

Supported `OS` values: `linux`, `darwin` (`mac`, `macos` aliases),
`windows` (`win` alias). Supported `ARCH` values: `amd64`, `arm64`.

The Electron artifacts are written to `dist/gui/`.

## Features

- **Workbench picker**: `Switch Workbench…` in the topbar runs
  `forge work init` on the chosen directory (no-op if already initialized)
  and switches the view.
- **Project list** (sidebar) — backed by `forge list --json` against the
  current Workbench. Status badge per project; stale-path projects are
  highlighted.
- **Status tab** — `forge status --json` from the project root.
- **Check tab** — runs `forge check --no-update --json` on demand. Lifecycle
  is *not* mutated from the GUI (intentional; see Stage 11 design notes).
- **History tab** — tail of `.forge/history.jsonl` (last 50 events).
- **Toolchain tab** — `forge tool show --json` with per-entry availability.

## Design notes

- The renderer never touches the filesystem or spawns subprocesses; all access
  goes through IPC handlers in `main.js`.
- No build step / bundler. Vanilla HTML, JS, CSS — the project deliberately
  keeps the GUI thin so it can later move to a richer stack (or be replaced
  by a daemon/REST front-end) without rewriting Forge logic.
- This stage covers the Stage 11 completion criteria: pick a workbench, list
  projects, view per-project status / check / history / toolchain, and
  surface command output.
