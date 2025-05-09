import { createSignal, createResource, For, Show, createEffect, onMount, onCleanup } from 'solid-js'
import { apiRequest, Card, Deck, Stats } from '~/lib/api'
import { useParams, useNavigate } from '@solidjs/router'
import AudioButton from '~/components/audio-button'
import { store, setStore } from '~/store'

const getFrontFaceClasses = (isFlipped: boolean, isTrans: boolean) => {
	let opacityClass = '';
	if (isTrans) {
		opacityClass = 'opacity-0';
	} else {
		opacityClass = isFlipped ? 'opacity-0' : 'opacity-100';
	}

	const rotationClass = isFlipped ? 'rotate-y-180' : 'rotate-y-0';
	const pointerEventsClass = (isTrans || isFlipped) ? 'pointer-events-none' : '';

	return `absolute inset-0 w-full flex flex-col items-center justify-center p-4 ${rotationClass} ${opacityClass} ${pointerEventsClass} transition-all duration-300 transform-gpu backface-hidden`;
};

const getBackFaceClasses = (isFlipped: boolean, isTrans: boolean) => {
	let opacityClass = '';
	if (isTrans) {
		opacityClass = 'opacity-0';
	} else {
		opacityClass = isFlipped ? 'opacity-100' : 'opacity-0';
	}

	const rotationClass = isFlipped ? 'rotate-y-0' : 'rotate-y-180';
	const pointerEventsClass = (isTrans || !isFlipped) ? 'pointer-events-none' : '';

	return `absolute inset-0 w-full flex flex-col items-center justify-center p-4 ${rotationClass} ${opacityClass} ${pointerEventsClass} transition-all duration-300 transform-gpu backface-hidden`;
};

