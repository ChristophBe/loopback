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
  const text = readFileSync(file, 'utf8')
  if (file.endsWith('.md')) {
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

// Validates every content file under dir, plus the check-result fixtures.
export function validateTree(contentDir = join(root, 'content'), fixtureDir = join(root, 'api/schemas/fixtures')) {
  const failures = []
  // template-* directories hold the learner template repos, not lessons.
  const files = walk(contentDir)
    .filter((f) => !relative(contentDir, f).startsWith('template-'))
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
