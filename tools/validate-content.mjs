// Validates lesson front matter and check-result fixtures against api/schemas.
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'

import Ajv from 'ajv'
import matter from 'gray-matter'

const root = new URL('..', import.meta.url).pathname
const ajv = new Ajv({ allErrors: true })

const load = (path) => JSON.parse(readFileSync(join(root, path), 'utf8'))
const lesson = ajv.compile(load('api/schemas/lesson.schema.json'))
const checkResult = ajv.compile(load('api/schemas/check-result.schema.json'))

function walk(dir) {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name)
    return statSync(path).isDirectory() ? walk(path) : [path]
  })
}

let failed = false
const report = (file, validate) => {
  for (const e of validate.errors) {
    failed = true
    console.error(`${relative(root, file)}: ${e.instancePath || '/'} ${e.message}`)
  }
}

const lessons = walk(join(root, 'content')).filter((f) => f.endsWith('.md'))
for (const file of lessons) {
  if (!lesson(matter(readFileSync(file, 'utf8')).data)) report(file, lesson)
}

const fixtures = walk(join(root, 'api/schemas/fixtures')).filter((f) => f.endsWith('.json'))
for (const file of fixtures) {
  if (!checkResult(JSON.parse(readFileSync(file, 'utf8')))) report(file, checkResult)
}

if (failed) process.exit(1)
console.log(`ok: ${lessons.length} lesson(s), ${fixtures.length} fixture(s)`)
