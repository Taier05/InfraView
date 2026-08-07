import { setupServer } from 'msw/node'

import {
  elasticsearchHandlers,
  handlers,
  javaHandlers,
  mysqlHandlers,
  rabbitMQHandlers,
} from './fixtures'

export const server = setupServer(
  ...handlers,
  ...mysqlHandlers,
  ...elasticsearchHandlers,
  ...rabbitMQHandlers,
  ...javaHandlers,
)
