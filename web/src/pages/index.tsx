import { createResource, For, Show } from 'solid-js'
import { apiRequest, Deck, Stats } from '~/lib/api'
import { useNavigate } from '@solidjs/router'

export default function Index() {
	const navigate = useNavigate()

	const [decks] = createResource<Deck[]>(async () => {
		const { data, error } = await apiRequest<Deck[]>('/decks')
		if (error) {
			console.error('Failed to fetch decks:', error)
			return []
		}
		return data || []
	})

	const [stats] = createResource<Stats>(async () => {
		const { data, error } = await apiRequest<Stats>('/stats')
		if (error) {
			console.error('Failed to fetch stats:', error)
			return { due_cards: 0 }
		}
		return data || { due_cards: 0 }
	})

	const handleSelectDeck = (deckId: string) => {
		navigate(`/cards/${deckId}`)
	}

	return (
		<div class="container mx-auto px-4 py-6 max-w-md flex flex-col items-center">
			<div class="w-full bg-card text-card-foreground rounded-lg shadow-md p-4 mb-6">
				<h2 class="text-xl font-bold mb-2">Flashcards</h2>
				<p class="text-muted-foreground">
					<Show when={stats()} fallback="Loading stats...">
						{stats()?.due_cards} cards due for review
					</Show>
				</p>
			</div>
			<div class="w-full">
				<h3 class="text-lg font-medium mb-4">Select a Deck</h3>
				<div class="space-y-3">
					<Show when={!decks.loading} fallback={<p class="text-muted-foreground">Loading decks...</p>}>
						<For each={decks()}>
							{(deck) => (
								<button
									onClick={() => handleSelectDeck(deck.id)}
									class="w-full bg-card text-card-foreground p-4 rounded-lg shadow-sm transition-colors flex justify-between items-center"
								>
									<div>
										<h4 class="font-medium">{deck.name}</h4>
										<p class="text-sm text-muted-foreground">{deck.description}</p>
									</div>
									<span class="text-primary text-sm font-medium">{deck.level}</span>
								</button>
							)}
						</For>
					</Show>
				</div>
			</div>
		</div>
	)
}
