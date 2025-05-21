import { For, Show, createSignal } from 'solid-js'
import { apiRequest, Deck } from '~/lib/api'
import { useNavigate } from '@solidjs/router'
import { useQuery } from '@tanstack/solid-query'
import DeckSettings from '~/components/deck-settings'
import { FlagIcon } from '~/pages/import-deck'
import { useTranslations } from '~/i18n/locale-context'

const DeckSkeleton = () => (
	<div class="space-y-3">
		{Array(3).fill(0).map((_, i) => (
			<div class="bg-card w-full text-card-foreground p-4 rounded-xl">
				<div class="flex justify-between items-center">
					<div class="w-full">
						<div class="h-5 bg-muted rounded animate-pulse w-2/3 mb-2"></div>
						<div class="h-3 bg-muted rounded animate-pulse w-1/3 mb-2"></div>
						<div class="h-3 bg-muted rounded animate-pulse w-1/2"></div>
					</div>
					<div class="ml-4 w-6 h-6 rounded-full bg-muted animate-pulse"></div>
				</div>
			</div>
		))}
	</div>
)

export default function Index() {
	const navigate = useNavigate()
	const [selectedDeck, setSelectedDeck] = createSignal<Deck | null>(null)

	const { t } = useTranslations()

	const decksQuery = useQuery(() => ({
		queryKey: ['decks'],
		queryFn: async () => {
			const { data, error } = await apiRequest<Deck[]>('/decks')
			if (error) {
				console.error('Failed to fetch decks:', error)
				return []
			}
			return data || []
		},
	}))

	const handleSelectDeck = (deckId: string) => {
		navigate(`/cards/${deckId}`)
	}

	const handleImportDeck = () => {
		navigate('/import-deck')
	}

	const handleUpdateDeck = (updatedDeck: Deck) => {
		decksQuery.refetch()
	}

	const handleDeleteDeck = () => {
		// When a deck is deleted, refresh both decks and stats
		decksQuery.refetch()
	}

	const getTotalCards = (deck: Deck) => {
		if (!deck.stats) return 0
		return deck.stats.review_cards + deck.stats.new_cards + deck.stats.learning_cards
	}

	return (
		<div class="container mx-auto px-4 py-6 max-w-md flex flex-col items-center overflow-y-auto h-screen pb-28">
			<div class="w-full">
				<div class="flex justify-between items-center mb-4">
					<h3 class="text-lg font-medium">
						{t('home.deck')}
					</h3>
					<button
						onClick={handleImportDeck}
						class="text-sm font-medium text-primary-foreground flex items-center"
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
						{t('home.add_deck')}
					</button>
				</div>
				<div class="space-y-2">
					<Show when={!decksQuery.isPending} fallback={<DeckSkeleton />}>
						<For each={decksQuery.data}>
							{(deck) => (
								<div
									class="bg-card w-full text-card-foreground p-4 rounded-xl transition-colors flex justify-between items-center">
									<button
										onClick={() => handleSelectDeck(deck.id)}
										class="flex-1 flex justify-between items-center text-start"
									>
										<div>
											<FlagIcon code={deck.language_code} width={16} height={16}
																clsName="rounded-full inline-block mr-1" />
											<div class="flex items-center">

												<h4 class="font-medium">{deck.name}</h4>
											</div>
											<p class="text-xs text-muted-foreground mt-1">
												{deck.new_cards_per_day} {t('home.new_cards_per_day')}
											</p>
											<div class="flex gap-1 text-xs text-muted-foreground mt-2">
												<svg xmlns="http://www.w3.org/2000/svg"
														 height="24px"
														 class="size-4"
														 viewBox="0 -960 960 960"
														 width="24px"
														 fill="currentColor">
													<path
														d="M680-160v-640q33 0 56.5 23.5T760-720v480q0 33-23.5 56.5T680-160ZM160-80q-33 0-56.5-23.5T80-160v-640q0-33 23.5-56.5T160-880h360q33 0 56.5 23.5T600-800v640q0 33-23.5 56.5T520-80H160Zm680-160v-480q25 0 42.5 17.5T900-660v360q0 25-17.5 42.5T840-240Zm-680 80h360v-640H160v640Zm0-640v640-640Z" />
												</svg>
												{getTotalCards(deck)}
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

						{decksQuery.data && decksQuery.data.length === 0 && (
							<div class="text-center py-8">
								<p class="text-muted-foreground mb-4">
									{t('home.no_decks')}
								</p>
								<button
									onClick={handleImportDeck}
									class="px-4 py-2 bg-primary text-primary-foreground rounded-md text-sm font-medium"
								>
									{t('home.add_your_first_deck')}
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
