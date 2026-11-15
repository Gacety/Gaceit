# 408 研习 — 原生桌面

Mode: Operate, with Read-oriented question typography.

The current design is implemented by `internal/nativeapp`. The legacy CSS in `prototype/` is historical, not current visual authority.

## Structure
A 206px quiet sidebar groups learning modes and subjects, following the structural example of mygo-music and Notion. A small top bar carries page location and persistence status. The main scrollable question column is centered at a maximum width of 820px. Answers are continuous native radio rows; selection, correct and incorrect states share their typography and spacing. A restrained footer holds previous/next and submit.

The 380px right writing pane (330px on narrower desktop windows) fills the workspace below the top bar. A single active-tool state selects notes or scratch; closing restores centered reading. Source content, errata and grading explanations remain separate.

## Type and colors
Use the exact user-supplied NotoSansSC-Bold.otf, registered as Study Noto. This is a single Bold face, so hierarchy relies on size, space and color, not invented weights. Code uses the native monospace role. Mathematical formulas use vector outlines and the current foreground color.

Light: warm white #fbfaf7, sidebar #f0efea, ink #30352f, muted #6d7369, accent #4f6653. Dracula: paper #282a36, sidebar #21222c, foreground #f8f8f2, muted #a5adce, accent #bd93f9. Functional writing grid only. No decorative glass or background image.

## Reference and implementation boundary
https://github.com/egoist/mygo-music was inspected for the sidebar/content structure and restrained native navigation. No application code from that AGPL project was copied. Native window construction uses `WindowOptions.Content: ui.View(...)`; there is no URL, WebView, HTML loader, React build or local server in the application entry point.

## Wrong-question review — supplied HTML is authoritative

The latest user correction requires literal reproduction of the `错题复习` board in `/Users/gacety/Downloads/刷題 App UI 設計.html`, including buttons and rules, not a stylistic reinterpretation. The implementation is `review_exact.go` and is still entirely MyGo native UI. The 1440×900 content area has a 240px sidebar, 56px header with 1px bottom rule, 32/40/24px body insets, a 32px column gap, 788px center column and 300px right rail. Question cards use 20px radius and 28/32px padding. Options are 64px tall, 14px radius, 1px rendered borders, 32px letter blocks, and 12px gaps. Footer buttons are 48px tall with 12px radius. There is no footer divider or extra inline recall toolbar.

Use the HTML dark palette for this page: #13141A / #181A21 / #1D1F28, #2B2E3A rules, #ECEDF3 text, #A5A9B8 muted, #8D92A3 faint, #A78BFA accent. To reproduce its typography, the review page uses locally bundled Noto Sans SC variable weights 400/500/700 as `Review Noto` (SIL OFL); the previously supplied static Bold face remains embedded for other native surfaces. This explicit request supersedes the older paper/Dracula style for the review screen. A light variant is retained.

Scope switches, current/total position, progress bar, answer-card grid and reshuffle are in the right rail. Answer numbers are real stable round positions, not the mock's 199 examples; completed items keep their number after scheduling, and the deck/cursor survive reload. The visible answer-card grid is five rows by six columns at the reference size, with native scrolling for further questions. Selecting a numbered cell navigates without recording an attempt. Writing tools temporarily replace the rail and return to it when closed. Recall quality and unknown actions remain available in the native question context menu without adding controls absent from the design.

## Chapter, random and notes boards — supplied HTML, 2026-11-09

`library_ui.go` implements the updated HTML's `章节练习`, `随机练习` and `我的笔记` boards in native MyGo UI. These three boards extend the review board's 240px sidebar, embedded Review Noto weights and exact dark colors; a light variant remains available. Main insets are 32px vertically and 40px horizontally; page headings are 28px/700 with 14px subtitles and 24px section gaps. The legacy paper layout no longer governs these landing pages.

Chapter practice has four subject tabs, a highlighted resume strip and three columns of chapter cards with 16px gaps, 18px radii, 20px padding, 6px progress tracks and real source/local statistics. Selecting a chapter opens the existing native reader. The random setup has a flexible configuration card, 24px column gap and 360px summary/history rail; subject chips support multiple selection while legacy single-subject settings remain readable. Rounds are drawn only on explicit start and remain without replacement. The exam tile matches the design but is access-disabled with an explanatory tooltip: deferred exam submission is outside this UI-only change. Summary subject bars show available pool counts, not fabricated per-subject random outcomes. Recent history shows only the existing real round, not the HTML's fictional dates/scores.

