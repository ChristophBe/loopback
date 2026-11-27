import createClient from 'openapi-fetch'

import type { paths } from './gen/api'

export function createApi(baseUrl = '/api/v1', fetch?: typeof globalThis.fetch) {
  return createClient<paths>({ baseUrl, fetch })
}
