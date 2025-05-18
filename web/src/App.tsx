import { createEffect, createSignal, Match, Switch } from 'solid-js'
import { setToken, setUser } from './store'
import { API_BASE_URL } from '~/lib/api'
import { NavigationProvider } from './lib/useNavigation'
import { QueryClient, QueryClientProvider } from '@tanstack/solid-query'
import { LocaleContextProvider } from '~/i18n/locale-context'

export const queryClient = new QueryClient({
	defaultOptions: {
		queries: {
			retry: 2,
			staleTime: 1000 * 60 * 5, // 5 minutes
			gcTime: 1000 * 60 * 5, // 5 minutes
		},
		mutations: {
			retry: 2,
		},
	},
})

export default function App(props: any) {
	const [isAuthenticated, setIsAuthenticated] = createSignal(false)
	const [isLoading, setIsLoading] = createSignal(true)

	// Set up theme based on Telegram colorScheme
	createEffect(() => {
		// Get the color scheme from Telegram WebApp
		const scheme = window.Telegram?.WebApp?.colorScheme || 'dark'

		// Remove both theme classes to avoid conflicts
		document.documentElement.classList.remove('light', 'dark')
		// Add the appropriate theme class
		document.documentElement.classList.add(scheme)

		// Set up event listener for theme changes
		if (window.Telegram?.WebApp) {
			const handleThemeChange = () => {
				const newScheme = window.Telegram.WebApp.colorScheme || 'dark'
				document.documentElement.classList.remove('light', 'dark')
				document.documentElement.classList.add(newScheme)
			}

			window.Telegram.WebApp.onEvent('themeChanged', handleThemeChange)
		}
	})

	createEffect(async () => {
		try {
			console.log('WEBAPP:', window.Telegram)

			const initData = window.Telegram.WebApp.initData

			const resp = await fetch(`${API_BASE_URL}/auth/telegram`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
				},
				body: JSON.stringify({ query: initData }),
			})

			if (resp.status !== 200) {
				setIsAuthenticated(false)
				setIsLoading(false)
				return
			}

			const data = await resp.json()

			setUser(data.user)
			setToken(data.token)

			// Set language based on user preference if available
			if (data.user?.language_code) {
				const langCode = data.user.language_code
				// Store language preference
				localStorage.setItem('locale', langCode === 'ru' ? 'ru' : 'en')
			}

			window.Telegram.WebApp.ready()
			window.Telegram.WebApp.expand()
			window.Telegram.WebApp.disableClosingConfirmation()
			window.Telegram.WebApp.disableVerticalSwipes()
			window.Telegram.WebApp.requestWriteAccess()

			// window.Telegram.WebApp.CloudStorage.removeItem('fb_community_popup')

			setIsAuthenticated(true)
			setIsLoading(false)

		} catch (e) {
			console.error('Failed to authenticate user:', e)
			setIsAuthenticated(false)
			setIsLoading(false)
		}
	})
	return (
		<LocaleContextProvider>
			<NavigationProvider>
				<QueryClientProvider client={queryClient}>
					<Switch>
						<Match when={isAuthenticated()}>
							{props.children}
						</Match>
						<Match when={!isAuthenticated() && isLoading()}>
							<div class="min-h-screen w-full flex-col items-start justify-center bg-background" />
						</Match>
						<Match when={!isAuthenticated() && !isLoading()}>
							<div
								class="flex text-center h-screen w-full flex-col items-center justify-center text-3xl">
								<p>
									Today nothing is gonna work
								</p>
							</div>
						</Match>
					</Switch>
				</QueryClientProvider>
			</NavigationProvider>
		</LocaleContextProvider>
	)
}
