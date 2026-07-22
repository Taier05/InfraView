import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { App } from './app/App'

const root = document.getElementById('root')
if (root === null) throw new Error('无法找到应用挂载节点')

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
