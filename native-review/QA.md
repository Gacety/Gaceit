# Pure native UI migration — 2026-11-09

- Desktop entry creates only `mygo.WindowOptions{Content: ui.View(a.View)}`. No URL, WebView, Bind bridge, HTTP listener, React bundle or frontend embed.
- Rebuilt the app with Go alone and verified local code signature.
- Inspected mygo-music's `ui_main.go`/`ui_kit.go` as structural references; no AGPL application code copied.
- Inspected native light, Dracula, formula, source table and right-note layouts using MyGo CPU rendering. Observed the actual running native window via accessibility and screenshot: it exposes native controls without HTML content or a web-area URL.
- 1,576 source questions, 249 imported wrong-book flags and 30 errata retained.
- 1,675 unique formula SVGs generated; all parse through MyGo. Converted 262 text objects to supplied-font paths; no missing glyphs. Original formula strings remain the manifest keys and copy/accessibility source.
- Go review model agrees with 30 browser-model fixtures across incorrect, unknown, uncertain, cold-start, early, short-repeat, relearning and spaced-success cases.
- Native headless tests exercise answer click, grading, undo preserving newer notes, note input, stroke input, exclusive tools, persistence/reload, stable random rounds, incomplete multi-blank rejection, disputed keys, formula/image/table assets and last-due feedback.
- Complete `go test ./...` passed, including the separately developed ink engine and demo. Old frontend tests remain historical, not proof of native behavior.
- QA learning files are isolated under `.local-data/native-qa` or per-test temporary directories. The live user's learning controls were not operated during screenshot inspection.

Limits: vector generation and automated checks do not constitute visual review of every question/formula. The scratchpad remains the authorized lightweight demo, separate from the advanced ink sandbox. Image-only source material is shown as its original image.

Final regression: 48 Go tests passed across all packages. Static source-resource audit: 73 tables, 243 image references (zero missing), 1,675 formulas (zero missing assets), zero SVG text nodes. Signed app verification passed.

## Literal HTML review layout — 2026-11-09

- Visual authority: `/Users/gacety/Downloads/刷題 App UI 設計.html`, wrong-question review board. Compared sanitized reference and native CPU renders at 1440×900; no HTML runs inside the shipped app.
- Reproduced sidebar, header rule, card borders/radii, option letter blocks, selected states, typography, toolbar buttons, footer and right-hand scope/answer cards. Removed added tool icons and footer divider absent from the reference.
- Geometry regression verifies the question card (280,88,788,536.25), rail (1100,88,300,788), option rows, title tools, directory, shuffle and submit button against measured reference coordinates.
- Scope counter row fixed to the reference's 46px height to avoid native font metrics moving the answer card by a fraction of a pixel.
- Review uses embedded Noto Sans SC variable weights with included OFL license; the originally supplied Bold font remains embedded and used by other pages and formula outlines.
- Real question text, live counts and progress remain authoritative; reference mock counts and rewritten punctuation were not copied into learning data.
- Native interaction checks cover scope changes, stable answer-card numbering and persistence, explicit reshuffle, mutually exclusive writing panes, keyboard grading and recall actions in the context menu.
- Final full regression: 66 Go tests passed, no failures. Optional capture test generated `review-html-dark.png` and `review-html-light.png`; both visually inspected. QA used temporary learning directories. Go-only desktop build and ad-hoc signature verification passed.
- Limits: native and browser rasterization can differ at glyph edges; screenshot inspection covers the representative review question, not every bank item or macOS display scale.

## Three additional HTML boards — 2026-11-09

- Implemented chapter practice, random setup and notes list/detail from the updated seven-board HTML. Shared HTML palette/sidebar/font remain native; original assets and learning data are retained.
- Bounded verification at the user's request: one existing storage/migration check and one optional visual-capture/primary-action smoke check, then Go-only desktop packaging. No full-suite run for this change.
- Captured chapter, random and notes at 1440×900 in dark, plus notes in light. Inspected once, corrected native flex stretching in one batch, then confirmed the three dark captures. Screenshot notes are fixtures held only in a temporary test store; user learning data was not opened or mutated.
- The random primary action starts a real 20-question round. Multi-subject selection uses the same stable source IDs and draws without replacement. New settings preserve legacy single-subject reading and explicitly empty selections.
- Simulated-exam tile is present but access-disabled and explains its unavailable status. Actual deferred grading/exam behavior was not added as part of this UI request.
- Native font metrics and real counts differ from mock statistics. Further chapters/long notes scroll within the desktop workspace; no source question or saved note is shortened on disk.

