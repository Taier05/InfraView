import { setupServer } from 'msw/node'

import {
  elasticsearchHandlers,
  handlers,
  mysqlHandlers,
} from './fixtures'

export const server = setupServer(
  ...handlers,
  ...mysqlHandlers,
  ...elasticsearchHandlers,
)
