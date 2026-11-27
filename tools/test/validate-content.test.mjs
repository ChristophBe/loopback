import assert from 'node:assert/strict'
import { cpSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, writeFileSync } from 'node:fs'
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

test('referenceSolution accepts relative directories and rejects unsafe paths', () => {
  const schema = JSON.parse(readFileSync(join(root, 'api/schemas/lesson.schema.json'), 'utf8'))
  const re = new RegExp(schema.properties.referenceSolution.pattern)
  for (const ok of ['solutions/01-a', 'solutions', 'a.b/c_d']) assert.ok(re.test(ok), ok)
  for (const bad of ['', '.', './x', 'a/./b', '..', 'a/..', 'a/../b', '/abs', 'a/', 'a//b', 'a\\b']) assert.ok(!re.test(bad), bad)
})

test('solutions directories are not validated as content', () => {
  const dir = mkdtempSync(join(tmpdir(), 'content-'))
  mkdirSync(join(dir, 'go/p/solutions/01'), { recursive: true })
  writeFileSync(join(dir, 'go/p/solutions/01/notes.md'), 'not a lesson')
  writeFileSync(join(dir, 'go/p/solutions/01/data.json'), '{}')
  assert.equal(validateTree(dir).files, 0)
})

// Writes a lesson with starter and test files into a temp content tree and returns the lesson path.
function lessonTree(files, front = 'starterFiles: starter/01-a\ntestFiles: tests/01-a\n') {
  const dir = mkdtempSync(join(tmpdir(), 'lesson-'))
  const lesson = readFileSync(join(fixtures, 'valid', 'lesson.md'), 'utf8')
  const owned = 'ownedFiles:\n  - resp/parse.go\n  - resp/parse_test.go\n'
  writeFileSync(join(dir, '01-a.md'), lesson.replace(/ownedFiles:\n( {2}- .*\n)+/, owned + front))
  for (const [rel, text] of Object.entries(files)) {
    mkdirSync(join(dir, rel, '..'), { recursive: true })
    writeFileSync(join(dir, rel), text)
  }
  return join(dir, '01-a.md')
}

const complete = { 'starter/01-a/resp/parse.go': 'package resp\n', 'tests/01-a/resp/parse_test.go': 'package resp\n' }

test('lesson with starter and test files for every owned file is valid', () => {
  assert.deepEqual(validateFile(lessonTree(complete)), [])
})

test('lesson without starterFiles and testFiles is valid', () => {
  assert.deepEqual(validateFile(lessonTree({}, '')), [])
})

test('owned file missing from the starter and test directories is named', () => {
  const problems = validateFile(lessonTree({ 'starter/01-a/resp/parse.go': 'package resp\n', 'tests/01-a/resp/other.txt': 'x' }))
  assert.deepEqual(problems, [
    { field: 'testFiles', message: 'file "resp/other.txt" in "tests/01-a" is not listed in ownedFiles' },
    { field: 'ownedFiles', message: 'file "resp/parse_test.go" is missing from the starterFiles and testFiles directories' },
  ])
})

test('missing directory is reported on its field', () => {
  const problems = validateFile(lessonTree({ 'starter/01-a/resp/parse.go': 'x', 'starter/01-a/resp/parse_test.go': 'x' }))
  assert.deepEqual(problems.map((p) => p.field), ['testFiles'])
})

test('starterFiles with an unsafe path is rejected on that field', () => {
  const problems = validateFile(lessonTree(complete, 'starterFiles: ../starter\ntestFiles: tests/01-a\n'))
  assert.deepEqual(problems.map((p) => p.field), ['starterFiles'])
})

test('starter and tests directories are not walked for lessons', () => {
  const lesson = lessonTree({ ...complete, 'starter/01-a/resp/notes.md': 'no front matter', 'tests/01-a/resp/data.json': '{' })
  const contentDir = mkdtempSync(join(tmpdir(), 'tree-'))
  const project = join(contentDir, 'go', 'p')
  mkdirSync(project, { recursive: true })
  for (const name of ['starter', 'tests']) cpSync(join(lesson, '..', name), join(project, name), { recursive: true })
  const { files } = validateTree(contentDir)
  assert.equal(files, 0)
})

test('starter file that is not UTF-8 text is reported', () => {
  const problems = validateFile(lessonTree({ ...complete, 'starter/01-a/resp/parse.go': 'a\0b' }))
  assert.deepEqual(problems, [
    { field: 'starterFiles', message: 'file "resp/parse.go" in "starter/01-a" is not UTF-8 text' },
    { field: 'ownedFiles', message: 'file "resp/parse.go" is missing from the starterFiles and testFiles directories' },
  ])
})