Notes uses a 380px searchable/filterable list and a flexible detail card separated by 20px. It displays complete original question content and rich note content, existing scratch drawings, and an autosaving editor. Cause tags are shown only when explicitly present in the user's note text; no new cause diagnosis, dates, sample notes or attempts are injected. Original text remains unchanged; list previews alone have a visual line limit. Empty states use the same card geometry.

## Unified question readers — 2026-11-09

Chapter and random question readers now use the same `reviewPage`, metadata, option rows, footer, typography, palette and answer-card rail as wrong-question review. `View` no longer routes either mode to the legacy paper reader. Header breadcrumbs retain the actual mode/subject/chapter. The chapter rail labels its sequence `本章练习 / 顺序作答` (or all chapters when applicable), while random uses `本轮随机练习 / 固定题序`. The fraction/progress track indicates current position. Answer cells retain the existing today's-local-attempt status semantics; explanatory copy makes that scope explicit.

The secondary answer-card action returns to the chapter dashboard or random setup rather than reshuffling. Only wrong-question review retains its explicit reshuffle action. Card navigation never grades or redraws; random order and cursor remain the existing persisted values. The shared writing panel remains mutually exclusive. Recall quality and unknown-answer controls use the same native context menu in every reader, and the displayed keyboard shortcuts work across all three.

## Native infinite canvas and study overlays — 2026-11-09

The latest user request replaces the workbook's lightweight scratch pane with a full native canvas. The supplied `无限画布草稿`, `进度面板` and `本题笔记卡` boards are appearance/interaction references only. No HTML integration layer, WebView or runtime JavaScript is used. `internal/ink` remains the renderer; `internal/inkcanvas` adapts the existing demo's native controller and caches, while `cmd/ink-demo` stays independently runnable and unchanged.

At 1440×900 the canvas uses dark #15161C or light #faf9f5 paper, a world-aligned 26px dot grid, a 720px world-space question anchored at world (360,110), 24px edge insets, and 44px controls in the bottom toolbar. The question retains its world anchor when notes open; the 332px note card remains screen-fixed at right24/top92. An explicit labeled “本题笔记” button at right24/top24 opens and closes the note card; the minimap is removed. The progress popup is left24/top84/380px with a 4px top progress rule. Toolbar/icon/border geometry is derived from the reference; settings provides the additional engine tools and theme toggle. Narrow desktop layouts raise the central toolbar; the question keeps its complete natural world-space height at every window size. Pan/zoom the canvas to reach content beyond the viewport.

Default pen/pencil ink adapts existing neutral strokes to white on dark paper and black on light paper. Explicit yellow/purple strokes retain their colors. Dark-paper highlighting uses translucent source-over for visibility; light-paper highlighting retains engine multiply. Theme changes invalidate the native caches, not source geometry. Point pressure, smoothing, non-destructive erasures and vector selection transformations remain the ink controller's native behavior.

Progress shows today's real local attempts within the existing queue, elapsed canvas time and an explicitly described 1.5-minute-per-remaining-question estimate. Filters never change the queue or review schedules. The note card saves explicit cause tags, complete text and immutable vector clips; private/sync state only controls gallery visibility. Source examples and mock statistics are not inserted. Scratch documents and camera positions now live only in the App session map per question. Closing the canvas or switching questions retains them; a fresh App starts blank. Historical persisted ink/strokes remain untouched but are not restored. Only explicit note clips and note text are saved to the learning file.

## Canvas icons and fixed-question navigation — 2026-11-09

Canvas tool icons now use pinned Phosphor Icons Regular (MIT), the same open icon family already vendored for the standalone ink demo: pen-nib, highlighter, eraser, lasso, pencil-simple, hand-palm, sliders-horizontal and undo/redo. Assets are embedded unmodified except the redo arrow, which mirrors the pinned library undo icon. `internal/nativeapp/assets/phosphor` retains license, upstream commit/URLs and hashes. Render at 22px within existing 44px targets.

The question is a world-space object sharing the ink camera, as clarified by the user. Its world rectangle stays at (360,110), width720; screen position is world×zoom+camera offset. The complete native UI subtree scales with the camera, including text/options/borders, without layout reflow. Navigation over it is converted from transformed local coordinates to screen DIPs before being routed to the ink controller. Long questions and explanations expand to their full natural height, with no inner Scroll, height cap or special Option-wheel reading mode. Pan/zoom the world canvas to inspect them. This routing does not consume ordinary answer clicks or change the queue. Home restores 100% and horizontally centers the starting world region for the current viewport. Floating controls, progress and notes remain screen-space UI. This supersedes the earlier screen-fixed interpretation.

## Canvas question crispness — 2026-11-09

