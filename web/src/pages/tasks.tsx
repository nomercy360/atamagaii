import { createResource, Show, For } from 'solid-js'
import { useNavigate } from '@solidjs/router'
import { getTasksPerDeck, TasksPerDeck } from '~/lib/api'
import Animation from '~/components/all-done-animation'

export default function Tasks() {
	const navigate = useNavigate()
	const [tasksPerDeck, { refetch }] = createResource<TasksPerDeck[]>(
		async () => {
			const { data, error } = await getTasksPerDeck()
			if (error) {
				console.error('Failed to fetch tasks:', error)
				return []
			}
			return data || []
		},
	)

	const handleSelectDeck = (deckId: string) => {
		navigate(`/tasks/${deckId}`)
	}

	const handleRefresh = () => {
		refetch()
	}

	return (
		<div class="container mx-auto px-4 py-10 max-w-md flex flex-col items-center min-h-screen">
			<Show when={tasksPerDeck.loading}>
				<div class="w-full flex flex-col items-center justify-center h-[300px]">
					<p class="text-muted-foreground">Loading tasks...</p>
				</div>
			</Show>

			<Show when={!tasksPerDeck.loading && tasksPerDeck()?.length}>
				<div class="w-full space-y-3">
					<For each={tasksPerDeck()}>
						{(item) => (
							<button
								onClick={() => handleSelectDeck(item.deck_id)}
								class="w-full flex items-center justify-between p-4 bg-card rounded-lg border border-border hover:bg-secondary/50 transition-colors"
							>
								<div class="flex-1">
									<h3 class="font-medium text-left">{item.deck_name}</h3>
									<p class="text-sm text-muted-foreground text-left">
										{item.total_tasks} {item.total_tasks === 1 ? 'task' : 'tasks'} available
									</p>
								</div>
								<div class="flex-shrink-0">
									<svg
										xmlns="http://www.w3.org/2000/svg"
										width="24"
										height="24"
										viewBox="0 0 24 24"
										fill="none"
										stroke="currentColor"
										stroke-width="2"
										stroke-linecap="round"
										stroke-linejoin="round"
										class="h-5 w-5 text-muted-foreground"
									>
										<path d="m9 18 6-6-6-6" />
									</svg>
								</div>
							</button>
						)}
					</For>
				</div>
			</Show>

			<Show when={!tasksPerDeck.loading && (!tasksPerDeck() || tasksPerDeck()?.length === 0)}>
				<div class="w-full flex flex-col items-center justify-center h-[400px] px-4">
					<Animation width={100} height={100} class="mb-2" src="/study-more.json" />
					<p class="text-xl font-medium text-center mb-4">No tasks available!</p>
					<p class="text-muted-foreground mb-4 text-center">
						Practice cards in your decks to generate tasks. Tasks appear when cards move to review stage.
					</p>
					<button
						onClick={handleRefresh}
						class="mb-4 px-4 py-2 bg-primary text-primary-foreground rounded-md"
					>
						Check Again
					</button>
					<button
						onClick={() => navigate('/')}
						class="mt-2 text-primary"
					>
						Back to decks
					</button>
				</div>
			</Show>
		</div>
	)
}
