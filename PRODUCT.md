# 408 研习

## Platform
Native desktop application. Every interactive surface uses Go + `github.com/egoist/mygo/ui`. No WebView, HTML page, React runtime or local HTTP server.

## Users and purpose
The user practices their supplied 408 question collection, reviews due mistakes, and keeps scratch work and question-linked notes locally. Desktop only.

## Capabilities and constraints
Preserve 1,576 original questions, stable source slugs, images, mathematical formulas, tables, 30 errata and imported history. New attempts stay separate from source snapshots. Preserve random rounds, HLR scheduling, undo, backups, notes and the native infinite ink canvas.

## Confirmed design decisions
Rework the old interface instead of reproducing it. Reference mygo-music's native navigation and Notion's quiet hierarchy. Preserve the supplied Noto Sans SC Bold font and light/Dracula themes. The question is centered when reading; a mutually exclusive scratch/notes pane opens on the right and fills available height. Triggers are beside the question heading.

## Stack
Go owns state, review calculations, rendering, input, atomic persistence and desktop lifecycle. MyGo native Markdown lays out rich original material; generated SVG outlines are embedded formula assets, not a browser surface. Formula generation uses MathJax and fontTools at asset-build time only; ordinary application builds require Go alone.

## Native canvas update
Scratch now opens a full native infinite canvas with a floating question, progress popup, explicit notes button, bottom tools and per-question note card. Black/white default ink adapts to light/dark paper. Vector selections can be saved as note clips. Existing legacy normalized strokes remain retained, and the standalone ink demo remains independent.