The world-space question is laid out at the ink camera's scale: width 720×zoom, with padding, gaps, radii, borders, badge/type sizes, option heights and letter blocks multiplied by the same factor. It is not a 720-DIP card with a bitmap-scaling transform; MyGo's element transform magnifies the glyph texture, which made text and inline formulas visibly soft above 100%. Input local to the card is translated by the card's screen origin only. Raster source images use one stable cached bitmap laid out at the card scale; changing the Markdown image URL per zoom step would reload asynchronously and flicker. Their pixels, not vector geometry, still limit sharpness.

## Option exclusion — 2026-11-09

A 450ms long press on an answer option toggles a local exclusion mark. Excluded options use a 0.5-opacity content label, a faded border/background and letter block, and one native strike line per text line. Long press again restores; only another long press restores an excluded option; short clicks do nothing until restored. Number shortcuts skip excluded options. The compatible `Record.Excluded` (`blank:letter`) field does not alter source answers or attempts. As of 2026-11-10, exclusions are cleared on every question entry/reset, including same-question reopen and application startup. Tool switches, rerenders and grading within the current visit preserve them.

## Tablet interruption and ink-surface recovery — 2026-11-10

Surface recovery measures the interval between actual ink Draw callbacks, not view builds: focus/layout passes are allowed to build without presenting the canvas. A >1.8s real paint gap requests backend-validated recovery: reuse the retained complete texture when valid, otherwise rebuild from original vectors. A genuine window OnFocus requests the same check without programmatically focusing any widget. The canvas remains focusable on actual input but no longer requests AutoFocus.

A system-canceled pen gesture commits only points received before cancellation. It never appends a cancellation coordinate or bridges to the next press; explicit tool/navigation cancellation retains its separate discard behavior. A repeated down similarly preserves the previous sampled fragment before starting a new gesture. The independent ink demo is unchanged.

Deferred-save timing is unchanged (600ms pause / maximum 3s), but JSON encoding, atomic file writing and fsync run on a worker over a detached model snapshot. Only immutable committed vector samples are shared. UI-thread completion applies persisted raw metadata without replacing newer live records. Explicit save, navigation, grading, import/export and closing wait for earlier writes, so an older worker cannot overwrite a newer snapshot. No background callback changes keyboard focus.

## Lasso editing and transient scratch — 2026-11-10

Lasso mode exposes copy, paste, delete and save-to-note in one floating action row; the canvas context menu offers the same commands. Native clipboard data uses a versioned private vector format, preserving pressure, stroke settings and erasure masks. Cmd-C copies detached samples, Cmd-V pastes and selects an offset copy (or places it near the current view center if the source is offscreen), Delete/Backspace deletes the selection. Cmd-Z and Cmd-Shift-Z undo/redo each edit. Note-card text editing keeps its own keyboard handling. No automatic clipboard restoration occurs.

Scratch and camera snapshots are session-only, keyed by question in App memory. They do not schedule learning-file writes. Explicitly saved note clips remain persistent and independent of the scratch session. Their dashboard previews use a separate read-only renderer, so they cannot replace or reconfigure the live scratch editor. Existing historical ink/strokes fields are preserved for compatibility without loading or updating them.

## Bounded canvas buffers — 2026-11-10

The overview/minimap and its deferred redraw timers are removed. LiveStroke.Reset reuses up to three viewport-sized surfaces across strokes, clearing their logical state and first-draw content; resize or explicit Release drops them. Prefix targets and spare dry/pan targets allocate only when first used. Pan overscan is 96 physical pixels per edge (previously 192), with a 24px guard. Closing the canvas releases editor surface caches while retaining session vector documents; note clips and undo behavior are preserved. No periodic keepalive or per-move full redraw is added.

## Option hold gesture completion — 2026-11-10

After a hold toggles an exclusion, pointer jitter no longer cancels the completed hold. Its click suppression persists through release and rebuilds, until the next explicit option press/keyboard activation or question reset. A release past 450ms resolves a hold even before the scheduled build; a system-canceled gesture does not initiate one. The canvas question has no hover tooltip.

## macOS tablet sample fidelity — 2026-11-10

The pinned vendored MyGo tabletPoint handler reads the position stored in each NSEvent via the same `s.location(ev)` conversion as mouse/drag events. It no longer queries a newer global cursor for an older pressure sample. While the canvas is open, AppKit mouse/tablet event coalescing is disabled for this process, with the prior value restored on close. These settings change no system preferences or other apps. The build/run scripts verify and reapply the small pinned vendor fix; physical tablet/driver verification remains separate from queued-sample, cancellation and native API tests.

## Bounded zoom and explanation recomposition — 2026-11-10