export default function Cards() {
	const params = useParams()
	const navigate = useNavigate()
	const [cardIndex, setCardIndex] = createSignal(0)
	const [flipped, setFlipped] = createSignal(false)
	const [isTransitioning, setIsTransitioning] = createSignal(false)
	const [timeSpentMs, setTimeSpentMs] = createSignal(0)
	const [startTime, setStartTime] = createSignal<number | null>(null)
	const [isTimerActive, setIsTimerActive] = createSignal(false)
	const [deckMetrics, setDeckMetrics] = createSignal<{
		newCards: number;
		learningCards: number;
		reviewCards: number;
	}>({ newCards: 0, learningCards: 0, reviewCards: 0 })

	const [deck, { refetch: refetchDeck }] = createResource<Deck | null>(
		async () => {
			if (!params.deckId) return null
			const { data, error } = await apiRequest<Deck>(`/decks/${params.deckId}`)
			if (error) {
				console.error(`Failed to fetch deck ${params.deckId}:`, error)
				return null
			}

			if (data) {
				setDeckMetrics({
					newCards: data.new_cards || 0,
					learningCards: data.learning_cards || 0,
					reviewCards: data.review_cards || 0
				})
			}

			return data
		},
	)

	const [cards] = createResource<Card[]>(
		async () => {
			if (!params.deckId) return []
			const { data, error } = await apiRequest<Card[]>(`/cards/due?deck_id=${params.deckId}`)
			if (error) {
				console.error(`Failed to fetch cards for deck ${params.deckId}:`, error)
				return []
			}

			// Reset timer when new cards are loaded
			setTimeout(() => {
				if (data && data.length > 0) {
					resetTimer();
				}
			}, 0);

			return data || []
		},
	)

	const currentCard = () => {
		const cardList = cards() || []
		if (cardList.length === 0) return null
		return cardList[cardIndex() % cardList.length]
	}

	// Manages the visibility change and page lifecycle events
	const handleVisibilityChange = () => {
		if (document.visibilityState === 'hidden') {
			// When tab is hidden or browser minimized, pause the timer
			pauseTimer();
		} else if (document.visibilityState === 'visible' && currentCard()) {
			// When tab becomes visible again and a card is displayed, resume the timer
			startTimer();
		}
	};

	// Start timing for a card
	const startTimer = () => {
		if (!isTimerActive()) {
			setStartTime(Date.now());
			setIsTimerActive(true);
		}
	};

	// Pause the timer and accumulate elapsed time
	const pauseTimer = () => {
		if (isTimerActive() && startTime() !== null) {
			const elapsedMs = Date.now() - startTime()!;
			setTimeSpentMs(prev => prev + elapsedMs);
			setStartTime(null);
			setIsTimerActive(false);
		}
	};

	// Reset the timer for a new card
	const resetTimer = () => {
		setTimeSpentMs(0);
		setStartTime(Date.now());
		setIsTimerActive(true);
	};

	// Get the current total time spent (including active timing)
	const getCurrentTimeSpent = () => {
		let total = timeSpentMs();
		if (isTimerActive() && startTime() !== null) {
			total += Date.now() - startTime()!;
		}
		return total;
	};

	// Setup visibility and lifecycle event listeners
	onMount(() => {
		// Start timing when component mounts if a card is available
		if (currentCard()) {
			resetTimer();
		}

		// Add event listeners for visibility changes
		document.addEventListener('visibilitychange', handleVisibilityChange);

		// Add event listeners for page unload
		window.addEventListener('beforeunload', pauseTimer);
	});

	onCleanup(() => {
		// Clean up event listeners
		document.removeEventListener('visibilitychange', handleVisibilityChange);
		window.removeEventListener('beforeunload', pauseTimer);
		pauseTimer();
	});

	const handleCardFlip = () => {
		if (isTransitioning()) return;
		setFlipped(!flipped());
	}

	const handleNextCard = () => {
		pauseTimer();
		setIsTransitioning(true);
		setTimeout(() => {
			setFlipped(false);
			setCardIndex(prevIndex => prevIndex + 1);
			setTimeout(() => {
				setIsTransitioning(false);
				resetTimer();
			}, 50);
		}, 300);
	}

	const handleReview = async (cardId: string, rating: number) => {
		pauseTimer();
		const finalTimeSpent = getCurrentTimeSpent();

		const { error } = await apiRequest(`/cards/${cardId}/review`, {
			method: 'POST',
			body: JSON.stringify({
				card_id: cardId,
				rating,
				time_spent_ms: finalTimeSpent,
			}),
		});
		if (error) {
			console.error('Failed to submit review:', error);
			return;
		}

		// Update deck metrics locally
		const metrics = deckMetrics();
		const card = currentCard();

		// Card is moving from "new" to "learning" state
		if (!card?.interval) {
			setDeckMetrics({
				newCards: Math.max(0, metrics.newCards - 1),
				learningCards: metrics.learningCards + 1,
				reviewCards: metrics.reviewCards
			});
		}
		// Update based on rating
		else {
			// Rating 1 (Again) - card goes to learning
			if (rating === 1) {
				// If card was in review, move to learning
				if (card.interval && card.interval > 1) {
					setDeckMetrics({
						newCards: metrics.newCards,
						learningCards: metrics.learningCards + 1,
						reviewCards: Math.max(0, metrics.reviewCards - 1)
					});
				}
			}
			// Rating 2-4 - card is eventually removed from daily count
			else {
				// For simplicity, just decrement the appropriate counter
				if (card.interval && card.interval <= 1) {
					// Card was in learning
					setDeckMetrics({
						newCards: metrics.newCards,
						learningCards: Math.max(0, metrics.learningCards - 1),
						reviewCards: metrics.reviewCards
					});
				} else {
					// Card was in review
					setDeckMetrics({
						newCards: metrics.newCards,
						learningCards: metrics.learningCards,
						reviewCards: Math.max(0, metrics.reviewCards - 1)
					});
				}
			}
		}

		// Update global stats by decrementing due count
		if (store.stats && typeof store.stats.due_cards === 'number') {
			const newDueCards = Math.max(0, store.stats.due_cards - 1);
			setStore('stats', {
				...store.stats,
				due_cards: newDueCards
			});
		}

		handleNextCard();
	}

	return (
		<div class="container mx-auto px-2 py-6 max-w-md flex flex-col items-center min-h-screen">
			{/* Deck metrics */}
			<Show when={deck() && !deck.loading}>
				<div class="w-full mb-4">
					<h2 class="text-lg font-semibold mb-1">{deck()?.name}</h2>
					<div class="flex gap-3 text-sm">
						<Show when={deckMetrics().newCards > 0}>
							<span class="text-blue-500">{deckMetrics().newCards} new</span>
						</Show>
						<Show when={deckMetrics().learningCards > 0}>
							<span class="text-yellow-500">{deckMetrics().learningCards} learning</span>
						</Show>
						<Show when={deckMetrics().reviewCards > 0}>
							<span class="text-green-500">{deckMetrics().reviewCards} review</span>
						</Show>
					</div>
				</div>
			</Show>

			<div class="w-full flex-grow flex flex-col items-center justify-start">
				<Show when={currentCard()}>
					<div class="w-full flex flex-col items-center">
						<div
							class={`w-full bg-card rounded-xl shadow-lg cursor-pointer relative perspective transition-all min-h-96 ${isTransitioning() ? 'pointer-events-none' : ''}`}
							onClick={handleCardFlip}
						>
							<div class={getFrontFaceClasses(flipped(), isTransitioning())}>
								<div class="text-4xl font-bold mb-4 font-jp">
									{currentCard()?.front.kanji ? currentCard()?.front.kanji : currentCard()?.front.kana}
								</div>
								<Show
									when={currentCard()?.back?.examples && (currentCard() ? currentCard()!.back.examples.length : 0) > 0}>
									<div class="text-sm bg-muted rounded-md p-3 mb-2 max-w-full">
										<p class="mb-1">
											<For each={currentCard()?.back.examples[0]?.sentence || []}>
												{(fragment) => (
													<span
														class={`text-2xl font-jp ${fragment.fragment === currentCard()?.front.kanji ?
															'text-blue-400 border-b-2 border-blue-400 border-primary' : ''}`}
													>
														{fragment.fragment}
													</span>
												)}
											</For>
										</p>
									</div>
								</Show>
							</div>

							<div class={getBackFaceClasses(flipped(), isTransitioning())}>
								<div class="text-4xl font-bold mb-6 flex flex-col items-center">
									<div class="flex items-center gap-2">
										{currentCard()?.front.kanji}
										<Show when={currentCard()?.back.audio_url}>
											<AudioButton
												audioUrl={currentCard()?.back.audio_url || ''}
												size="sm"
												label="Play word pronunciation"
											/>
										</Show>
									</div>
									<Show when={currentCard()?.front.kana}>
                                       <span class="text-lg font-jp text-muted-foreground">
                                        {currentCard()?.front.kana}
                                       </span>
									</Show>
								</div>
								<div class="text-center text-2xl font-medium mb-8">{currentCard()?.back.translation}</div>
								<div class="text-sm space-y-2 w-full">
									<For each={currentCard()?.back.examples}>
										{(example) => (
											<div class="bg-muted rounded-md p-2">
												<div class="flex items-start justify-between mb-1">
													<p class="flex-grow text-2xl font-jp">
														<For each={example.sentence}>
															{(fragment) => (
																<span>
																	{fragment.furigana ? (
																		<ruby>
																			{fragment.fragment}
																			<rt class="text-xs text-primary">{fragment.furigana}</rt>
																		</ruby>
																	) : (
																		fragment.fragment
																	)}
																</span>
															)}
														</For>
													</p>
													<Show when={example.audio_url}>
														<AudioButton
															audioUrl={example.audio_url || ''}
															size="sm"
															label="Play example audio"
														/>
													</Show>
												</div>
												<p class="text-xs text-muted-foreground">{example.translation}</p>
											</div>
										)}
									</For>
								</div>
							</div>
						</div>
					</div>
				</Show>

				<Show when={cards.loading}>
					<div class="w-full flex flex-col items-center justify-center h-[300px]">
						<p class="text-muted-foreground">Loading cards...</p>
					</div>
				</Show>

				<Show when={!cards.loading && (!cards() || cards()!.length === 0)}>
					<div class="w-full flex flex-col items-center justify-center h-[300px]">
						<p class="text-muted-foreground">No cards found in this deck.</p>
						<button
							onClick={() => navigate('/')}
							class="mt-4 text-primary"
						>
							Back to decks
						</button>
					</div>
				</Show>
			</div>

			<Show when={flipped() && currentCard() && !isTransitioning()}>
				<div class="fixed bottom-0 left-0 right-0 bg-background border-t border-border pb-8">
					<div class="mx-auto px-4 py-4">
						<div class="grid grid-cols-4 gap-2">
							<button
								onClick={() => handleReview(currentCard()!.id, 1)}
								class="py-3 px-4 bg-error text-error-foreground rounded-md transition-opacity font-medium"
							>
								Again
							</button>
							<button
								onClick={() => handleReview(currentCard()!.id, 2)}
								class="py-3 px-4 bg-warning text-warning-foreground rounded-md transition-opacity font-medium"
							>
								Hard
							</button>
							<button
								onClick={() => handleReview(currentCard()!.id, 3)}
								class="py-3 px-4 bg-info text-info-foreground rounded-md transition-opacity font-medium"
							>
								Good
							</button>
							<button
								onClick={() => handleReview(currentCard()!.id, 4)}
								class="py-3 px-4 bg-success text-success-foreground rounded-md transition-opacity font-medium"
							>
								Easy
							</button>
						</div>
					</div>
				</div>
			</Show>
		</div>
	)
}
