import { setupServer } from 'msw/node'

import {
  elasticsearchHandlers,
  handlers,
  mysqlHandlers,
  rabbitMQHandlers,
} from './fixtures'

export const server = setupServer(
  ...handlers,
  ...mysqlHandlers,
  ...elasticsearchHandlers,
  ...rabbitMQHandlers,
)