The canvas starts at 80%, centered on the same world-space question anchor. Home restores 80%; the percentage control still restores 100%. App-specific zoom limits are 50%–200%, including loaded session camera values. Buttons change zoom by 1.1×, while wheel/pinch input uses damped logarithmic growth capped at 1.12× per input event. Pointer-anchored zoom preserves its world point. The standalone editor defaults remain compatible unless explicitly configured.

Zoom-preview settling measures time from the last zoom input, not from each paint. Its Draw callback schedules the settle frame itself. Opening/closing analysis or errata requests a progressive recomposition of the complete ink base; requests carry a generation and survive an active zoom/rebake. The last complete viewport stays visible while the rebake advances in bounded batches. A change in the card's normalized natural height uses the same path, so submitting an answer does not block on a one-frame full ink rebuild. No vector samples, question content or session notes are discarded, and long questions still expand to natural height with camera navigation.

## Markdown notes and pasted images — 2026-11-10

Reader notes and the canvas note card add a compact native Markdown/preview toggle; the dashboard keeps its existing read/edit flow. Preview uses the same native rich renderer for headings, emphasis, lists, code, links and images. Editing remains Markdown source, with the unobtrusive hint “Markdown · ⌘V 粘贴图片”. Pasting an actual clipboard image inserts a short Markdown image reference at the current Unicode selection; plain text retains normal native paste. Attachments are stored alongside the question record and included in existing backups, keeping the editor source small and preserving offline access. Images are content-addressed and retained even when a reference is removed; no automatic destructive cleanup occurs. The existing file permissions and atomic save behavior are unchanged.

## Interruptible navigation rebakes — 2026-11-10

Pure pan repairs exposed strips immediately, using whole-pixel copies of the existing complete surface plus exact vector repair. Zoom camera rebakes process no more than eight visible strokes per frame, skipping offscreen strokes and keeping the last complete view as a temporary preview. A new camera supersedes pending work. The completed replacement swaps atomically; the temporary work uses no more than three viewport-sized textures. Input stops extend no timers except the navigation settle deadline. The original vector samples, pressure, erasures and paint order are unchanged.

DrawRegion/DrawViewport reject segments outside their repair mask before preparing shader uniforms. Immutable prepared geometry uses a bounded FIFO cache (256 strokes, 16,384 segments), keyed by sample identity, translation and AA pixel size, and is released on close. Mutable live samples bypass the cache. Card-height changes caused solely by camera zoom do not invalidate the base during navigation. Original content expansion still requests composition recovery.

Historical CPU benchmark (240 strokes, 40 samples each, 480×320): complete settled zoom ~408ms versus a bake frame ~20ms. The old ~1.7ms pan preview deferred newly exposed ink and was rejected by the user; it is no longer used. Current complete pan repair remains costlier than a stale screenshot and is optimized without dropping coverage.

## Continuous-pan coverage and bounded reuse — 2026-11-10

The previous stop-to-fill strategy is removed. Every pure-pan frame repairs newly exposed regions immediately; this is verified against an independent complete vector render for repeated large deltas and reversals. When translation interrupts an unfinished zoom bake, one complete base at the current camera prevents infinite restart/starvation, then incremental strip repair resumes.

Performance is retained through a bounded spatial index (128-world-unit cells, 131072 ID entries, exact fallback for huge paths) and complete small-stroke mask reuse. Raster mask pixels are limited to 4 MiB, at most 256 masks and 8192 recorded geometry segments; no visited viewport screenshot collection is kept. Full mask identity includes immutable samples, AA scale, translation and subpixel phase. Repair clipping preserves pressure, paint order, translucent ink and erasures. Local CPU pan benchmark improved from about 79ms to about 58ms per full-coverage frame before the additional command budget, with the usual limitation that CPU tests do not measure the user's native GPU/input latency. Visibility is never traded for the lower latency of a blank stale preview.

## Rendering and queue allocation audit — 2026-11-10

The answer-card grid is now a native virtual List of six-cell rows, retaining the existing 32px cell height, 8px gaps and 192px window. Only visible rows and native overscan are constructed, with the same stable question keys and automatic following of the current question. Queue construction avoids duplicate all-library lists and membership scans; review reads avoid cloning state. Fixed source text, current chapter/random round ordering and HLR scheduling remain unchanged.

Rich caches have bounded entry/estimated-byte budgets: prepared Markdown 256 / 2MiB; formula SVG 256 / 8MiB source data; bitmap images 32 / 96MiB RGBA estimate. Eviction reconstructs rendering data from retained original assets or note attachments. The limits do not claim to cap the whole application's RSS/GPU memory. Smooth-curve preparation shares one preallocated output, verified identical to the original algorithm. Benchmarks, verification and limits are recorded in native-review/PERFORMANCE.md.

