import { readFileSync } from 'node:fs'

import type { FlashcardExercise, QuizExercise, BadCodeReviewExercise } from './gen/content'

function example<T>(name: string): T {
  const path = `../api/schemas/examples/${name}-valid.json` // tests run from web/
  return JSON.parse(readFileSync(path, 'utf8')) as T
}

describe('generated content types', () => {
  it('describe the valid quiz example', () => {
    const quiz = example<QuizExercise>('quiz')
    expect(quiz.type).toBe('quiz')
    expect(quiz.questions[0]?.choices.length).toBeGreaterThan(1)
  })

  it('describe the valid flashcard example', () => {
    expect(example<FlashcardExercise>('flashcard').cards.length).toBeGreaterThan(0)
  })

  it('describe the valid review example', () => {
    expect(example<BadCodeReviewExercise>('review').flaggedLines.length).toBeGreaterThan(0)
  })
})
