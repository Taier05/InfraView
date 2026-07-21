import { setupServer } from 'msw/node'

import { handlers } from './fixtures'

export const server = setupServer(...handlers)
