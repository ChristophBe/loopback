import assert from 'node:assert/strict'
import { mkdtempSync, readdirSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { spawnSync } from 'node:child_process'
import { join } from 'node:path'
import { test } from 'node:test'

import { docExamples, root, validateDoc, validateFile, validateTree } from '../content-validator.mjs'

const fixtures = new URL('./fixtures/', import.meta.url).pathname
const types = ['lesson', 'quiz', 'flashcard', 'review']
const ext = (t) => (t === 'lesson' ? 'md' : 'json')

for (const t of types) {
  test(`valid ${t} passes`, () => {
    assert.deepEqual(validateFile(join(fixtures, 'valid', `${t}.${ext(t)}`)), [])
  })
  test(`invalid ${t} fails and names fields`, () => {
    const problems = validateFile(join(fixtures, 'invalid', `${t}.${ext(t)}`))
    assert.ok(problems.length > 0)
    assert.ok(problems.every((p) => p.field && p.message))
  })
}

test('invalid lesson names the offending fields', () => {
  const fields = validateFile(join(fixtures, 'invalid', 'lesson.md')).map((p) => p.field)
  assert.ok(fields.includes('xp'), 'missing required field xp')
  assert.ok(fields.includes('minutes'), 'minutes above maximum')
  assert.ok(fields.includes('body'), 'missing section')
})

test('invalid quiz and review name nested fields', () => {
  const quiz = validateFile(join(fixtures, 'invalid', 'quiz.json')).map((p) => p.field)
  assert.ok(quiz.includes('questions.0.answer'))
  const review = validateFile(join(fixtures, 'invalid', 'review.json')).map((p) => p.field)
  assert.ok(review.includes('flaggedLines.0.note'))
})

test('lesson without Objective reports the section', () => {
  const problems = validateFile(join(fixtures, 'invalid', 'lesson-missing-section.md'))
  assert.deepEqual(problems, [{ field: 'body', message: 'is missing the section "## Objective"' }])
})

test('unknown or missing type is reported on the type field', () => {
  const [p] = validateFile(join(fixtures, 'invalid', 'unknown-type.json'))
  assert.equal(p.field, 'type')
})

test('schema examples: valid ones pass, invalid ones fail', () => {
  const dir = join(root, 'api/schemas/examples')
  for (const name of readdirSync(dir)) {
    const problems = validateFile(join(dir, name))
    if (name.endsWith('-valid.json')) assert.deepEqual(problems, [], name)
    else assert.ok(problems.length > 0, name)
  }
})

test('repository content and check-result fixtures are valid', () => {
  assert.deepEqual(validateTree().failures, [])
})

test('CLI prints file and field and exits 1 on failure', () => {
  const dir = join(fixtures, 'invalid')
  const r = spawnSync('node', ['validate-content.mjs', dir], { cwd: join(root, 'tools'), encoding: 'utf8' })
  assert.equal(r.status, 1)
  assert.match(r.stderr, /tools\/test\/fixtures\/invalid\/quiz\.json: questions\.0\.answer /)
})

test('CLI exits 0 on the repository content', () => {
  const r = spawnSync('node', ['validate-content.mjs'], { cwd: join(root, 'tools'), encoding: 'utf8' })
  assert.equal(r.status, 0, r.stderr)
})

test('docs/content-format.md has one valid example of each type', () => {
  const doc = join(root, 'docs/content-format.md')
  assert.deepEqual(docExamples(doc).map((e) => e.type).sort(), [...types].sort())
  assert.deepEqual(validateDoc(doc).failures, [])
})

test('doc examples that are invalid or missing are reported', () => {
  const dir = mkdtempSync(join(tmpdir(), 'doc-'))
  const doc = join(dir, 'doc.md')
  writeFileSync(doc, '<!-- example: quiz -->\n```json\n{"type":"quiz"}\n```\n')
  const { failures } = validateDoc(doc)
  assert.ok(failures.some((f) => f.field === 'title'))
  assert.ok(failures.some((f) => f.field === 'examples' && /"lesson"/.test(f.message)))
})

test('lesson with a malformed referenceSolution is rejected on that field', () => {
  const problems = validateFile(join(fixtures, 'invalid', 'lesson-bad-reference-solution.md'))
  assert.deepEqual(problems.map((p) => p.field), ['referenceSolution'])
})
