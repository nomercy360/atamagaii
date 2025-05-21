import { createSignal, Show, createEffect, onMount, onCleanup, createResource } from 'solid-js'
import { apiRequest, Card, CardReviewResponse, Deck, DeckProgress } from '~/lib/api'
import { useParams, useNavigate } from '@solidjs/router'
import { useQuery } from '@tanstack/solid-query'
import AudioButton from '~/components/audio-button'
import { cn, hapticFeedback } from '~/lib/utils'
import TranscriptionText from '~/components/transcription-text'
import { audioService } from '~/lib/audio-service'
import AllDoneAnimation from '~/components/all-done-animation'
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

	return `space-y-2 absolute inset-0 w-full flex flex-col items-center justify-center p-4 ${rotationClass} ${opacityClass} ${pointerEventsClass} transition-all duration-200 transform-gpu backface-hidden`
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

	return `absolute inset-0 w-full flex flex-col items-center justify-center p-4 ${rotationClass} ${opacityClass} ${pointerEventsClass} transition-all duration-200 transform-gpu backface-hidden`
}

export default function Cards() {
	const params = useParams()
	const navigate = useNavigate()
	const [cardIndex, setCardIndex] = createSignal(0)
	const [flipped, setFlipped] = createSignal(false)
	const [isTransitioning, setIsTransitioning] = createSignal(false)
	const [feedbackType, setFeedbackType] = createSignal<'again' | 'good' | null>(null)
	const [showFeedback, setShowFeedback] = createSignal(false)
	const [timerStart, setTimerStart] = createSignal<number>(0)
	const [timeAccumulated, setTimeAccumulated] = createSignal<number>(0)
	const [timerPaused, setTimerPaused] = createSignal<boolean>(false)
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

	// Timer functions
	const startTimer = () => {
		setTimerStart(Date.now())
		setTimerPaused(false)
	}

	const pauseTimer = () => {
		if (timerStart() > 0 && !timerPaused()) {
			// Add the elapsed time since the timer started to the accumulated time
			setTimeAccumulated(timeAccumulated() + (Date.now() - timerStart()))
			setTimerPaused(true)
		}
	}

	const resumeTimer = () => {
		if (timerPaused()) {
			// Start the timer again from the current time
			setTimerStart(Date.now())
			setTimerPaused(false)
		}
	}

	const resetTimer = () => {
		setTimerStart(0)
		setTimeAccumulated(0)
		setTimerPaused(false)
	}

	const getCurrentTimeSpent = () => {
		if (timerStart() === 0) return timeAccumulated()

		// Return accumulated time + current running time
		return timeAccumulated() + (timerPaused() ? 0 : (Date.now() - timerStart()))
	}

	const [cardBuffer, setCardBuffer] = createSignal<Card[]>([])
	const [needMoreCards, setNeedMoreCards] = createSignal(true)

	const deckQuery = useQuery(() => ({
		queryKey: ['deck', params.deckId],
		queryFn: async () => {
			if (!params.deckId) return null
			const { data, error } = await apiRequest<Deck>(`/decks/${params.deckId}`)
			if (error) {
				console.error(`Failed to fetch deck ${params.deckId}:`, error)
				throw new Error(error)
			}

			if (data?.stats) {
				setDeckMetrics(data.stats)
			}

			return data
		},
	}))

	// Handle page visibility events to pause/resume timer
	onMount(() => {
		const handleVisibilityChange = () => {
			if (document.hidden) {
				// Tab is hidden, pause the timer
				pauseTimer()
			} else {
				// Tab is visible again, resume the timer if there's a current card
				// Timer should run on both front and back sides
				if (currentCard()) {
					resumeTimer()
				}
			}
		}

		// Start the timer when component mounts if there's a card
		if (currentCard()) {
			startTimer()
		}

		// Listen for visibility changes
		document.addEventListener('visibilitychange', handleVisibilityChange)

		// Clean up event listener
		onCleanup(() => {
			document.removeEventListener('visibilitychange', handleVisibilityChange)
		})
	})

	const [cards, { refetch: refetchCards }] = createResource<Card[], boolean>(
		() => needMoreCards() && cardBuffer().length === 0,
		async (shouldFetch) => {
			if (!shouldFetch) {
				return cardBuffer()
			}
			if (!params.deckId) return []

			const { data, error } = await apiRequest<Card[]>(`/cards/due?deck_id=${params.deckId}&limit=10`)

			if (error) {
				return []
			}

			if (data && data.length > 0) {
				setCardBuffer(prev => [...prev, ...data])
			} else {
				setNeedMoreCards(false)
			}

			return data || []
		},
	)

	const currentCard = () => {
		const buffer = cardBuffer()
		const idx = cardIndex()
		if (buffer.length === 0 || idx >= buffer.length) return null
		return buffer[idx]
	}

	// Start timer and preload audio when a new card is shown
	createEffect(() => {
		// We need to access cardIndex() for this effect to run when card changes
		const currentIdx = cardIndex()
		const hasCard = currentCard() !== null
		const card = currentCard()

		if (hasCard && !isTransitioning() && document.visibilityState === 'visible') {
			// Reset and start the timer when a new card is shown
			// Timer runs on both front and back sides
			resetTimer()
			startTimer()

			// Preload audio files for current card and next card if they exist
			if (card) {
				const audioFiles = []
				if (card.fields.audio_word) {
					audioFiles.push(card.fields.audio_word)
				}
				if (card.fields.audio_example) {
					audioFiles.push(card.fields.audio_example)
				}

				// Try to preload next card audio if available
				const nextIdx = currentIdx + 1
				const buffer = cardBuffer()
				if (buffer.length > nextIdx) {
					const nextCard = buffer[nextIdx]
					if (nextCard?.fields.audio_word) {
						audioFiles.push(nextCard.fields.audio_word)
					}
					if (nextCard?.fields.audio_example) {
						audioFiles.push(nextCard.fields.audio_example)
					}
				}

				if (audioFiles.length > 0) {
					audioService.preloadMultipleAudio(audioFiles)
				}
			}
		}
	})

	const stopAllAudio = () => {
		audioService.stopAll()
	}

	const playCardAudio = () => {
		const card = currentCard()
		if (card?.fields.audio_word) {
			try {
				// First make sure any existing audio is properly stopped
				audioService.stopAll()

				if (card?.fields.audio_example) {
					audioService.playSequence(card.fields.audio_word, card.fields.audio_example)
				} else {
					audioService.playAudio(card.fields.audio_word, 'word')
				}
			} catch (error) {
				console.error('Error playing card audio:', error)
			}
		}
	}

	const handleCardFlip = (e: MouseEvent) => {
		if (isTransitioning()) return

		// Only flip if card is not already flipped
		// This prevents the click from both flipping and rating
		if (!flipped()) {
			e.stopPropagation() // Prevent click from propagating to rating handlers
			hapticFeedback('impact', 'light')

			setFlipped(true)
			// Play audio immediately
			playCardAudio()
		}
	}

	const handleNextCard = () => {
		// Stop audio after a small delay to prevent AbortError
		setTimeout(() => {
			stopAllAudio()
		}, 50)

		setIsTransitioning(true)
		setTimeout(() => {
			setFlipped(false)
			setCardIndex(prevIndex => prevIndex + 1)

			// Reset timer for the next card
			resetTimer()

			setTimeout(() => {
				setIsTransitioning(false)

				// Start timer for the new card once transition is complete
				startTimer()
			}, 50)
		}, 200)
	}

	const handleReview = async (cardId: string, rating: number) => {
		// Explicitly pause the timer when an answer button is clicked
		// This captures the full time spent on both sides of the card
		pauseTimer()

		const finalTimeSpent = getCurrentTimeSpent()
		hapticFeedback('impact', 'light')

		// Show feedback indicator
		setFeedbackType(rating === 1 ? 'again' : 'good')
		setShowFeedback(true)

		// Hide feedback after a short delay (will still be visible during card transition)
		setTimeout(() => {
			setShowFeedback(false)
		}, 200)

		// Ensure we send at least a minimum time value
		// This prevents extremely short times if users immediately answer
		const timeToSend = Math.max(finalTimeSpent, 1000)

		console.log(`Total time spent on card (both sides): ${timeToSend}ms`)

		handleNextCard()

		void (async () => {
			try {
				const {
					data,
					error,
				} = await apiRequest<CardReviewResponse>(`/cards/${cardId}/review`, {
					method: 'POST',
					body: JSON.stringify({
						card_id: cardId,
						rating,
						time_spent_ms: timeToSend,
					}),
				})

				if (error) return

				if (data?.stats) {
					setDeckMetrics(data.stats)
				}

				if (data?.next_cards && data.next_cards.length > 0) {
					const current = currentCard()
					const filteredNewCards = current
						? data.next_cards.filter(c => c.id !== current.id)
						: data.next_cards

					if (filteredNewCards.length > 0) {
						setCardBuffer(prev => {
							const index = cardIndex()
							const before = prev.slice(0, index + 1)
							return [...before, ...filteredNewCards]
						})
					} else {
						setCardBuffer(prev => [...prev, ...data.next_cards])
					}
				}
			} catch (e) {
				console.error(`Exception during background review submission for card ${cardId}:`, e)
			}
		})()
	}

	const CardSkeleton = () => (
		<div class="w-full relative min-h-[500px] mt-6">
			<div class="absolute inset-0 w-full flex flex-col items-center justify-center p-4">
				<div class="w-3/4 h-12 bg-muted rounded-md animate-pulse mb-6"></div>
				<div class="w-full h-8 bg-muted rounded-md animate-pulse mb-2"></div>
				<div class="w-5/6 h-8 bg-muted rounded-md animate-pulse"></div>
			</div>
		</div>
	)

	return (
		<div class="container mx-auto px-2 py-6 max-w-md flex flex-col items-center min-h-screen">
			<Show when={showFeedback()}>
				<div
					class={`fixed top-20 left-1/2 z-50 flex items-center justify-center pointer-events-none transition-opacity duration-200  transform -translate-x-1/2 ${showFeedback() ? 'opacity-100' : 'opacity-0'}`}
				>
					<div
						class={`rounded-full p-2 flex items-center justify-center ${
							feedbackType() === 'again' ? 'bg-error/90 text-error-foreground' : 'bg-info/90 text-info-foreground'
						}`}
					>
						{feedbackType() === 'again' ? (
							<svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
								<path d="M6 16.5L18 7.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
								<path d="M6 7.5L18 16.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
							</svg>
						) : (
							<svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
								<path d="M5 13L9 17L19 7" stroke="currentColor" stroke-width="2" stroke-linecap="round"
											stroke-linejoin="round" />
							</svg>
						)}
					</div>
				</div>
			</Show>
			<Show when={cardBuffer().length > 0}>
				<ProgressBar
					completed={progressInfo().completed}
					total={progressInfo().total}
				/>
			</Show>

			<div class="w-full flex-grow flex flex-col items-center justify-start">
				<Show when={currentCard()}>
					<div class="w-full flex flex-col items-center">
						<div
							class={`text-center w-full cursor-pointer relative perspective transition-all min-h-[500px] ${isTransitioning() ? 'pointer-events-none' : ''}`}
							onClick={handleCardFlip}
						>
							<div class={getFrontFaceClasses(flipped(), isTransitioning())}>
								<TranscriptionText
									text={currentCard()?.fields.term || currentCard()?.fields.term || ''}
									class="text-5xl font-bold"
									language={currentCard()?.fields.language_code || 'jp'}
								/>
								<Show
									when={currentCard()?.fields.example_native || currentCard()?.fields.example_native}>
									<TranscriptionText
										text={currentCard()?.fields.example_native || currentCard()?.fields.example_native || ''}
										class="font-semibold text-xl text-secondary-foreground"
										language={currentCard()?.fields.language_code || 'jp'}
									/>
								</Show>
							</div>

							<div class={getBackFaceClasses(flipped(), isTransitioning())}>
								<div class="flex flex-col items-center justify-center">
									<Show
										when={!currentCard()?.fields.term_with_transcription}>
										 <span class="text-xl text-foreground font-bold">
												{currentCard()?.fields.transcription}
										 </span>
									</Show>
									{currentCard()?.fields.term_with_transcription ? (
										<TranscriptionText
											text={currentCard()?.fields.term_with_transcription || ''}
											class="font-bold text-5xl"
											rtClass="text-xl font-semibold"
											language={currentCard()?.fields.language_code || 'jp'}
										/>
									) : (
										<TranscriptionText
											text={currentCard()?.fields.term || currentCard()?.fields.term || ''}
											class="text-5xl font-bold"
											language={currentCard()?.fields.language_code || 'jp'}
										/>
									)}
								</div>
								<div
									class="text-center text-secondary-foreground text-xl font-normal mb-12 mt-3">
									{currentCard()?.fields.meaning_ru || currentCard()?.fields.meaning_en}
								</div>
								<Show when={currentCard()?.fields.example_native}>
									<div class="flex items-center justify-between mb-1">
										<p class="flex-grow">
											{currentCard()?.fields.example_with_transcription ? (
												<TranscriptionText
													text={currentCard()?.fields.example_with_transcription || ''}
													class="tracking-wider text-xl font-semibold"
													language={currentCard()?.fields.language_code || 'jp'}
													rtClass="font-semibold text-xs"
												/>
											) : (
												<TranscriptionText
													text={currentCard()?.fields.example_native || ''}
													class="text-xl font-semibold"
													language={currentCard()?.fields.language_code || 'jp'}
												/>
											)}
										</p>
									</div>
									<p class="text-center text-xl text-secondary-foreground">
										{currentCard()?.fields.example_ru || currentCard()?.fields.example_en}
									</p>
								</Show>
							</div>
						</div>
					</div>
				</Show>

				<Show when={cards.loading}>
					<CardSkeleton />
				</Show>

				<Show when={!cards.loading && !currentCard()}>
					<div class="w-full flex flex-col items-center justify-center h-[400px] px-4">
						<AllDoneAnimation width={100} height={100} class="mb-2" />
						<p class="text-xl font-medium text-center mb-4">All done for today!</p>
						<p class="text-muted-foreground mb-4 text-center">You've completed all your cards for this session.</p>
						<button
							onClick={() => {
								setNeedMoreCards(true)
								refetchCards()
							}}
							class="mb-4 px-4 py-2 bg-primary text-primary-foreground rounded-md"
						>
							Check Again
						</button>
						<button
							onClick={() => navigate('/')}
							class="mt-2 text-primary-foreground"
						>
							Back to decks
						</button>
					</div>
				</Show>
			</div>

			{/* Review buttons - show only when card is flipped */}
			<Show when={flipped() && currentCard() && !isTransitioning()}>
				{/* Click areas for rating cards by screen side click */}
				<div class="fixed inset-0 bottom-28 w-full pointer-events-auto" onClick={(e) => {
					// Get click position relative to window width
					const x = e.clientX
					const windowWidth = window.innerWidth

					// Determine if click was on left or right side
					if (x < windowWidth / 2) {
						// Left side - "Again"
						handleReview(currentCard()!.id, 1)
					} else {
						// Right side - "Good"
						handleReview(currentCard()!.id, 2)
					}
				}}>
				</div>

				<div class="h-32 fixed bottom-0 left-0 right-0 bg-transparent z-10">
					<div class="mx-auto px-4 py-4">
						<div class="flex flex-row items-center justify-center gap-8">
							<button
								onClick={() => handleReview(currentCard()!.id, 1)}
								class="rounded-[120px] justify-center flex flex-col items-center h-14 px-6 bg-error text-foreground transition-opacity font-bold text-sm"
							>
								<span>
									Again
								</span>
								<span class="text-[11px] leading-none font-semibold opacity-70">
									{currentCard()?.next_intervals.again}
								</span>
							</button>
							<div class="flex flex-row items-center justify-center gap-3">
								<Show when={currentCard()?.fields.audio_word}>
									<button
										class="rounded-full p-3.5 size-14 flex items-center justify-center bg-primary text-primary-foreground">
										<svg
											xmlns="http://www.w3.org/2000/svg"
											height="24px"
											viewBox="0 -960 960 960"
											width="24px"
											fill="currentColor">
											<path
												d="m603-202-34 97q-4 11-14 18t-22 7q-20 0-32.5-16.5T496-133l152-402q5-11 15-18t22-7h30q12 0 22 7t15 18l152 403q8 19-4 35.5T868-80q-13 0-22.5-7T831-106l-34-96H603ZM362-401 188-228q-11 11-27.5 11.5T132-228q-11-11-11-28t11-28l174-174q-35-35-63.5-80T190-640h84q20 39 40 68t48 58q33-33 68.5-92.5T484-720H80q-17 0-28.5-11.5T40-760q0-17 11.5-28.5T80-800h240v-40q0-17 11.5-28.5T360-880q17 0 28.5 11.5T400-840v40h240q17 0 28.5 11.5T680-760q0 17-11.5 28.5T640-720h-76q-21 72-63 148t-83 116l96 98-30 82-122-125Zm266 129h144l-72-204-72 204Z" />
										</svg>
									</button>
									<AudioButton
										size="lg"
										audioUrl={currentCard()?.fields.audio_word!}
										secondaryAudioUrl={currentCard()?.fields.audio_example}
									/>
								</Show>
							</div>
							<button
								onClick={() => handleReview(currentCard()!.id, 2)}
								class="rounded-[120px] justify-center flex flex-col items-center h-14 px-6 bg-success text-foreground  transition-opacity font-bold text-sm"
							>
								<span>
									Good
								</span>
								<span class="text-[11px] leading-none font-semibold opacity-70">
									{currentCard()?.next_intervals.good}
								</span>
							</button>
						</div>
					</div>
				</div>
			</Show>

			{/* Deck metrics - show only when card is not flipped */}
			<Show when={!flipped() && currentCard() && !isTransitioning() && deckQuery.data && !deckQuery.isPending}>
				<div class="h-32 fixed bottom-0 left-0 right-0 bg-background border-t border-border">
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