## Boundary snapshots, sparse history and idle texture retention — 2026-11-10

The active editor is authoritative while writing. Ordinary strokes and undo/redo no longer copy a session document; the latest camera/strokes are captured on navigation, canvas close or explicit save. Scratch remains process-local. Undo retains fifty sparse add/remove/replace commands, sharing immutable vector samples. Complex eraser/lasso gestures can keep a temporary cancellation snapshot; only affected strokes enter committed history, and evicted history entries clear their references. History applies the current theme to neutral ink when restored.

The pinned MyGo Metal patch permits explicit preservation of a complete canvas texture across idle trimming. The editor opts in only the last complete displayed surface, with a backend cap of 128MiB; spare buffers, masks and atlases remain reclaimable. On idle/refocus, backend-confirmed valid pixels are reused. A retained overscan surface is cropped into one viewport copy, and an invalid/different-size/DPR surface falls back to full vector recovery. This uses no heartbeat frames and retains original samples during live-stroke cache recovery. Shutdown releases even opted-in textures. Real-Metal tests verify retention, unchanged pixels, spare release, budget fallback and renderer shutdown.

## Pen capture diagnostics and regional tile cache

The native ink surface explicitly opts in to pen capture isolation. MyGo rejects foreign-device pointer down/move/up before changing the held pen's position, buttons or capture. Other controls keep normal mouse behavior. Genuine cancellation still commits only observed pen samples as a fragment. Direct controller calls apply the same isolation. A bounded in-memory trace records delivery gaps, repeated downs, cancellation and ignored foreign events; the canvas settings action “复制画布诊断” copies counts and the latest 32 anomaly timestamps, without question text, coordinates or ink data. Delivery timestamps measure UI receipt cadence, not tablet hardware sampling.

The existing overscan viewport remains the immediate display and idle-retention surface. Its exposed strips, and local erase/move repairs, are now composed from world-anchored 256px raster blocks with 2px gutters. Blocks use the exact current physical zoom and theme, and preserve source paint order including translucent/erased ink. A lasso drag composites z-order runs of the document: static prefix, selected/unselected contiguous segments (clipped layers or direct vector drawing for highlighter/oversized cases), then the suffix; only drop commits the translated vectors. Deletion and local repairs clear and repaint only each tile’s dirty sub-rectangles; unaffected tile pixels are copied, never recomposited. Only changed bounds invalidate resident blocks; ordered insertion/deletion aligns unchanged stroke headers. The LRU table holds at most 124 blocks under a 32MiB RGBA estimate. This is an additional raster-cache table budget, not the application's total memory limit; active viewport resources and backend allocations remain separate. Complete tile command scenes can replay after GPU trimming, and reset/close releases the table without deleting vectors. No tile waits for a pan-idle timer. Zoom retains its existing temporary preview and bounded vector rebake; this change does not cache unlimited zoom levels or promise cheaper first visits.

## Device-separated native question annotation

The app's canvas has a cached paper backdrop, the existing world-space interactive question card, then a transparent world-space ink surface at z=3. Floating controls and note/progress panels retain their higher z order. The ink surface opts into `PenInputOnly`: native hit testing admits pen pointer events there, while mouse/trackpad clicks, text selection, context menus, scrolling and pinch reach the card beneath it. Pen capture isolation remains active during contact. On blank paper, a trackpad/mouse primary drag pans for drawing tools; deliberate lasso/eraser tools remain usable there. The independent ink demo retains its prior opaque and mouse-writing behavior.

Only ink lives in dry/pan/tile/navigation surfaces. The paper surface is one reusable full-viewport texture, rerendered on camera/theme/size changes and released with surface caches; writing frames reuse it. Transparent repairs copy pixels outside the replaced subrectangles, then paint replacement ink into clear alpha; drawing transparent paint over stale pixels would not erase them. Region-erase preview replaces the ink output, rather than overlaying an erased image on an unerased dry layer. Source highlighter strokes remain unchanged; transparent annotation uses source-over to keep color visible over either native theme. Selection run ordering, complete-view retention, memory budgets and source samples remain intact.

Stem text owns a native selectable container and a context menu with normal Copy/Select All plus explicit full-stem/Markdown copy. Reader/canvas choices continue to use their controlled native option buttons and long-press exclusions. Canvas clipboard/tool shortcuts yield while the user interacts with question text. No browser, HTML overlay or runtime JavaScript is used.