## Unified chapter and random readers — 2026-11-09

- Routed both question readers through the exact review shell, retaining distinct queues, breadcrumbs, metadata and return actions. Shared title tools, bordered option cards, footer, right progress card and answer-card geometry now match review.
- Necessary targeted checks only: review reference geometry and native answer/exclusive-writing-tool interaction passed. One optional native capture/smoke check verified both modes' card navigation changes the selected question without recording attempts or changing random IDs.
- Inspected chapter dark, random dark and random light captures at 1440×900. QA used temporary storage. No full regression suite or user learning-file interaction was performed for this visual change.
- Captures: `practice-reader-dark.png`, `random-reader-dark.png`, `random-reader-light.png`. Right rail labels sequence scope and explains that cell colors represent today's local attempts; progress is positional.

## Native ink integration — 2026-11-09

- Reused `internal/ink` directly through the `internal/inkcanvas` native controller adapted from the local demo. Original `cmd/ink-demo` source files and independent entry point were not changed. No HTML/browser/JS runtime was introduced.
- Necessary targeted checks passed: new stroke persistence, legacy-field retention, existing neutral ink adapting white↔black, real lasso-to-vector-note capture, note-only-clip gallery inclusion, clearing tags/privacy across reload, undo/redo, pan/zoom preserving strokes, unsupported document rejection without writes, and creation of the one-time pre-ink backup. Updated existing native answer/tool-switch and writing persistence checks passed for the full-canvas interaction.
- Captured and inspected native CPU renders in a bounded pass: 1440×900 dark/light canvas, progress popup and note card; 1060×720 light note-card layout. One batch corrected toolbar width, light-theme shadow opacity and overlay details; confirmed final dark note and light canvas renders. Screenshot note/stroke examples exist only in temporary QA stores.
- Progress uses current queue IDs and today's local attempts. The time estimate is labelled as a per-question estimate; no mock completion/score values were persisted.
- Existing progress fields and unknown record/document metadata remain retained. The first advanced ink save makes a private, non-overwriting `progress-before-native-ink-v1.json` copy when an existing learning file is present. Invalid version or non-finite geometry is rejected before restoring state. Closing or navigating is blocked if the pre-save backup could not be made.
- Verification remains limited to targeted CPU/native UI event checks and Go desktop build/signature, not a new full-suite/benchmark run or physical tablet/Metal latency certification. Original ink-engine evidence remains documented separately.

## Phosphor icons and fixed question — 2026-11-09

- Replaced the hand-authored canvas SVGs with pinned, hash-checked Phosphor Regular assets from the existing independent demo. Preserved MIT license/source metadata; redo is a documented horizontal mirror of the library undo arrow.
- One targeted native-event regression passed: with the original explanation making the question overflow, touchpad scroll, pinch and hand dragging originating over the question changed only the ink camera. Both question-frame and stem bounds and the internal question scroll offset remained unchanged; no stroke or learning attempt was created.
- One 1440×900 native CPU capture (`canvas-phosphor-icons.png`) inspected to confirm the new toolbar. No full test suite or benchmark run for this narrow change.

## Clarified world-space question — 2026-11-09

- User clarified that “fixed” means anchored in canvas WORLD coordinates, not fixed to the camera viewport. The preceding screen-fixed check documents the earlier interpretation and is superseded by this implementation.
- The question now shares exactly the ink camera's offset and zoom, with a fixed 720px layout at world (360,110). Native transform scales the complete question subtree rather than changing font size/reflow. Opening the note overlay does not alter its world anchor. Home restores 100% and the starting world region, centered horizontally in the current window.
- One targeted interaction test passed: pan updates the card's screen position by the camera delta; 80% pinch scales card and stem bounds; answer selection remains clickable while scaled; scaled hand-drag preserves exact screen pointer delta; Home restores the original frame/stem bounds; note overlay leaves the anchor unchanged; navigation creates neither ink strokes nor attempts.
- Native capture at 75% (`canvas-world-75.png`) supplements the 100% capture. Existing ink/learning data formats were not changed. No full-suite run for this correction.

