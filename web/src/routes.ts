import type { RouteDefinition } from '@solidjs/router'

import NavigationTabs from '~/components/navigation-tabs'
import Index from '~/pages'
import Cards from '~/pages/cards'
import EditCard from '~/pages/edit-card'
import Task from '~/pages/task'

export const routes: RouteDefinition[] = [
  {
    path: '/',
    component: Index,
  },
  {
    path: '/cards/:deckId',
    component: Cards,
  },
  {
    path: '/edit-card/:deckId/:cardId',
    component: EditCard,
  },
  {
    path: '/tasks/:deckId',
    component: Task,
  },
]
