import { createResource, For, Show, createSignal } from 'solid-js'
import { apiRequest, Deck } from '~/lib/api'
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

	const handleSelectDeck = (deckId: string) => {
		navigate(`/cards/${deckId}`)
	}

	const handleImportDeck = () => {
		navigate('/import-deck')
	}

	const handleUpdateDeck = (updatedDeck: Deck) => {
		refetch()
	}

	const handleDeleteDeck = () => {
		// When a deck is deleted, refresh both decks and stats
		refetch()
	}

	return (
		<div class="container mx-auto px-4 py-6 max-w-md flex flex-col items-center overflow-y-auto h-screen pb-24">
			<div class="w-full">
				<div class="flex justify-between items-center mb-4">
					<h3 class="text-lg font-medium">Select a Deck</h3>
					<button
						onClick={handleImportDeck}
						class="text-sm font-medium text-primary flex items-center"
					>
						<svg
							xmlns="http://www.w3.org/2000/svg"
							height="24px"
							viewBox="0 -960 960 960"
							width="24px"
							class="size-5 mr-1"
							fill="currentColor">
							<path
								d="M440-440H240q-17 0-28.5-11.5T200-480q0-17 11.5-28.5T240-520h200v-200q0-17 11.5-28.5T480-760q17 0 28.5 11.5T520-720v200h200q17 0 28.5 11.5T760-480q0 17-11.5 28.5T720-440H520v200q0 17-11.5 28.5T480-200q-17 0-28.5-11.5T440-240v-200Z" />
						</svg>
						New Deck
					</button>
				</div>
				<div class="space-y-2">
					<Show when={!decks.loading} fallback={<p class="text-muted-foreground">Loading decks...</p>}>
						<For each={decks()}>
							{(deck) => (
								<div
									class="bg-card w-full text-card-foreground p-4 rounded-xl transition-colors flex justify-between items-center">
									<button
										onClick={() => handleSelectDeck(deck.id)}
										class="flex-1 flex justify-between items-center text-start"
									>
										<div>
											<h4 class="font-medium">{deck.name}</h4>
											<p class="text-xs text-muted-foreground mt-1">
												{deck.new_cards_per_day} new cards per day
											</p>
											<div class="flex gap-2 text-xs text-muted-foreground mt-1">
												<Show when={deck.stats?.new_cards && deck.stats.new_cards > 0}>
													<span class="text-blue-500">{deck.stats?.new_cards} new</span>
												</Show>
												<Show when={deck.stats?.learning_cards && deck.stats.learning_cards > 0}>
													<span class="text-yellow-500">{deck.stats?.learning_cards} learning</span>
												</Show>
												<Show when={deck.stats?.review_cards && deck.stats?.review_cards > 0}>
													<span class="text-green-500">{deck.stats?.review_cards} review</span>
												</Show>
												<Show when={(!deck.stats?.review_cards || deck.stats?.new_cards === 0) &&
													(!deck.stats?.learning_cards || deck.stats?.learning_cards === 0) &&
													(!deck.stats?.review_cards || deck.stats?.review_cards === 0)}>
													<span class="text-muted-foreground">No cards to study</span>
												</Show>
											</div>
										</div>
									</button>
									<button
										onClick={(e) => {
											e.stopPropagation()
											setSelectedDeck(deck)
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
											<path
												d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" />
											<circle cx="12" cy="12" r="3" />
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
									Add Your First Deck
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
					onDelete={handleDeleteDeck}
					onClose={() => setSelectedDeck(null)}
				/>
			)}
		</div>
	)
}