## Complete long-question cards — 2026-11-09

- Removed the canvas question's 660px height cap, nested Scroll, scrollbar state and Option-wheel exception. The single world-space card now lays out full source content, options and post-answer explanation at natural height.
- Targeted checks passed for the world-camera interaction and a real long source question (`relax-os-03-037`, including three tables). At 25% zoom its complete card, final option, original explanation and bottom answer actions are visible within the card; natural height exceeds the former 660px limit. The initial 50% QA view was too tall for the window, so the whole-card inspection used a smaller camera zoom rather than altering content.
- `canvas-long-full.png` captures this isolated QA state. Go desktop package/signature build completed. No learning-file format or source text changes, no full-suite run.

## Infinite canvas responsiveness and texture recovery — 2026-11-09

- Minimap: previously it cloned every stroke and redrew all of them inside its 144×84 thumbnail on every frame. A 1,200-stroke CPU fixture measured ~76ms per frame for the rebuild versus ~10ms with the cached overview, which explains the non-following pen. It now caches a `ui.Canvas` keyed by `Editor.Revision`, rebuilds only when no stroke is in progress, at least 900ms after the last ink change, and at most twice a second. The viewport outline still updates every frame.
- Save path: `flushInk` now updates memory and arms a debounced snapshot write (600ms after the last edit, forced after 3s of continuous input). Navigation, canvas close, backup/import and window close force the write. This removes the synchronous 1.1MB record rewrite from every pen-up. New tests verify deferred, flushed and pending-cleared behavior; existing reload tests explicitly flush.
- Texture loss: no short keepalive frame is scheduled, because presenting a frame after MyGo released textures showed a blank page. A frame gap over 1.8s rebuilds dry and pan layers from all vector strokes before drawing; the minimap is invalidated with it. `TestSurfaceResetRebuildsCompleteInk` compares CPU pixels before and after the reset.
- Zoom: each zoom frame stretches the last baked viewport; after 180ms without a zoom event one settle frame rebuilds vectors at the new scale. A 1,200-stroke CPU fixture dropped from about 1.3s per zoom frame to about 12ms, with a one-time ~1.0s settle. The eraser tool now uses `CursorNone` while its ring is drawn, so no crosshair sits inside the ring.
- Theme adaptation no longer walks every stroke on every frame; it runs once when the paper/theme actually changes. Bounds are cached by edit revision. Prepared Markdown is cached per source string, avoiding table HTML parsing each frame.
- Full `go test ./...`, `go vet ./...` and a Go-only build passed. Dense-canvas timings are CPU-renderer measurements; Metal latency was not hardware-certified.

## Canvas question crispness — 2026-11-09

- Replaced the world card's `TransformOrigin/Scale` with layout at the camera scale: 720×zoom width and scaled padding, radii, borders, shadows, meta badges, question/option typography and letter blocks. At 200% the native glyphs render at 42/34 DIPs instead of magnifying a 21/17-DIP glyph texture; the previous 200% capture was visibly soft, the new one is crisp.
- 100% renders are pixel-identical to the prior screen-fixed-size layout. Local card input now adds the screen origin without multiplying by zoom; pan, pinch, option clicks and Home still pass the world-camera regression.
- Source images use one stable cached bitmap and are laid out at the card scale; no per-zoom URL or asynchronous reload remains, so pinch does not flicker. No fake vector conversion was introduced, and their pixels still limit sharpness. Formula SVGs and inline formula objects scale through the scaled type sizes.
- `native-review/canvas-zoom-200.png` captures an isolated 200% QA state. The text/option geometry is native DIPs at the zoom factor, not an upscaled 100% bitmap.
- Inline source images are now promoted to their own Markdown block: MyGo did not call `LoadImage` for an image inside a text paragraph, so those questions showed `[image]` placeholders even at 100%. `native-review/canvas-zoom-image-200.png` shows the real raster diagram at 200% after block promotion; the image keeps one stable source while its layout width follows zoom, so pinch does not reload it.

## Long-press option exclusion — 2026-11-09

