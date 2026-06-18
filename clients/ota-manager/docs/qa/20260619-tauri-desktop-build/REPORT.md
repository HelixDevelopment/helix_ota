# Tauri v2 Desktop Build Report — macOS (Apple Silicon)

## Metadata

| Field | Value |
|---|---|
| Date | 2026-06-19 |
| Platform | macOS (Apple Silicon / arm64) |
| Rust version | rustc 1.95.0 (59807616e 2026-04-14) |
| Cargo version | cargo 1.95.0 (f2d3ce0bd 2026-03-21) |
| Tauri CLI version | tauri-cli 2.11.2 |
| Frontend framework | Vite 6 + React 19 + TanStack Router |

## Build result

**PASS**

## Binary location and sizes

| Artifact | Path | Size |
|---|---|---|
| Binary | `src-tauri/target/release/ota-manager` | 11 MiB |
| .app bundle | `src-tauri/target/release/bundle/macos/Helix OTA Manager.app` | — |
| .dmg installer | `src-tauri/target/release/bundle/dmg/Helix OTA Manager_0.1.0_aarch64.dmg` | 3.9 MiB |

Binary type: Mach-O 64-bit executable arm64.

## Issues encountered and resolutions

### 1. Missing frontend PostCSS dependency (`@tailwindcss/postcss`)

**Error:** Vite build failed with `Cannot find module '@tailwindcss/postcss'`.

**Root cause:** `postcss.config.js` references `@tailwindcss/postcss` plugin which was
not listed in `package.json` dependencies.

**Resolution:** Installed `@tailwindcss/postcss`:
```
pnpm add -D @tailwindcss/postcss@latest
```

### 2. Wrong module import path for `useLogin`

**Error:** `Cannot find module '@/hooks/useLogin'`.

**Root cause:** `src/features/auth/login-page.tsx` imported `useLogin` from
`@/hooks/useLogin` (file does not exist), but the actual export lives in
`@/hooks/use-auth.ts`.

**Resolution:** Changed import to `@/hooks/use-auth`.

### 3. `NavLink` unexported by `@tanstack/react-router`

**Error:** `"NavLink" is not exported by "@tanstack/react-router"`.

**Root cause:** `@tanstack/react-router` v1.170.16 uses `Link` instead of `NavLink`.

**Resolution:** Replaced `NavLink` with `Link` in `src/features/layout/sidebar.tsx`.

### 4. Default vs named export mismatch for `LoginPage`

**Error:** Route tree imported `LoginPage` as a named export, but `login-page.tsx`
uses `export default function LoginPage`.

**Resolution:** Changed import in `src/route-tree.gen.ts` from named to default import.

### 5. Missing app icons

**Error:** Tauri codegen panicked: `failed to open icon icons/32x32.png: No such file or
directory`.

**Root cause:** `src-tauri/icons/` directory did not exist; no icons were scaffolded.

**Resolution:** Generated a 1024x1024 gradient source icon via ImageMagick, then used
`pnpm tauri icon` to produce all required formats (PNG 32x32, 128x128, 128x128@2x,
icon.icns, icon.ico, etc.).

## Summary

The Tauri v2 desktop binary for macOS ARM64 was built successfully. All frontend
build issues (missing dependency, wrong imports, API naming differences) and
build-time issues (missing icons) were resolved iteratively. The resulting binary
is 11 MiB and the distributable DMG is 3.9 MiB.
