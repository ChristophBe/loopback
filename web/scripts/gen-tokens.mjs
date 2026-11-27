// Generates src/gen/tokens.css from ../brand/tokens.json.
import { readFileSync, writeFileSync } from 'node:fs'

const tokens = JSON.parse(readFileSync(new URL('../../brand/tokens.json', import.meta.url), 'utf8'))

const colors = (theme) =>
  tokens.color.tokens.map((t) => `  --lb-${t.name}: ${t.value[theme]};`).join('\n')

const fixed = [
  ...Object.entries(tokens.type.families).map(([name, value]) => `  --lb-font-${name}: ${value};`),
  ...tokens.spacing.tokens.map((t) => `  --lb-${t.name}: ${t.value};`),
  ...tokens.radius.tokens.map((t) => `  --lb-${t.name}: ${t.value};`),
  ...tokens.type.groups.flatMap((g) =>
    g.styles.flatMap((s) =>
      [
        `  --lb-type-${s.name}-size: ${s.fontSize};`,
        `  --lb-type-${s.name}-line: ${s.lineHeight};`,
        `  --lb-type-${s.name}-weight: ${s.fontWeight};`,
        s.letterSpacing ? `  --lb-type-${s.name}-tracking: ${s.letterSpacing};` : null,
      ].filter(Boolean),
    ),
  ),
].join('\n')

const css = `/* Generated from brand/tokens.json by scripts/gen-tokens.mjs. Do not edit. */
:root {
${fixed}
${colors('dark')}
  color-scheme: dark;
}

@media (prefers-color-scheme: light) {
  :root:not([data-theme='dark']) {
${colors('light')
  .split('\n')
  .map((l) => `  ${l}`)
  .join('\n')}
    color-scheme: light;
  }
}

:root[data-theme='light'] {
${colors('light')}
  color-scheme: light;
}
`

writeFileSync(new URL('../src/gen/tokens.css', import.meta.url), css)
