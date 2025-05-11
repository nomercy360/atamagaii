import { createSignal, createResource, Show, createEffect, onMount, onCleanup } from 'solid-js'
import { apiRequest, Card, CardReviewResponse, Deck, DeckProgress } from '~/lib/api'
import { useParams, useNavigate } from '@solidjs/router'
import AudioButton from '~/components/audio-button'
import { hapticFeedback } from '~/lib/utils'
import TranscriptionText from '~/components/transcription-text'
import { audioService } from '~/lib/audio-service'
import ProgressBar from '~/components/progress-bar'

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
	const [deckMetrics, setDeckMetrics] = createSignal<DeckProgress>({
		new_cards: 0,
		learning_cards: 0,
		review_cards: 0,
		completed_today_cards: 0,
	})

	const progressInfo = () => {
		const metrics = deckMetrics()
		const total = metrics.new_cards + metrics.learning_cards + metrics.review_cards + metrics.completed_today_cards
		return {
			completed: metrics.completed_today_cards,
			total: total,
			percentage: total > 0 ? Math.round((metrics.completed_today_cards / total) * 100) : 0,
		}
	}

	const [cardBuffer, setCardBuffer] = createSignal<Card[]>([])
	const [needMoreCards, setNeedMoreCards] = createSignal(true)
	const [isFetchingMore, setIsFetchingMore] = createSignal(false)

	const [deck, { refetch: refetchDeck }] = createResource<Deck | null>(
		async () => {
			if (!params.deckId) return null
			const { data, error } = await apiRequest<Deck>(`/decks/${params.deckId}`)
			if (error) {
				console.error(`Failed to fetch deck ${params.deckId}:`, error)
				return null
			}

			if (data?.stats) {
				setDeckMetrics(data.stats)
			}

			return data
		},
	)

	const [fetchFailureCount, setFetchFailureCount] = createSignal<number>(0)
	const MAX_FETCH_FAILURES = 1

	const fetchCards = async (): Promise<Card[]> => {
		if (!params.deckId || isFetchingMore()) {
			console.log('Fetch skipped: No deckId or already fetching.')
			return []
		}

		if (fetchFailureCount() >= MAX_FETCH_FAILURES) {
			console.log(`Skipping fetch - reached max failure count (${MAX_FETCH_FAILURES})`)
			setNeedMoreCards(false)
			return []
		}

		console.log('Attempting to fetch cards...')
		setIsFetchingMore(true)

		try {
			const { data, error } = await apiRequest<Card[]>(`/cards/due?deck_id=${params.deckId}&limit=3`)
			if (error) {
				console.error(`Failed to fetch cards for deck ${params.deckId}:`, error)
				setIsFetchingMore(false)

				const newCount = fetchFailureCount() + 1
				setFetchFailureCount(newCount)

				if (newCount >= MAX_FETCH_FAILURES) {
					console.log(`Max failure count reached (${MAX_FETCH_FAILURES}), stopping automatic retries`)
					setNeedMoreCards(false) // Prevent further fetch attempts
				}

				return []
			}

			setFetchFailureCount(0)

			if (!data || data.length === 0) {
				console.log('API returned no cards. Setting needMoreCards to false.')
				setNeedMoreCards(false)
			}

			const newCards = data || []

			if (newCards.length > 1 && currentCard()) {
				const currentCardId = currentCard()!.id
				const currentCardIndex = newCards.findIndex(card => card.id === currentCardId)
				if (currentCardIndex !== -1) {
					newCards.splice(currentCardIndex, 1)
					console.log(`Removed current card from new cards: ${currentCardId}`)
				}
			}

			if (newCards.length > 0) {
				setCardBuffer(prev => [...prev, ...newCards])
				console.log(`Fetched and added ${newCards.length} new cards to buffer.`)
			} else {
				console.log('Fetched cards, but no *new* cards to add (either all processed or API returned processed cards).')
			}

			setIsFetchingMore(false)
			return newCards
		} catch (e) {
			console.error('Exception during fetchCards:', e)
			setIsFetchingMore(false)

			const newCount = fetchFailureCount() + 1
			setFetchFailureCount(newCount)

			if (newCount >= MAX_FETCH_FAILURES) {
				console.log(`Max failure count reached (${MAX_FETCH_FAILURES}), stopping automatic retries`)
				setNeedMoreCards(false)
			}

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

	const stopAllAudio = () => {
		audioService.stopAll()
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
			// console.log('Current session time:', current, 'ms, total with previous:', total, 'ms')
		} else {
			// console.log('Timer inactive, returning accumulated time:', total, 'ms')
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
		if (card?.fields.audio_word) {
			if (card?.fields.audio_example) {
				audioService.playSequence(card.fields.audio_word, card.fields.audio_example, 300)
			} else {
				audioService.playAudio(card.fields.audio_word, 'word')
			}
		}
	}

	const handleCardFlip = () => {
		if (isTransitioning()) return
		// Only allow flipping from front to back, not back to front
		if (!flipped()) {
			// Provide haptic feedback when flipping the card
			hapticFeedback('impact', 'light')

			setFlipped(true)
			// We'll play the audio after a short delay to ensure the flip animation has started
			setTimeout(() => playCardAudio(), 150)
		}
	}

	const handleNextCard = () => {
		stopAllAudio()

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
		hapticFeedback('impact', 'light')

		const timeToSend = finalTimeSpent > 0 ? finalTimeSpent : 1000

		handleNextCard()

		;(async () => {
			try {
				// Add small delay for testing purposes
				const artificialDelay = 1000 // 1 second delay
				console.log(`Adding ${artificialDelay}ms artificial delay for testing...`)

				// Sleep function for the artificial delay
				const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms))
				await sleep(artificialDelay)

				const { data, error } = await apiRequest<CardReviewResponse>(`/cards/${cardId}/review`, {
					method: 'POST',
					body: JSON.stringify({
						card_id: cardId,
						rating,
						time_spent_ms: timeToSend,
					}),
				})

				if (error) {
					console.error(`Failed to submit review for card ${cardId}:`, error)
					// Optionally: Implement a retry queue or notify user of sync failure
					return
				}

				if (data?.stats) {
					console.log('Review submitted successfully, updating stats:', data.stats)
					// Replace our optimistic stats with the real ones from the server
					setDeckMetrics(data.stats)
				} else {
					console.warn(`Review API call for card ${cardId} succeeded but returned no stats data.`)
				}
			} catch (e) {
				console.error(`Exception during background review submission for card ${cardId}:`, e)
				// Optionally: Implement a retry queue or notify user of sync failure
			}
		})()
	}

	return (
		<div class="container mx-auto px-2 py-6 max-w-md flex flex-col items-center min-h-screen">
			{/* Deck name and progress */}
			<Show when={deck() && !deck.loading}>
				<div class="w-full mb-4">
					<h2 class="text-lg font-semibold mb-1">{deck()?.name}</h2>
					<ProgressBar
						completed={progressInfo().completed}
						total={progressInfo().total}
						showPercentage={true}
					/>
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
								<div class="text-5xl font-semibold mb-4">
									<TranscriptionText
										text={currentCard()?.fields.term || currentCard()?.fields.term || ''}
										textSize="5xl"
										language={currentCard()?.fields.language_code || 'ja'}
										transcriptionType={currentCard()?.fields.transcription_type || 'furigana'}
									/>
								</div>
								<Show
									when={currentCard()?.fields.example_native || currentCard()?.fields.example_native}>
									<div class="text-2xl p-3 mb-2 max-w-full">
										<TranscriptionText
											text={currentCard()?.fields.example_native || currentCard()?.fields.example_native || ''}
											textSize="2xl"
											language={currentCard()?.fields.language_code || 'ja'}
											transcriptionType={currentCard()?.fields.transcription_type || 'furigana'}
										/>
									</div>
								</Show>
							</div>

							<div class={getBackFaceClasses(flipped(), isTransitioning())}>
								<div class="text-5xl font-semibold mb-6 flex flex-col items-center">
									<div class="flex items-center gap-2 pl-8">
										{currentCard()?.fields.term_with_transcription || currentCard()?.fields.term_with_transcription ? (
											<TranscriptionText
												text={currentCard()?.fields.term_with_transcription || currentCard()?.fields.term_with_transcription!}
												textSize="5xl"
												language={currentCard()?.fields.language_code || 'ja'}
												transcriptionType={currentCard()?.fields.transcription_type || 'furigana'}
											/>
										) : (
											<TranscriptionText
												text={currentCard()?.fields.term || currentCard()?.fields.term || ''}
												textSize="5xl"
												language={currentCard()?.fields.language_code || 'ja'}
												transcriptionType={currentCard()?.fields.transcription_type || 'furigana'}
											/>
										)}
										<Show when={currentCard()?.fields.audio_word}>
											<AudioButton
												audioUrl={currentCard()?.fields.audio_word || ''}
												size="sm"
												label="Play word pronunciation"
												type="word"
											/>
										</Show>
									</div>
									<Show
										when={(currentCard()?.fields.transcription || currentCard()?.fields.transcription) && !(currentCard()?.fields.term_with_transcription || currentCard()?.fields.term_with_transcription)}>
										 <span class="text-lg text-muted-foreground">
												{currentCard()?.fields.transcription || currentCard()?.fields.transcription}
										 </span>
									</Show>
								</div>
								<div class="text-center text-2xl font-medium mb-8">{currentCard()?.fields.meaning_ru}</div>
								<div class="text-sm space-y-2 w-full">
									<div class="bg-muted rounded-md p-2">
										<div class="flex items-start justify-between mb-1">
											<p class="flex-grow">
												{currentCard()?.fields.example_with_transcription || currentCard()?.fields.example_with_transcription ? (
													<TranscriptionText
														text={currentCard()?.fields.example_with_transcription || currentCard()?.fields.example_with_transcription!}
														textSize="2xl"
														language={currentCard()?.fields.language_code || 'ja'}
														transcriptionType={currentCard()?.fields.transcription_type || 'furigana'}
													/>
												) : (
													<TranscriptionText
														text={currentCard()?.fields.example_native || currentCard()?.fields.example_native || ''}
														textSize="2xl"
														language={currentCard()?.fields.language_code || 'ja'}
														transcriptionType={currentCard()?.fields.transcription_type || 'furigana'}
													/>
												)}
											</p>
											<Show when={currentCard()?.fields.audio_example}>
												<AudioButton
													audioUrl={currentCard()?.fields.audio_example || ''}
													size="sm"
													label="Play example audio"
													type="example"
												/>
											</Show>
										</div>
										<p class="text-xs text-muted-foreground">{currentCard()?.fields.example_ru}</p>
									</div>
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
						<Show when={fetchFailureCount() >= MAX_FETCH_FAILURES}>
							<p class="text-muted-foreground mb-2">Failed to load cards. Please try again later.</p>
							<button
								onClick={() => {
									setFetchFailureCount(0)
									setNeedMoreCards(true)
									fetchCards()
								}}
								class="mb-4 px-4 py-2 bg-primary text-primary-foreground rounded-md"
							>
								Retry
							</button>
						</Show>
						<Show when={fetchFailureCount() < MAX_FETCH_FAILURES}>
							<p class="text-muted-foreground">No cards found in this deck.</p>
						</Show>
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
						<div class="grid grid-cols-2 gap-4">
							<button
								onClick={() => handleReview(currentCard()!.id, 1)}
								class="py-3 px-4 bg-error text-error-foreground rounded-md transition-opacity font-medium text-lg"
							>
								Again
							</button>
							<button
								onClick={() => handleReview(currentCard()!.id, 2)}
								class="py-3 px-4 bg-info text-info-foreground rounded-md transition-opacity font-medium text-lg"
							>
								Good
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
							<Show when={deckMetrics().new_cards > 0}>
								<span
									class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
									{deckMetrics().new_cards} new
								</span>
							</Show>
							<Show when={deckMetrics().learning_cards > 0}>
								<span
									class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800">
									{deckMetrics().learning_cards} learning
								</span>
							</Show>
							<Show when={deckMetrics().review_cards > 0}>
								<span
									class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-green-100 text-green-800">
									{deckMetrics().review_cards} review
								</span>
							</Show>
						</div>
					</div>
				</div>
			</Show>
		</div>
	)
}
