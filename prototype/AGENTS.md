# Prototype Instructions

## Confirmed user decisions

- Superseded Web prototype: the current desktop app is entirely Go + mygo ui in ../internal/nativeapp. React/KaTeX files here are historical and are not loaded by the app. Follow ../AGENTS.md for current native work.
- User explicitly does not use mobile phones. Prioritize desktop and tablet layouts; do not spend follow-up work on mobile-phone adaptation or mobile QA.
- Latest user design decision (2026-11-09): replace Liquid Glass with paper minimalism; provide light and black themes. Desktop only.
- Old `.impeccable/review/source.png` is historical and no longer visual authority. Current tokens: `src/paper.css`.
- Preserve horizontal navigation, upper question area, lower scratchpad and right notes column. Use flat paper and thin rules.
- Scratchpaper may remain an empty shell or lightweight demo. Do not expand into handwriting recognition or a full notebook editor without a new request.
- Current collection is the user-supplied relax-1000-27.xlsx, 1,576 questions. The original 8 demo questions are archived, not active.
- Preserve source slug IDs and the 20 workbook columns. Rich local details matched all normalized fields; formatting may be restored from those verified details.
- Import 30 errata and 249 wrong-book flags; preserve 235 error vs 205 dont-know snapshots separately. Do not infer membership from outcome.
- Keep source answer/analysis and errata separate, including disputed keys. Do not silently rewrite answers or regrade historical snapshots.
- New local attempts remain separate from imported snapshots; repeat imports must preserve user notes and mastery changes.
- Wrong-question rounds must use a random question order. Generate the order at the start of a round, keep it stable while answering/filtering/editing notes, and restore it on refresh.
- Provide a random question drawer with subject, pool and count. Sample without replacement within a round; only explicit redraw begins a new round.
- Latest review requirement: filter wrong questions by the local HLR due schedule before randomizing. Default to today's due questions, with a separate all-wrongs view. Correct answers update schedules and never automatically remove membership.
- Use the actual new review history and elapsed time; undated imports remain cold-start priors without fabricated review dates. Keep unknown/uncertain/incorrect outcomes distinct; short or early retries must not inflate long-term retention.
- Source keys explicitly marked disputed remain in the wrong book for manual checking and are excluded from automatic scheduling. See REVIEW-ALGORITHM.md for the actual model and guardrails.

Run the local server yourself and open the preview in the browser available to this environment. Do not give the user server-start instructions when you can run it.

Before making substantial visual changes, use the Product Design plugin's `get-context` skill when the visual source is unclear or no longer matches the current goal. When the user gives durable prototype-specific design feedback, preferences, or decisions, record them in `AGENTS.md`.

When implementing from a selected generated mock, treat that image as the source of truth for layout, component anatomy, density, spacing, color, typography, visible content, and hierarchy.

Build app UI in `src/`. Keep `.openai/hosting.json`, `worker/index.js`, `scripts/prepare-sites-build.mjs`, and `tests/sites-worker.test.mjs` intact so the same local prototype can be handed to Sites. Before a Sites handoff, run `npm run build` and `npm run test:sites`; the build must leave `dist/client/index.html`, `dist/server/index.js`, and `dist/.openai/hosting.json`.

- Latest theme decision (2026-11-09): replace the black theme with Dracula (purple accent, #282a36 background, #f8f8f2 foreground); preserve the light theme and paper layout.

- Latest font decision (2026-11-09): use the user-supplied NotoSansSC-Bold.otf for interface, headings, questions, answers and notes. Bundle the exact file locally at public/fonts/NotoSansSC-Bold.otf. Keep KaTeX mathematical fonts and code monospace. The supplied face is Bold only.

- Writing tools: scratchpad and question notes are separate, independently toggled panels. Both are hidden on startup; show explicit open/close buttons at the right of the question heading. Hiding preserves notes and strokes. Opening one closes the other. The notes list may open its target note directly.

- Cohesion update: user dislikes fragmented choices and overall UI. Use a continuous paper reading surface, vertical borderless answer rows with compact letter/radio markers, and one shared action bar for writing tools, previous/next and submit. Avoid boxed choices and redundant container rules.

- Latest layout decision: center the entire question/answers/action column by default. Opening scratchpad or notes shifts the question column left and docks those tools on the right. Tools are hidden on startup. Latest correction: only one may be open; switching tools replaces the current right pane. Restore centered reading when both close.

- Latest writing-panel decision: scratchpad and notes are mutually exclusive, controlled by a single active-tool state. Fill the entire available right-hand writing area. Place their labeled trigger buttons at the right of the question title block (the user supplied a screenshot of this area), not in the bottom action bar.
