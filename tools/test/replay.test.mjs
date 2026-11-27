import assert from 'node:assert/strict'
import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import { test } from 'node:test'

import { replay, testCommands } from '../replay.mjs'

// The fake "test command" fails when any .txt file in the project contains BROKEN.
const script = `const fs=require('fs');const bad=fs.readdirSync('.',{recursive:true}).filter(f=>f.endsWith('.txt')&&fs.readFileSync(f,'utf8').includes('BROKEN'));if(bad.length){console.log('broken: '+bad);process.exit(1)}`
const commands = { go: ['node', '-e', script], rust: ['node', '-e', script] }

const put = (dir, path, text) => {
  mkdirSync(dirname(join(dir, path)), { recursive: true })
  writeFileSync(join(dir, path), text)
}

const lesson = (n, extra = '') =>
  `---\nid: go-demo-0${n}\ntype: lesson\nlanguage: go\nproject: demo\norder: ${n}\nownedFiles:\n  - a.txt\nreferenceSolution: solutions/0${n}\n${extra}---\n`

function tree(solutions) {
  const dir = mkdtempSync(join(tmpdir(), 'replay-test-'))
  put(dir, 'template-go/go.mod', 'module x\n')
  put(dir, 'template-rust/Cargo.toml', '')
  solutions.forEach((text, i) => {
    put(dir, `go/demo/0${i + 1}.md`, lesson(i + 1))
    if (text !== null) put(dir, `go/demo/solutions/0${i + 1}/a.txt`, text)
  })
  return dir
}

test('passes with valid solutions and a rust language without lessons', () => {
  const log = []
  assert.deepEqual(replay(tree(['one', 'two']), { commands, log: (l) => log.push(l) }), [])
  assert.deepEqual(log, [
    'ok   go demo (template)',
    'ok   go go-demo-01',
    'ok   go go-demo-02',
    'ok   rust template (template)',
  ])
})

test('fails and names the lesson whose solution is broken, skipping later lessons', () => {
  const failures = replay(tree(['one', 'BROKEN', 'three']), { commands })
  assert.equal(failures.length, 1)
  assert.equal(failures[0].lesson, 'go-demo-02')
  assert.match(failures[0].message, /broken: a\.txt/)
})

test('fails and names the lesson when the solution file is missing', () => {
  const failures = replay(tree(['one', null]), { commands })
  assert.equal(failures[0].lesson, 'go-demo-02')
  assert.match(failures[0].message, /lacks a\.txt/)
})

test('fails when the template itself does not pass', () => {
  const dir = tree([])
  put(dir, 'template-rust/x.txt', 'BROKEN')
  const failures = replay(dir, { commands })
  assert.equal(failures[0].language, 'rust')
  assert.match(failures[0].lesson, /template/)
})

test('the real commands are go test and cargo test', () => {
  assert.deepEqual(testCommands, { go: ['go', 'test', './...'], rust: ['cargo', 'test'] })
})
