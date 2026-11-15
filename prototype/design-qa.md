# Design QA — 408 研习

## Source and evidence
- Source visual truth: `.impeccable/review/source.png`, the second displayed image in the final warm Liquid Glass ideation set selected by the user.
- Original source pixels: 1487 × 1058. Normalized to 1440 × 1024 in `source-normalized.png`; aspect-ratio difference is below 0.2%.
- Desktop implementation: `.impeccable/review/desktop.jpg`, 1440 × 1024 CSS viewport, density 1, captured in the in-app browser.
- Tablet implementation: `.impeccable/review/tablet.jpg`, 834 × 1194 CSS viewport, density 1.
- Mobile implementation: `.impeccable/review/mobile.jpg`, 390 × 844 CSS viewport, full-page capture. The browser's persistent vertical scrollbar slightly reduces drawable page width; no horizontal overflow.
- Full-view joint comparison: `.impeccable/review/comparison.jpg` (source left, implementation right).
- Focused typography/chrome comparison: `.impeccable/review/type-comparison.jpg` (source top, implementation bottom).
- State: first data-structure demonstration question, no selected answer, blank scratchpad and notes.

## Intentional content differences
- The user explicitly allowed an empty shell or demo for scratchpaper. The implementation provides a blank interactive canvas rather than a prefilled worked solution.
- Notes start blank rather than fabricating personal notes or revealing a solution before answering.
- Progress is 01 / 08, reflecting the actual eight authored demo questions rather than the mock's illustrative 04 / 20.
- Previous-question control is disabled on the first question.

## Comparison history
1. Initial browser inspection found a P2 horizontal-overflow defect: liquid-glass highlight siblings participated in flex layout and increased desktop scroll width to 1667 at a 1440 viewport. Fixed by absolutely positioning the library layers inside the submit-control slot, containing decorative layers and reducing the library's default shadow. This was a real visual fix, not a build troubleshooting step.
2. Post-fix capture: desktop scroll width 1440 at viewport1440; tablet834 at viewport834; mobile no horizontal overflow. Revised screenshots listed above contain the fix.

## Required fidelity surfaces
- Typography: bundled Noto Serif SC with locally served weights 400/500/600, matching the Chinese editorial serif character; the independent review classified the smaller type scale as optional P3 polish.
- Spacing/layout: horizontal navigation, upper question/options section, lower large scratchpad and right note pane retained. Narrow screens switch tools with tabs rather than shrinking columns.
- Color/tokens: warm apricot/sage optical background, ivory reading surface, dark olive CTA, thin beige rules. Legible surfaces remain more opaque than glass chrome.
- Image quality: generated 1536 × 1024 background based on the selected visual; independent asset and prompt retained. Icons use Phosphor. Drawing surface is a functioning canvas, not a placeholder illustration.
- Copy/content: all eight problems labelled demonstration; no fabricated exam provenance or performance metrics. Main visible Chinese controls follow the source; scratch demo/save/privacy labels clarify actual prototype behavior.

## Primary interactions tested
- Submission without a choice displays an actionable prompt.
- An incorrect answer adds the question to wrong questions and shows the correct answer/explanation after submission.
- Wrong-question mode starts with choices and explanation hidden; submitting compares previous and current answers.
- Explicit mastery removes a question from the redo queue while retaining notes and attempt history.
- Notes can be edited, listed, reopened and restored after browser reload.
- Four-subject filter and previous/next question navigation operate.
- Pointer drag draws on the demo canvas; clear and undo restore expected states.
- Mobile tool tabs reveal the note editor.
- In-app browser console warning/error logs: empty in the final checked session.
- Production build: passed; no unresolved asset warnings after dependency installation completed.

## Detector
One advisory identifies the grid-line canvas background. This is intentionally an actual scratchpaper canvas, which the detector's rule explicitly permits.

## Findings
Independent finish review disposition: **ship**. No P0/P1/P2 findings in the supplied final desktop, tablet and mobile evidence. No UI edits were made after this review.

P3 follow-up: implementation typography is slightly smaller/lighter than the selected mock, particularly question text and navigation; it remains readable and coherent. Optional tuning can bring it closer to the reference. The generated background's optical highlights also differ slightly from the mock, while preserving the selected art direction.

Scope of review: visual evidence was independently inspected; the functional checks above were performed by the implementation agent in the in-app browser. Full screen-reader behavior and numeric WCAG contrast compliance have not been certified. A production accessibility audit remains outside this prototype's completed checks.



## Imported collection update

