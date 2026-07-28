import { setupServer } from 'msw/node'

import { handlers, mysqlHandlers } from './fixtures'

export const server = setupServer(...handlers, ...mysqlHandlers)
