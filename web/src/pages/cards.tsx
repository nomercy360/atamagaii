import { createSignal, createResource, For, Show, createEffect, onMount, onCleanup } from 'solid-js'
import { apiRequest, Card, CardReviewRequest, CardReviewResponse, Deck, Stats } from '~/lib/api'
import { useParams, useNavigate } from '@solidjs/router'
import AudioButton from '~/components/audio-button'

const PREFETCH_BUFFER_THRESHOLD = 2

const getFrontFaceClasses = (isFlipped: boolean, isTrans: boolean) => {
	let opacityClass = ''
	if (isTrans) {
		opacityClass = 'opacity-0'
	} else {
		opacityClass = isFlipped ? 'opacity-0' : 'opacity-100'
	}

	const rotationClass = isFlipped ? 'rotate-y-180' : 'rotate-y-0'
	const pointerEventsClass = (isTrans || isFlipped) ? 'pointer-events-none' : ''

	return `absolute inset-0 w-full flex flex-col items-center justify-center p-4 ${rotationClass} ${opacityClass} ${pointerEventsClass} transition-all duration-300 transform-gpu backface-hidden`
}

const getBackFaceClasses = (isFlipped: boolean, isTrans: boolean) => {
	let opacityClass = ''
	if (isTrans) {
		opacityClass = 'opacity-0'
	} else {
		opacityClass = isFlipped ? 'opacity-100' : 'opacity-0'
	}

	const rotationClass = isFlipped ? 'rotate-y-0' : 'rotate-y-180'
	const pointerEventsClass = (isTrans || !isFlipped) ? 'pointer-events-none' : ''

	return `absolute inset-0 w-full flex flex-col items-center justify-center p-4 ${rotationClass} ${opacityClass} ${pointerEventsClass} transition-all duration-300 transform-gpu backface-hidden`
}

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

	const [cardBuffer, setCardBuffer] = createSignal<Card[]>([])
	const [needMoreCards, setNeedMoreCards] = createSignal(true)
	const [processedCardIds, setProcessedCardIds] = createSignal<Set<string>>(new Set())
	const [isFetchingMore, setIsFetchingMore] = createSignal(false)

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
					reviewCards: data.review_cards || 0,
				})
			}

			return data
		},
	)

	const fetchCards = async (): Promise<Card[]> => {
		if (!params.deckId || isFetchingMore()) {
			console.log('Fetch skipped: No deckId or already fetching.')
			return []
		}

		console.log('Attempting to fetch cards...')
		setIsFetchingMore(true)

		try {
			const { data, error } = await apiRequest<Card[]>(`/cards/due?deck_id=${params.deckId}&limit=3`)
			if (error) {
				console.error(`Failed to fetch cards for deck ${params.deckId}:`, error)
				setIsFetchingMore(false)
				return []
			}

			const processed = processedCardIds()
			const newCards = (data || []).filter(card => !processed.has(card.id))

			if (newCards.length > 0) {
				setCardBuffer(prev => [...prev, ...newCards])
				console.log(`Fetched and added ${newCards.length} new cards to buffer.`)
			} else {
				console.log('Fetched cards, but no *new* cards to add (either all processed or API returned processed cards).')
			}

			if (!data || data.length === 0) {
				console.log('API returned no cards. Setting needMoreCards to false.')
				setNeedMoreCards(false)
			}

			setIsFetchingMore(false)
			return newCards
		} catch (e) {
			console.error('Exception during fetchCards:', e)
			setIsFetchingMore(false)
			return []
		}
	}

	const [cards] = createResource<Card[], boolean>(
		() => needMoreCards() && cardBuffer().length === 0,
		async (shouldFetch) => {
			if (!shouldFetch) {
				return cardBuffer()
			}
			console.log('Resource: Triggering initial fetchCards.')
			await fetchCards()
			return cardBuffer()
		},
	)

	// Effect to maintain our buffer of at least 2 cards when possible
	createEffect(() => {
		const buffer = cardBuffer()
		const currentIdx = cardIndex()
		const remaining = buffer.length - currentIdx

		// Log buffer state for debugging
		console.log(`Card buffer state: ${remaining} remaining (${currentIdx}/${buffer.length})`)

		// If we have 1 or fewer cards left in our buffer, fetch more
		if (remaining <= 1 && needMoreCards()) {
			console.log('Fetching more cards for buffer')
			fetchCards()
		}
	})

	const currentCard = () => {
		const buffer = cardBuffer()
		const idx = cardIndex()
		if (buffer.length === 0 || idx >= buffer.length) return null
		return buffer[idx]
	}

	const handleVisibilityChange = () => {
		if (document.visibilityState === 'hidden') {
			pauseTimer()
		} else if (document.visibilityState === 'visible' && currentCard()) {
			startTimer()
		}
	}

	const startTimer = () => {
		if (!isTimerActive()) {
			setStartTime(Date.now())
			setIsTimerActive(true)
		}
	}

	const pauseTimer = () => {
		if (isTimerActive() && startTime() !== null) {
			const now = Date.now()
			const elapsedMs = now - startTime()!
			const newTotal = timeSpentMs() + elapsedMs
			setTimeSpentMs(newTotal)
			setStartTime(null)
			setIsTimerActive(false)
		}
	}

	const resetTimer = () => {
		setTimeSpentMs(0)
		setStartTime(Date.now())
		setIsTimerActive(true)
	}

	const getCurrentTimeSpent = () => {
		let total = timeSpentMs()
		if (isTimerActive() && startTime() !== null) {
			const now = Date.now()
			const current = now - startTime()!
			total += current
			console.log('Current session time:', current, 'ms, total with previous:', total, 'ms')
		} else {
			console.log('Timer inactive, returning accumulated time:', total, 'ms')
		}
		return total
	}

	onMount(() => {
		createEffect(() => {
			if (currentCard() && startTime() === null) {
				console.log('onMount/Effect: First card available, resetting timer.')
				resetTimer()
			}
		})

		document.addEventListener('visibilitychange', handleVisibilityChange)
		window.addEventListener('beforeunload', pauseTimer)
	})

	onCleanup(() => {
		document.removeEventListener('visibilitychange', handleVisibilityChange)
		window.removeEventListener('beforeunload', pauseTimer)
		pauseTimer()
	})

	const playCardAudio = () => {
		const card = currentCard()
		if (card?.back.audio_url) {
			const audio = new Audio(card.back.audio_url)
			audio.play().catch(error => {
				console.error('Error playing audio:', error)
			})
		}
	}

	const handleCardFlip = () => {
		if (isTransitioning()) return
		// Only allow flipping from front to back, not back to front
		if (!flipped()) {
			setFlipped(true)
			// We'll play the audio after a short delay to ensure the flip animation has started
			setTimeout(() => playCardAudio(), 150)
		}
	}

	const handleNextCard = () => {
		setIsTransitioning(true)
		setTimeout(() => {
			setFlipped(false)
			setCardIndex(prevIndex => prevIndex + 1)
			setTimeout(() => {
				setIsTransitioning(false)
				resetTimer()
			}, 50)
		}, 300)
	}

	const handleReview = async (cardId: string, rating: number) => {
		pauseTimer()
		const finalTimeSpent = getCurrentTimeSpent()
		console.log('Sending review with time_spent_ms:', finalTimeSpent)

		const timeToSend = finalTimeSpent > 0 ? finalTimeSpent : 1000

		const { data, error } = await apiRequest<CardReviewResponse>(`/cards/${cardId}/review`, {
			method: 'POST',
			body: JSON.stringify({
				card_id: cardId,
				rating,
				time_spent_ms: timeToSend,
			}),
		})

		if (error) {
			console.error('Failed to submit review:', error)
			return
		}

		if (data?.stats) {
			setDeckMetrics({
				newCards: data.stats.new_cards || 0,
				learningCards: data.stats.learning_cards || 0,
				reviewCards: data.stats.review_cards || 0,
			})
		}

		setProcessedCardIds(prev => {
			const updated = new Set(prev)
			updated.add(cardId)
			return updated
		})

		handleNextCard()
	}

	return (
		<div class="container mx-auto px-2 py-6 max-w-md flex flex-col items-center min-h-screen">
			{/* Deck name */}
			<Show when={deck() && !deck.loading}>
				<div class="w-full mb-4">
					<h2 class="text-lg font-semibold mb-1">{deck()?.name}</h2>
				</div>
			</Show>

			<div class="w-full flex-grow flex flex-col items-center justify-start">
				<Show when={currentCard()}>
					<div class="w-full flex flex-col items-center">
						<div
							class={`w-full cursor-pointer relative perspective transition-all min-h-96 ${isTransitioning() ? 'pointer-events-none' : ''}`}
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

				<Show when={!cards.loading && !currentCard()}>
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

			{/* Review buttons - show only when card is flipped */}
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

			{/* Deck metrics - show only when card is not flipped */}
			<Show when={!flipped() && currentCard() && !isTransitioning() && deck() && !deck.loading}>
				<div class="fixed bottom-0 left-0 right-0 bg-background border-t border-border pb-8">
					<div class="mx-auto px-4 py-4">
						<div class="flex justify-center gap-3">
							<Show when={deckMetrics().newCards > 0}>
								<span class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
									{deckMetrics().newCards} new
								</span>
							</Show>
							<Show when={deckMetrics().learningCards > 0}>
								<span class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800">
									{deckMetrics().learningCards} learning
								</span>
							</Show>
							<Show when={deckMetrics().reviewCards > 0}>
								<span class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-green-100 text-green-800">
									{deckMetrics().reviewCards} review
								</span>
							</Show>
						</div>
					</div>
				</div>
			</Show>
		</div>
	)
}
