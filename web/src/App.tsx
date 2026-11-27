import { useEffect, useState } from 'react'

import { createApi } from './api'
import type { components } from './gen/api'

type Config = components['schemas']['Config']

type State = { status: 'loading' } | { status: 'error' } | { status: 'ready'; config: Config }

export function App({ api = createApi() }: { api?: ReturnType<typeof createApi> }) {
  const [state, setState] = useState<State>({ status: 'loading' })

  useEffect(() => {
    let active = true
    api
      .GET('/config')
      .then(({ data }) => {
        if (active) setState(data ? { status: 'ready', config: data } : { status: 'error' })
      })
      .catch(() => {
        if (active) setState({ status: 'error' })
      })
    return () => {
      active = false
    }
  }, [api])

  return (
    <main className="page">
      <p className="label">dashboard</p>
      <h1>loopback</h1>
      {state.status === 'loading' && <p>$ loading</p>}
      {state.status === 'error' && <p className="fail">✗ fail: could not reach the API</p>}
      {state.status === 'ready' && (
        <p>
          {state.config.gamification.xpPerLesson} xp per lesson · daily goal{' '}
          {state.config.gamification.dailyGoalLessons}
        </p>
      )}
    </main>
  )
}
