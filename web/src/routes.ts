import type { RouteDefinition } from '@solidjs/router'
import NavigationTabs from '~/components/navigation-tabs'
import Index from '~/pages'
import Cards from '~/pages/cards'
import ImportDeck from '~/pages/import-deck'


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
]
