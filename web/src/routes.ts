import type { RouteDefinition } from '@solidjs/router'
import NavigationTabs from '~/components/navigation-tabs'
import Index from '~/pages'


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
]
