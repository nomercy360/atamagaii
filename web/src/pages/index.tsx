import { createResource, For, Show, createSignal } from 'solid-js'
import { apiRequest, Deck, Stats } from '~/lib/api'
import { useNavigate } from '@solidjs/router'
import DeckSettings from '~/components/deck-settings'

export default function Index() {
	const navigate = useNavigate()
	const [selectedDeck, setSelectedDeck] = createSignal<Deck | null>(null)

	const [decks, { refetch }] = createResource<Deck[]>(async () => {
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

	const handleImportDeck = () => {
		navigate('/import-deck')
	}

	const handleUpdateDeck = (updatedDeck: Deck) => {
		refetch()
	}

	return (
		<div class="container mx-auto px-4 py-6 max-w-md flex flex-col items-center overflow-y-auto h-screen pb-24">
			<div class="w-full bg-card text-card-foreground rounded-lg shadow-md p-4 mb-6">
				<h2 class="text-xl font-bold mb-2">Flashcards</h2>
				<p class="text-muted-foreground">
					<Show when={stats()} fallback="Loading stats...">
						{stats()?.due_cards} cards due for review
					</Show>
				</p>
			</div>
			<div class="w-full">
				<div class="flex justify-between items-center mb-4">
					<h3 class="text-lg font-medium">Select a Deck</h3>
					<button
						onClick={handleImportDeck}
						class="text-sm font-medium text-primary flex items-center"
					>
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="16"
							height="16"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
							stroke-linecap="round"
							stroke-linejoin="round"
							class="mr-1"
						>
							<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
							<polyline points="17 8 12 3 7 8"/>
							<line x1="12" y1="3" x2="12" y2="15"/>
						</svg>
						Import Deck
					</button>
				</div>
				<div class="space-y-3">
					<Show when={!decks.loading} fallback={<p class="text-muted-foreground">Loading decks...</p>}>
						<For each={decks()}>
							{(deck) => (
								<div class="bg-card w-full text-card-foreground p-4 rounded-lg shadow-sm transition-colors flex justify-between items-center">
									<button
										onClick={() => handleSelectDeck(deck.id)}
										class="flex-1 flex justify-between items-center text-start"
									>
										<div>
											<h4 class="font-medium">{deck.name}</h4>
											<p class="text-xs text-muted-foreground">{deck.description}</p>
											<p class="text-xs text-muted-foreground mt-1">
												{deck.new_cards_per_day} new cards per day
											</p>
										</div>
										<span class="text-primary text-sm font-medium">{deck.level}</span>
									</button>
									<button
										onClick={(e) => {
											e.stopPropagation();
											setSelectedDeck(deck);
										}}
										class="ml-4"
									>
										<svg
											xmlns="http://www.w3.org/2000/svg"
											width="18"
											height="18"
											viewBox="0 0 24 24"
											fill="none"
											stroke="currentColor"
											stroke-width="2"
											stroke-linecap="round"
											stroke-linejoin="round"
											class="text-muted-foreground"
										>
											<path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/>
											<circle cx="12" cy="12" r="3"/>
										</svg>
									</button>
								</div>
							)}
						</For>

						{decks() && decks()!.length === 0 && (
							<div class="text-center py-8">
								<p class="text-muted-foreground mb-4">No decks found</p>
								<button
									onClick={handleImportDeck}
									class="px-4 py-2 bg-primary text-primary-foreground rounded-md text-sm font-medium"
								>
									Import Your First Deck
								</button>
							</div>
						)}
					</Show>
				</div>
			</div>

			{selectedDeck() && (
				<DeckSettings
					deck={selectedDeck()!}
					onUpdate={handleUpdateDeck}
					onClose={() => setSelectedDeck(null)}
				/>
			)}
		</div>
	)
}