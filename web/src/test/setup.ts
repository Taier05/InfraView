import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterAll, afterEach, beforeAll, beforeEach } from 'vitest'

import { resetFixtureState } from './fixtures'
import { server } from './server'

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }))
beforeEach(() => resetFixtureState())
afterEach(() => {
  cleanup()
  server.resetHandlers()
})
afterAll(() => server.close())
