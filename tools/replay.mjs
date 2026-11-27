// Replays the lessons in order on top of the template repos: copies content/template-<language>,
// applies each lesson's reference solution over it and runs the language's tests after every step.
// A language without lessons only has its template checked. Usage: node replay.mjs [content dir]
import { cpSync, existsSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync } from 'node:fs'
import { spawnSync } from 'node:child_process'
import { tmpdir } from 'node:os'
import { dirname, join, relative } from 'node:path'
import { pathToFileURL } from 'node:url'

import matter from 'gray-matter'

import { root, walk } from './content-validator.mjs'

export const languages = ['go', 'rust']
export const testCommands = { go: ['go', 'test', './...'], rust: ['cargo', 'test'] }

const runCommand = ([cmd, ...args], cwd) => {
  const r = spawnSync(cmd, args, { cwd, encoding: 'utf8' })
  return { ok: r.status === 0, output: `${r.stdout ?? ''}${r.stderr ?? ''}${r.error ?? ''}` }
}

// Lessons by language, each list sorted by project and order. Lessons live outside template-* and solutions dirs.
export function loadLessons(contentDir) {
  const lessons = walk(contentDir)
    .filter((f) => f.endsWith('.md') && !relative(contentDir, f).split('/').some((s) => s.startsWith('template-') || s === 'solutions'))
    .map((file) => ({ file, ...matter(readFileSync(file, 'utf8')).data }))
  return Object.fromEntries(
    languages.map((l) => [l, lessons.filter((x) => x.language === l).sort((a, b) => a.project.localeCompare(b.project) || a.order - b.order)]),
  )
}

// Returns a list of failures [{ language, lesson, message }]; empty means every step passed.
export function replay(contentDir = join(root, 'content'), { commands = testCommands, log = () => {} } = {}) {
  const lessons = loadLessons(contentDir)
  const failures = []
  const work = mkdtempSync(join(tmpdir(), 'loopback-replay-'))
  try {
    for (const language of languages) {
      const template = join(contentDir, `template-${language}`)
      const fail = (lesson, message) => failures.push({ language, lesson, message })
      if (!existsSync(template)) {
        fail('template', `missing directory ${relative(contentDir, template)}`)
        continue
      }
      // One fresh copy per project, because a project builds on its own earlier lessons only.
      const projects = [...new Set(lessons[language].map((l) => l.project))]
      for (const project of projects.length ? projects : [undefined]) {
        const dir = join(work, language, project ?? 'template')
        cpSync(template, dir, { recursive: true })
        const check = (label) => {
          const { ok, output } = runCommand(commands[language], dir)
          log(`${ok ? 'ok  ' : 'FAIL'} ${language} ${label}`)
          if (!ok) fail(label, `${commands[language].join(' ')} failed\n${output}`)
          return ok
        }
        if (!check(`${project ?? 'template'} (template)`)) continue
        for (const lesson of lessons[language].filter((l) => l.project === project)) {
          const label = lesson.id ?? relative(contentDir, lesson.file)
          const solution = lesson.referenceSolution && join(dirname(lesson.file), lesson.referenceSolution)
          if (!solution) {
            fail(label, 'has no referenceSolution')
            break
          }
          const missing = lesson.ownedFiles.filter((f) => !existsSync(join(solution, f)))
          if (missing.length) {
            fail(label, `reference solution lacks ${missing.join(', ')}`)
            break
          }
          for (const f of lesson.ownedFiles) {
            mkdirSync(dirname(join(dir, f)), { recursive: true })
            cpSync(join(solution, f), join(dir, f))
          }
          if (!check(label)) break
        }
      }
    }
  } finally {
    rmSync(work, { recursive: true, force: true })
  }
  return failures
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  const failures = replay(process.argv[2], { log: console.log })
  for (const f of failures) console.error(`replay failed at lesson ${f.lesson} (${f.language}): ${f.message}`)
  process.exit(failures.length ? 1 : 0)
}
