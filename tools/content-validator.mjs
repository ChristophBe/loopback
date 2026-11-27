// Validates content files against the schemas in api/schemas.
// Lessons are Markdown files with YAML front matter. Quizzes, flashcards and reviews
// are JSON files. The "type" field selects the schema for a JSON file.
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'

import Ajv from 'ajv'
import matter from 'gray-matter'

export const root = new URL('..', import.meta.url).pathname

const load = (path) => JSON.parse(readFileSync(join(root, path), 'utf8'))
const ajv = new Ajv({ allErrors: true, strict: true })
const lessonSchema = load('api/schemas/lesson.schema.json')

const lesson = ajv.compile(lessonSchema)
const checkResult = ajv.compile(load('api/schemas/check-result.schema.json'))
// JSON content, selected by its "type" field.
const exercises = {
  quiz: ajv.compile(load('api/schemas/quiz.schema.json')),
  flashcard: ajv.compile(load('api/schemas/flashcard.schema.json')),
  review: ajv.compile(load('api/schemas/review.schema.json')),
}

export const requiredBodySections = lessonSchema.definitions.requiredBodySections.const

export function walk(dir) {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name)
    return statSync(path).isDirectory() ? walk(path) : [path]
  })
}

// Turns an Ajv error into "<field> <message>", naming the offending field.
function describe(e) {
  const path = e.instancePath.split('/').slice(1).join('.')
  const extra = e.params.missingProperty ?? e.params.additionalProperty
  const field = [path, extra].filter(Boolean).join('.') || '(root)'
  return { field, message: e.message }
}

function schemaProblems(validate, data) {
  return validate(data) ? [] : validate.errors.map(describe)
}

// Level-2 headings outside fenced code blocks.
function headings(body) {
  let fence = false
  const found = []
  for (const line of body.split('\n')) {
    if (/^\s*(```|~~~)/.test(line)) fence = !fence
    else if (!fence && /^## /.test(line)) found.push(line.slice(3).trim())
  }
  return found
}

function bodyProblems(body) {
  const found = headings(body)
  const problems = requiredBodySections
    .filter((s) => !found.includes(s))
    .map((s) => ({ field: 'body', message: `is missing the section "## ${s}"` }))
  const order = found.filter((h) => requiredBodySections.includes(h))
  if (!problems.length && requiredBodySections.some((s, i) => order[i] !== s)) {
    problems.push({ field: 'body', message: `must contain the sections ${requiredBodySections.map((s) => `"## ${s}"`).join(', ')} in this order` })
  }
  return problems
}

// Returns [{ field, message }] for one content file. An empty array means valid.
export function validateFile(file) {
  return validateText(readFileSync(file, 'utf8'), file.endsWith('.md'))
}

// Validates the text of a lesson (markdown) or of a JSON exercise.
export function validateText(text, markdown) {
  if (markdown) {
    const { data, content } = matter(text)
    return [...schemaProblems(lesson, data), ...bodyProblems(content)]
  }
  let data
  try {
    data = JSON.parse(text)
  } catch (e) {
    return [{ field: '(root)', message: `is not valid JSON: ${e.message}` }]
  }
  const validate = Object.hasOwn(exercises, data?.type) ? exercises[data.type] : undefined
  if (!validate) {
    return [{ field: 'type', message: 'must be one of quiz, flashcard, review' }]
  }
  return schemaProblems(validate, data)
}

// template-* directories hold the learner template repos and solutions directories the reference solutions, not content.
export const isContentPath = (contentDir, file) => {
  const segments = relative(contentDir, file).split('/')
  return !segments[0].startsWith('template-') && !segments.includes('solutions')
}

// Validates every content file under dir, plus the check-result fixtures.
export function validateTree(contentDir = join(root, 'content'), fixtureDir = join(root, 'api/schemas/fixtures')) {
  const failures = []
  const files = walk(contentDir)
    .filter((f) => isContentPath(contentDir, f))
    .filter((f) => /\.(md|json)$/.test(f))
  for (const file of files) {
    for (const p of validateFile(file)) failures.push({ file: relative(root, file), ...p })
  }
  const fixtures = walk(fixtureDir).filter((f) => f.endsWith('.json'))
  for (const file of fixtures) {
    for (const p of schemaProblems(checkResult, JSON.parse(readFileSync(file, 'utf8')))) {
      failures.push({ file: relative(root, file), ...p })
    }
  }
  return { failures, files: files.length, fixtures: fixtures.length }
}

// Fenced examples in a doc are marked with `<!-- example: <type> -->` on the line before the fence.
const exampleRe = /<!-- example: (\w+) -->\n(`{3,}|~{3,})\w*\n([\s\S]*?)\n\2$/gm

export function docExamples(docFile) {
  return [...readFileSync(docFile, 'utf8').matchAll(exampleRe)].map((m) => ({ type: m[1], text: m[3] + '\n' }))
}

// Validates every marked example in a doc and requires one for each content type.
export function validateDoc(docFile = join(root, 'docs/content-format.md')) {
  const file = relative(root, docFile)
  const examples = docExamples(docFile)
  const failures = []
  for (const type of ['lesson', 'quiz', 'flashcard', 'review']) {
    const n = examples.filter((e) => e.type === type).length
    if (n !== 1) failures.push({ file, field: 'examples', message: `must contain exactly one "${type}" example, found ${n}` })
  }
  for (const e of examples) {
    const problems = validateText(e.text, e.type === 'lesson')
    if (!problems.length && e.type !== 'lesson' && JSON.parse(e.text).type !== e.type) {
      problems.push({ field: 'type', message: `must be "${e.type}" in the ${e.type} example` })
    }
    for (const p of problems) failures.push({ file, ...p, message: `${p.message} (${e.type} example)` })
  }
  return { failures, examples: examples.length }
}