The source collection now contains 1,576 user-supplied questions rather than eight demo questions. This is a content/interaction extension of the chosen visual, not a redesign. Source counts, errata, independent wrong-book membership and original snapshots are verified in data/relax-1000-27/导入报告.md.

- Browser-rendered evidence: .impeccable/review/import/desktop.jpg (1440×1024 viewport, full page); tablet.jpg (834×1194); mobile.jpg (390×844 full-page); history-review.jpg; directory-mobile-fixed.jpg.
- First small-screen directory inspection exposed a P2 flex-shrink issue in the chapter/history selectors. Replaced the directory controls with a two-selector grid plus close control, then full-width search row. Final controls each measure135px at a390px viewport and the screenshot confirms readable labels.
- No horizontal overflow at desktop1440, tablet834 or mobile390.
- Browser tests: imported wrong-book badge249 and queue249; full catalogue1576; filters30errata/235error/205dont-know; multi-blank completeness guard and correct sequence; original history separate from new result; tuple options restored; four graph-option images loaded; table in original analysis visible; refresh retained imported membership.
- Final browser console warnings/errors: none.
- Full programmatic render:9474 non-empty fields,73 tables,243 image references, no KaTeX/render errors. This is not character-by-character OCR or answer correctness verification.
- Source serif typography, optical backdrop, glass navigation, workbook proportions and green primary action retained. New source/type labels and the collapsible directory use existing colors and spacing.
- Source code font warning is a sanctioned technical-content exception; narrow sfmono-regular ignore persisted and the actual code font role is recorded in DESIGN.md. No whole-rule/file suppression used.
- No open P0/P1/P2 findings in the verified import states. Earlier optional typography refinement remains P3.



## Random practice update

- Added a four-item navigation with a dedicated random question drawer, using the existing olive/ivory controls and typography.
- Wrong-question rounds shuffle once on entry or explicit reshuffle; ordering stays fixed through question navigation and refresh. Mastered removals preserve the relative order of remaining questions.
- Drawer supports subject, requested count, and all/current-wrong/locally-unpracticed pools. Sampling is without replacement; invalid or oversized requests are rejected. Redrawing creates a new round.
- Round IDs, cursor, selected mode and settings persist independently of source history and per-question records.
- Browser checks: wrong queue shuffled; explicit reshuffle changed order; reload retained all displayed wrong queue rows. OS/current-wrong scope offered46 questions; drawing3 produced3 unique in-scope rows; refresh at2/3 restored the same cursor and queue. Last question stopped at3/3 with Next disabled. Source wrong-book count remained249.
- Browser screenshots: .impeccable/review/random/desktop.jpg (1440×1024 viewport/full-page), mobile.jpg (390×844/full-page), tablet.jpg (834×1194/full-page).
- No horizontal overflow in desktop/mobile/tablet measurements. Final checked browser warning/error logs were empty.
-11 focused tests passed, including5 new round/sampling regression tests and6 original import/history tests. Production build passed; existing large-dataset bundle-size advisory remains unrelated to correctness.
- No additional P0/P1/P2 findings in these checked states. The previous optional P3 typography adjustment is unchanged.


## Scheduled wrong-question review

- Current scope is desktop and tablet only, per the user's explicit preference. No further phone-specific work is planned.
- Local HLR adaptation filters due wrong questions before randomization; all-wrongs browsing and a due-only drawer pool remain available. Correct answers preserve wrong-book membership.
- Browser checks: first independent success scheduled tomorrow and reduced due count; uncertain success and explicit unknown scheduled ten-minute relearning. Undo restored the prior schedule and count while preserving a newer note. All test attempts were undone and the note restored.
- Initial state remains 249 original wrong-book members, 248 cold-start questions due, and 1 disputed answer key awaiting manual checking. Original undated records did not acquire fictional dates.
- Browser verified all-wrongs count249, due drawer pool248, and reload restoration of today's review view. Desktop1440 and tablet834 were checked; tablet document width826 fits its viewport. Browser warning/error logs were empty.
- Evidence: .impeccable/review/scheduler/preview.jpg, desktop.jpg, tablet.jpg, correct-plan.jpg.
-31 focused import, random-round and scheduler tests passed. Final production build passed; the pre-existing dataset bundle-size advisory remains.
- Model coefficients are cold-start estimates, not validated 408 learner parameters. Documentation records the HLR adaptation and distinguishes it from SM20/LECTOR.
- No additional P0/P1/P2 issues found in the checked review states.

final result: passed