- A 450ms press toggles exclusion; movement over 8 DIPs or a scroll cancels. Excluded rows fade the border, background, letter block and content, with native strike lines per text line. A second press restores; choosing an excluded option restores it.
- State is stored per question as `blank:letter` keys in `Record.Excluded`, participates in the debounced save, and survives reload. The optional-field cleanup list now removes an emptied `excluded` field from raw records.
- Tests cover exclusion, restore, reload persistence, immediate choice stability after long press, excluding the selected option, excluding a different option, and choosing an excluded option. Captures: `native-review/option-exclude-dark.png`, `option-exclude-light.png`, `option-exclude-multiline.png`. Plain options use native text strikethrough; options containing inline math fall back to per-line painted strikes.

## Tablet interruption and recovery — 2026-11-10

- Current user identified a pen tablet. QA used isolated learning files and MyGo's PointerPen input, not the user's real history or tablet driver.
- Added a continuous 32-step circular gesture crossing behind/around the question, with a simulated real paint-gap recovery midway. Canvas focus and all sampled points remained intact, and incremental ink matched a complete vector rebuild. The minimap is excluded from pixel identity because its intentional wall-clock refresh may occur between slow instrumentation frames.
- System cancellation preserves the received curved fragment without appending an artificial cancel-location point; a later press starts a separate stroke. Existing standalone-demo cancellation remains unchanged.
- Moved full snapshot encoding/fsync to a detached worker while preserving save debounce/max-delay and forced-save ordering. A worker result cannot replace newer live edits; explicit save/close waits before writing the newest state.
- Full `internal/nativeapp` and `internal/inkcanvas` regression passed after implementation. Targeted race checks passed for actual frame-gap recovery and save-worker/deferred-save ordering. Race instrumentation made the full circular-render check slow and exposed a timer-driven minimap difference in its old whole-window comparison; the comparison now targets the ink region. No physical tablet-driver/Metal input latency certification is claimed.
- No source questions, HLR rules, progress keys, or original user data were changed. Final desktop build is native Go/MyGo only.


## Canvas submit responsiveness — 2026-11-11

- Submitting from the infinite canvas previously changed `answered`, which made `syncCanvasCardSurface` call `ResetSurfaceCaches` before the next draw. That discarded dry/pan/tile caches and rebuilt every stroke in one frame: a 1,200-stroke CPU fixture measured ~1.22s for the first answered frame, followed by ~9ms.
- The answer/details/card-height paths now call `Editor.RequestRecompose`, which keeps the last complete viewport and advances the complete rebake at most eight visible strokes per paint. The first answered frame measured ~48ms on the same CPU fixture; subsequent rebake frames carry the renderer cost rather than blocking the judgment frame. A regression test renders to completion and compares the progressive result byte-for-byte with a full vector rebuild.
- Submit still performs the intentional synchronous grading save (~7ms in the isolated fixture) and explanation layout; those are no longer followed by a one-second full-surface stall.


## Note-clip preview editor isolation — 2026-11-11

- Confirmed on a fresh App: opening the notes dashboard with a saved `InkClip`, then entering scratch, opened at `HomeZoom=1` / `Zoom=1` instead of the app policy's 80% and bypassed 50%–200% limits. `inkClipView` created `newInkPreview()` and assigned it to `App.inkEditor`; `ensureInk` then skipped `ConfigureZoom`.
- Saved-clip previews now use a dedicated `App.inkPreview` (released on canvas close). Regression test `TestNoteClipPreviewDoesNotTakeOverCanvasEditor` asserts the live `inkEditor` remains nil while the notes preview renders, then opens scratch at the configured 80% zoom. Full suite and signed build pass.


## Regional tile repair for lasso movement — 2026-11-11

- Lasso movement previously unioned all selected old/new bounds and rerendered the full affected dry area; with 1,200 strokes and 80 selected it measured ~231ms per moved frame in the CPU tester. Tiles existed for pan, but every changed tile was rebuilt as a whole.
- `tileCache.syncForRepair` now updates source geometry/index without discarding resident tiles. `tileCache.repair` clears only each tile's dirty sub-rectangles, repaints paper and the strokes intersecting those rectangles in paint order, and keeps every other tile pixel. Dirty rectangles are coalesced per tile to avoid hundreds of tiny clip/query calls.
- Same fixture: first move ~234ms (cold tile population), subsequent moves ~27–35ms; a compact 80-stroke selection measured ~27–28ms. This is CPU-renderer data, not Metal latency.
- `TestSelectionMoveRepairMatchesFullRebuild` compares the patched result byte-for-byte with a full vector render after a translated selection move. Full Go suite and vet pass.


