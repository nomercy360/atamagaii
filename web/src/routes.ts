import type { RouteDefinition } from '@solidjs/router'
import NavigationTabs from '~/components/navigation-tabs'
import Index from '~/pages'
import Cards from '~/pages/cards'
import ImportDeck from '~/pages/import-deck'
import EditCard from '~/pages/edit-card'


export const routes: RouteDefinition[] = [
	{
		path: '/',
		component: NavigationTabs,
		children: [
			{
				'path': '/',
				'component': Index,
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
]
