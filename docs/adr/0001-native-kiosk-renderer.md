# ADR-0001: Native kiosk renderer

Status: Proposed (long term)
Date: 2026-10-12

## Context

The desk monitor has no desktop. Today the plan is a minimal Wayland kiosk
(cage + foot) running nocterm in a fullscreen terminal for truecolor, or the
bare VT at 16 colors. Unheaded already runs Doom with no desktop, so drawing
pixels directly is proven on this hardware.

## Idea

A second output backend that draws straight to the display (DRM/KMS dumb
buffer or fbdev) instead of a terminal. It would turn nocterm into a real
kiosk: smooth graphs, real fonts at any size, no terminal emulator, no
compositor. Further out, the same wrapper could host other binary GUI
applications fullscreen on a headless box.

## Shape

- `ui.Render` already returns a canvas, not terminal output. A pixel backend
  rasterizes the canvas (or richer primitives behind the same panel API).
- Panels stay backend-agnostic; only `internal/tui` knows about terminals.
- The SSH path keeps the terminal backend. The same binary picks one by flag.

## Open questions

- DRM/KMS ownership vs a running getty or compositor; VT switching.
- Font rasterization: embed a bitmap font, or x/image/opentype.
- Input: none needed (view only), which keeps it simple.
- Whether to stop at nocterm or grow into a generic kiosk host.

## Decision

None yet. Keep the canvas/output boundary clean so this stays possible.
