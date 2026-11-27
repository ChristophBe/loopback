// CLI for `make lint-content` (optional args: content dir, fixture dir): prints "<file>: <field> <message>" for every problem.
import { validateTree } from './content-validator.mjs'

const { failures, files, fixtures } = validateTree(...process.argv.slice(2))
for (const f of failures) console.error(`${f.file}: ${f.field} ${f.message}`)
if (failures.length) process.exit(1)
console.log(`ok: ${files} content file(s), ${fixtures} fixture(s)`)
