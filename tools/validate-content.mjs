// CLI for `make lint-content` (optional args: content dir, fixture dir): prints "<file>: <field> <message>" for every problem.
import { validateDoc, validateTree } from './content-validator.mjs'

const tree = validateTree(...process.argv.slice(2))
const doc = process.argv.length > 2 ? { failures: [], examples: 0 } : validateDoc()
const failures = [...tree.failures, ...doc.failures]
const { files, fixtures } = tree
for (const f of failures) console.error(`${f.file}: ${f.field} ${f.message}`)
if (failures.length) process.exit(1)
console.log(`ok: ${files} content file(s), ${fixtures} fixture(s), ${doc.examples} doc example(s)`)
