import { render, screen } from '@testing-library/react'

import { App } from './App'
import { createApi } from './api'

function apiReturning(response: Response) {
  return createApi('http://localhost/api/v1', () => Promise.resolve(response))
}

describe('App', () => {
  it('shows the gamification rules from the API', async () => {
    const body = {
      version: 'test',
      gamification: { xpPerLesson: 50, xpPerLevel: 500, dailyGoalLessons: 1 },
    }
    const response = new Response(JSON.stringify(body), {
      headers: { 'Content-Type': 'application/json' },
    })

    render(<App api={apiReturning(response)} />)

    expect(await screen.findByText(/50 xp per lesson/)).toBeInTheDocument()
  })

  it('shows a failure when the API is not reachable', async () => {
    render(<App api={apiReturning(new Response('nope', { status: 500 }))} />)

    expect(await screen.findByText(/✗ fail/)).toBeInTheDocument()
  })
})
