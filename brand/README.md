# loopback brand

Tokens are in [`tokens.json`](tokens.json): colors for the `dark` and `light` themes, type styles, spacing and radius. The font is JetBrains Mono (Google Fonts, OFL). The rules below say how to use them.

loopback is a daily coding trainer for intermediate developers learning Rust and Go. The interface looks like the tools the learner works in: a terminal and a code editor. It is quiet, monospace and exact.

## Content fundamentals

- Write lowercase for the name, buttons, labels and meta lines: `loopback`, `run checks`, `lesson 04 · 5 min`. Capitalize only code identifiers and proper nouns (Rust, Go, Redis).
- Address the learner as "you" and give imperative instructions: "Return an error when the input has no `+` prefix."
- Keep sentences short and literal. Name the exact function, test or file. No exclamation marks, no emoji, no praise filler.
- Report results with a glyph and a word: `✓ pass`, `✗ fail`, `! hint`. Follow a failure with what was expected, what was returned and one next step.
- Quote real project content in examples (`TestParseSimpleString`, `+OK\r\n`), never lorem ipsum.

## Visual foundations

- **Mono first.** Set every piece of text in the `mono` family (JetBrains Mono, loaded from Google Fonts). Use the `display`, `title`, `heading`, `subheading`, `body`, `body-sm`, `label`, `code` and `gutter` styles; add no other sizes.
- **Color.** Put text in `ink` on `surface-0`, `surface-1` and `surface-2`; secondary text in `ink-muted`. The neutrals are IDE grey with no tint, so the interface sits next to an editor without competing with it. `accent` is the only brand color, a muted steel blue, and appears once or twice per screen: prompt glyphs, links, focus rings, the primary button, progress. Use `on-accent` for text on an accent fill.
- **Status.** `pass` shares the accent hue, `fail` is orange and `warn` is yellow. Always add the glyph and the word; never rely on color alone. Put status text on its `-subtle` background only for the check result and review lines.
- **Themes.** Follow the IDE: `dark` is the primary theme and `light` is its counterpart. The plugin sets `data-theme` from the IDE theme; the web app follows the system setting.
- **Surfaces.** Separate regions with `line` hairlines and the step from `surface-0` to `surface-1` to `surface-2`. Draw control borders in `line-strong`. Use no shadows and no gradients.
- **Shape.** Use `radius-none` for cards and code blocks, `radius-sm` for buttons, inputs and chips, `radius-md` only for popovers and dialogs.
- **Spacing.** Use the `space-1` to `space-12` steps on a 4px grid. Cards pad with `space-4`; groups are `space-6` apart; the page gutter is `space-4`.
- **Layout.** Left-align everything. Content columns stay under 72 characters wide. Put meta information in a `label` line above the title.
- **Code.** Show code in the `code` style on `surface-1` with a `gutter` of line numbers in `ink-muted`. Color syntax with the `syn-*` tokens. Mark a flagged line in a review with `fail-subtle` and a `✗` in the gutter.
- **Focus.** Draw a solid 2px `accent` outline with a 2px offset on every focusable element.
- **Motion.** The only animation is a block cursor blinking in two steps per second. Stop it when `prefers-reduced-motion` is set. Nothing else moves.

## Iconography

Use no icon set. Use the characters `>`, `$`, `_`, `✓`, `✗`, `!`, `·` and `→` in the `mono` family. The brand mark in `brand/logo` is a prompt chevron with a cursor: use `mark-on-dark.svg` on dark surfaces and `mark-on-light.svg` on light ones. The word mark is `loopback` set in `title` or `display`, lowercase, with no logo lockup.

## Components

The components are plain markup with `lb-` class names. The reference styles and live previews are in the design system artifact: <https://claude.ai/artifact/5dp6AJLGrT2i3xDkWGHVHa> (private to the owner). Rules for each:

### Button

A text button in the `label` voice. Use `lb-btn lb-btn--primary` for the one action that moves the learner forward (`run checks`, `start lesson`) and plain `lb-btn` for everything else; show at most one primary button per view.

The consumer provides the label as lowercase text of one to three words. Mark a button that cannot be used yet with the `disabled` attribute: it turns dashed and `ink-muted`, and its label must still say what it does.

Do not use icons inside buttons. Do not make a button wider than its content except on phone width, where it may fill the row.

### Prompt

A single-line text field styled as a terminal prompt, for quiz answers, search and short learner input. Wrap a `lb-prompt__glyph` (`$` for answers, `>` for search) and an `lb-input` inside `lb-prompt`. Add a `lb-label` line above it with a real `for`/`id` pair.

The consumer provides the label and the placeholder text. The empty state shows the blinking `lb-cursor` only where the field is decorative; a real input uses the browser caret, colored `accent`. Show errors in a `fail` line below the field, starting with `✗`.

### LessonCard

One lesson in the fixed path. Structure: an `lb-label` meta line (`language · project · lesson NN · N min`), an `lb-lesson__title`, an optional `lb-lesson__body` of at most two sentences, and a foot with the XP reward and one button.

Use `lb-lesson--current` on the lesson the learner does today; it is the only highlighted card in a list. A finished lesson drops the body, shows `✓` before the XP and offers `redo`. A locked lesson is not shown as a card: the path is fixed and learners cannot skip ahead.

The consumer provides the real lesson number, duration and XP. The lesson number is shown because the order is the information.

### CheckResult

The output of the automated check after the learner runs it. Render one `lb-check__row` per test: a pass row is plain `surface-1`, a fail row is `lb-check__row--fail` with an `lb-check__detail` showing `expected:` and `got:` on aligned lines, and a hint row is `lb-check__row--warn`.

Every status has a glyph and a word (`✓ pass`, `✗ fail`, `! hint`). Put the failing tests first when the check has failed, and end the list with one sentence on the next step. The consumer provides the test names, timings and the expected/got values straight from the test run.

### XpBar

Shows level and XP as a 20-segment track, with the daily goal and streak on a second `lb-xp__meta` line. Fill `lb-xp__seg--on` for the share of XP earned in the level (segments = round(xp / next-level xp × 20)).

The text always states the numbers (`340 / 500 xp`, `7-day streak`), so the bar never carries information alone. The consumer provides level, XP, the level's XP target, the streak length and whether today's lesson is done. Do not animate the fill except a single step when XP is earned.

### CodeBlock

Shows source code with a gutter of line numbers. Use `lb-code__line` per line with an `lb-code__no` and an `lb-code__src`, and color tokens with `syn-k` (keyword), `syn-s` (string), `syn-f` (function and type), `syn-n` (number) and `syn-c` (comment).

For a bad-code review, add `lb-code__line--flag` to the flagged line (the gutter shows `✗` before the number) and an `lb-code__note` row directly under it that says what is wrong and what to do. The consumer provides the code as text; escape `<`, `>` and `&`. Long lines scroll inside the block; never wrap code.
