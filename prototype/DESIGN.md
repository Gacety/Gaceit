# 408 研习 — 桌面纸张

This replaces the old Liquid Glass direction at the user's request (2026-11-09).

Mode: Operate, with a Read surface for original questions and solutions.

Use flat warm-white paper on a light gray desk, or Dracula paper (#282a36) on a deeper blue-gray desk (#21222c). One continuous workbook, fine rules, small rectangular controls, quiet sage selection in light mode and purple accents in Dracula mode and the user-supplied Noto Sans SC Bold reading face. No glass, optical background or decorative gradients.

The first viewport holds horizontal navigation, original source metadata, question and vertical answer rows. Writing tools are independent, collapsed panels. Keep the question footer in normal document flow so long source material remains accessible. Random practice, due-review and notes share the same paper system.

The light/dark switch is explicit and persisted. All colors in `src/paper.css` use theme tokens, except original source images, which retain a white backing to protect their fidelity. Mathematical typography uses KaTeX. Scratch grid and ruled notes are functional writing guides.

Desktop minimum width 1060px. No mobile UI or mobile QA. Keyboard focus, selected answers, errors, save progress/failure and empty states remain visible in both themes. Reduced-motion settings disable control transitions.

Dracula palette reference: https://draculatheme.com/contribute . Dark secondary text is lifted to #a5adce for readable small labels; borders/selection use #44475a, accent #bd93f9, errors #ff5555.

Typography update: the exact supplied NotoSansSC-Bold.otf is self-hosted under public/fonts/ and used for the UI and all Chinese reading surfaces in both themes. It is a single static Bold face, not a multi-weight family; mathematical fonts and code monospace remain specialized.

Writing tools update: scratchpad and question notes are independent full-width panels, collapsed on startup. Each has its own trigger and close control; both may be open. Collapse never clears saved notes or drawings.

## Continuous reading surface

The page has one paper background from navigation through question and actions. Unboxed vertical answer rows retain the natural A–D reading order; small letter circles carry hover/selected/correct/error states, with a light row tint where needed. No individual option cards. A single bottom action bar groups writing-tool triggers, previous/next and submit. Metadata stays secondary, and blank space separates groups instead of stacking horizontal rules. Review, random and notes views inherit the same flat surface. Preserve all original source content, keyboard radio navigation and independent collapsed writing tools.

## Centered reading and right writing sidebar
Default question, answer and action column is centered with an 840px maximum reading width. Opening either writing tool expands the workspace into a left reading column and right sticky sidebar. Only one tool may be open; switching tools replaces the active pane and closing it restores the centered column. Preserve state and scroll access to long questions and writing panels.

Latest panel correction: use a single mutually exclusive active tool. Right panel fills the available viewport height, with the canvas or note textarea flexing below its heading. Tool triggers sit at the right of the question title block.
