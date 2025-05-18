import type { RouteDefinition } from '@solidjs/router'
import NavigationTabs from '~/components/navigation-tabs'
import Index from '~/pages'
import Cards from '~/pages/cards'
import ImportDeck from '~/pages/import-deck'
import EditCard from '~/pages/edit-card'
import Statistics from '~/pages/stats'
import Profile from '~/pages/profile'
import Tasks from '~/pages/tasks'
import Task from '~/pages/task'


export const routes: RouteDefinition[] = [
	{
		path: '/',
		component: NavigationTabs,
		children: [
			{
				'path': '/',
				'component': Index,
			},
			{
				'path': '/tasks',
				'component': Tasks,
			},
			{
				'path': '/stats',
				'component': Statistics,
			},
			{
				'path': '/profile',
				'component': Profile,
			},
		],
	},
	{
		'path': '/cards/:deckId',
		'component': Cards,
	},
	{
		'path': '/import-deck',
		'component': ImportDeck,
	},
	{
		'path': '/edit-card/:deckId/:cardId',
		'component': EditCard,
	},
	{
		'path': '/tasks/:deckId',
		'component': Task,
	},
]