## Lasso drag as a base + selected layer — 2026-11-11

- The regional repair path still recomposited thousands of strokes per drag frame. A 1,200-stroke CPU fixture with 80 selected measured ~234ms for the cold first move and ~27–35ms afterward.
- A drag now builds, once, a complete viewport base with the selected strokes removed and a clipped transparent layer containing those strokes at zero delta. Each moved frame draws only those two images; drop commits the translated vectors and composes base + selection into the new dry canvas.
- Same fixture: prepare ~58ms, drag frames ~1.2–1.5ms, commit ~22ms. The layer is viewport-clipped and released on drop; no unbounded raster. Highlighter selections draw directly instead of precompositing, matching a full vector render.
- `TestSelectionLayerPreviewAndCommitMatchFullRender` compares the live fractional-delta preview and committed result against an independent full vector render, including mixed pen/highlighter strokes. Full suite and vet pass.

## Canvas logic hardening — 2026-11-11

- Zoom-out preview previously filled the whole viewport with background/paper before drawing the retained viewport, hiding ink that the new, larger view exposed. It now draws the retained viewport first and fills only the four outside bands with paper plus the visible vectors in those bands. `TestZoomOutPreviewShowsNewlyExposedStroke` verifies a newly exposed stroke is present.
- Card-height recomposition requests now carry a generation and are accepted even while a zoom preview or navigation bake is active. The active bake checks the generation before clearing the request; `TestRecomposeRequestSurvivesActiveBake` sends a second request mid-bake and verifies it runs.
- Selection preview is now z-order segmented: a static prefix, selected/unselected contiguous runs (small transparent layers, or direct vectors for highlighter runs), and the suffix. Paint order matches a full vector render. Over-budget or over-512-run selections use exact direct-range fallback. Tests cover interleaved order, highlighter/pen mixing, direct fallback and final commit.
- CPU fixtures: contiguous 80-stroke suffix prepare ~6ms, drag ~0.03–0.04ms; 160 alternating runs prepare ~234ms, drag ~6ms. Zoom-out preview with 1,200 strokes is ~153ms; real Metal still needs experience confirmation.

## Dense zoom and large-selection follow-up — 2026-11-11

- Dense zoom previews no longer vector-fill every newly exposed band. Up to 256 strokes retain the exact outside-band fill; larger documents paint paper only in those bands and rely on the settle bake. 2,000/5,000-stroke CPU zoom-out previews dropped from ~1.2s/~9.3s to ~1.2ms; settle frames are ~6ms.
- All-strokes lasso selection now reuses the current complete dry image as the moving layer with a paper base: 5,000 strokes measured ~14ms prepare and ~10ms drag, instead of ~7.6s prepare.
- Z-order run layers remain for non-all selections; contiguous suffix 80 strokes ~6ms prepare/~0.03ms drag; 160 alternating runs ~234ms prepare/~6ms drag. More than 512 runs still uses exact direct-range fallback and is inherently proportional to the selected range.

## Dense zoom and all-selection measurements — 2026-11-11

- With >256 strokes, zoom-out previews no longer vector-fill outside bands: 2,000/5,000-stroke CPU preview frames measured ~1.2ms (previously ~1.2s/9.3s). Settle frames measure ~6ms; new bands show paper until the bounded bake fills them.
- All-strokes lasso selection uses the existing complete dry image as the moving layer with a paper base: 5,000 strokes measured ~14ms prepare and ~10ms/drag frame. This fast path intentionally moves the paper/dot layer with the ink, a known visual tradeoff.
- Non-all selection remains z-order-segmented. A contiguous 80-stroke suffix is ~6ms prepare and ~0.03ms/drag; 160 alternating runs are ~234ms prepare and ~6ms/drag. More than 512 runs uses exact direct-range fallback and remains proportional to the selected range.
