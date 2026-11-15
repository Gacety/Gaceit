# Desktop paper rebuild — 2026-11-09

- Built and launched the actual Go + mygo v0.8.3 macOS app, not just a browser mock.
- Ad-hoc signature verified. Output: `../build/408研习.app` (~45 MB). No notarization.
- Native 1440×960 light and dark themes inspected; screenshots in `.impeccable/review/paper/`.
- Existing 1,576 question import, source preservation, scheduling, undo and random-round tests pass. Added desktop storage sequencing and >64 KB request coverage: 36 frontend tests pass.
- Go store tests cover missing files, restart/load, private permissions, rejected writes preserving existing notes and corrupt file detection.
- Go-backed browser verification: answered the first question, entered a note, observed durable save, reloaded and recovered note/theme. Disk file was 137,695 bytes with the recorded attempt. Test records are isolated in `.local-data/preview`; native verification uses `.local-data/desktop`.
- Visual detector reports one advisory for the scratchpad grid. Retained because it belongs to the actual writing canvas.
- Existing development server remains at 127.0.0.1:4173. Browser uses existing localStorage; native app uses Go file storage. Backup export/import handles migration explicitly.
- Full `go test ./...` encountered five failures in concurrently added `internal/ink` tests (threshold/containment, circle AA, quadratic radius, cubic distance, highlighter). The desktop app does not import that package. No changes made to the separate ink implementation.

## Dracula update
Replaced dark tokens with Dracula background/foreground/selection/purple/red, with lifted secondary text for legibility. Light tokens and layout unchanged. Browser screenshot: `.impeccable/review/paper/desktop-dracula.png`. Rebuilt and verified the signed app.

## UI cohesion update
Removed option cards and outer workbook borders. Choices are continuous A–D rows with letter markers and state tinting. Consolidated writing tools, previous/next and submit into one action bar with matching keyboard order. Reviewed both themes, keyboard arrow selection, long formula content, review/random screens and notes empty state. Preserved collapsed independent writing panels and exact supplied font. Screenshots: paper/unified-light.png and paper/unified-dracula.png. 36 frontend tests pass; desktop build and signature verification pass. Detector retains only the actual scratch-canvas grid advisory.

## Centered question / right tools
Default question column measured 840px wide, x=214.5 at a 1280px viewport. With tools it moved to x=72 and the tool panels started at x=816.6, to the right of the question. Verified scratch only, both stacked, notes only, then closing both restores x=214.5 and 840px. Reviewed light notes and dark scratch layouts. 36 frontend tests passed. Screenshots: centered-reading.png, right-writing-panels.png, right-notes-light.png.

## Mutually exclusive full-height writing pane
Replaced two booleans with one active-tool state. Verified scratch → notes → scratch replacement and closing restores the centered question. Trigger buttons are in the right of the question heading, beside the area in the supplied screenshot. The active note panel measured 588px, exactly matching the right sidebar; canvas and textarea fill remaining height. Frontend tests: 36 passed. Screenshot: paper/exclusive-right-notes.png.
