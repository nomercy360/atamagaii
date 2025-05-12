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

	return `absolute inset-0 w-full flex flex-col items-center justify-center p-4 ${rotationClass} ${opacityClass} ${pointerEventsClass} transition-all duration-200 transform-gpu backface-hidden`
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
	const [timeSpentMs, setTimeSpentMs] = createSignal(0)
	const [startTime, setStartTime] = createSignal<number | null>(null)
	const [isTimerActive, setIsTimerActive] = createSignal(false)
	const [settingsOpen, setSettingsOpen] = createSignal(false)
	const [feedbackType, setFeedbackType] = createSignal<'again' | 'good' | null>(null)
	const [showFeedback, setShowFeedback] = createSignal(false)
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

	createEffect(() => {
		const buffer = cardBuffer()
		const currentIdx = cardIndex()
		const remaining = buffer.length - currentIdx

		console.log(`Card buffer state: ${remaining} remaining (${currentIdx}/${buffer.length})`)
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
		document.addEventListener('click', handleClickOutside)
	})

	onCleanup(() => {
		document.removeEventListener('visibilitychange', handleVisibilityChange)
		window.removeEventListener('beforeunload', pauseTimer)
		document.removeEventListener('click', handleClickOutside)
		pauseTimer()
	})

	const playCardAudio = () => {
		const card = currentCard()
		if (card?.fields.audio_word) {
			if (card?.fields.audio_example) {
				audioService.playSequence(card.fields.audio_word, card.fields.audio_example, 0)
			} else {
				audioService.playAudio(card.fields.audio_word, 'word')
			}
		}
	}

	const toggleSettings = (e: MouseEvent) => {
		e.stopPropagation()
		e.preventDefault()
		setSettingsOpen(!settingsOpen())
	}

	const handleHideCard = (e: MouseEvent) => {
		e.stopPropagation()
		setSettingsOpen(false)
		console.log('Hide card clicked', currentCard()?.id)
		// For demo, just go to next card
		handleNextCard()
	}

	const handleEditCard = (e: MouseEvent) => {
		e.stopPropagation()
		setSettingsOpen(false)

		const card = currentCard()
		if (card) {
			console.log('Navigating to edit card page', card.id)
			navigate(`/edit-card/${params.deckId}/${card.id}`, { state: { back: true } })
		}
	}

	const handleClickOutside = (e: MouseEvent) => {
		if (e.target instanceof Element) {
			const isMenuButton = (e.target as Element).closest('button[aria-label="Card settings"]')
			const isMenuContent = (e.target as Element).closest('.settings-dropdown') ||
				(e.target as Element).closest('button[onClick="handleHideCard"]') ||
				(e.target as Element).closest('button[onClick="handleEditCard"]')

			if (isMenuButton || isMenuContent) {
				return
			}

			if (settingsOpen()) {
				e.stopPropagation()
				setSettingsOpen(false)
			}
		}
	}

	const handleCardFlip = (e: MouseEvent) => {
		// Don't flip if the settings dropdown is open
		if (settingsOpen()) {
			return
		}

		if (isTransitioning()) return
		if (!flipped()) {
			hapticFeedback('impact', 'light')

			setFlipped(true)
			// Play audio immediately
			playCardAudio()
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

		// Show feedback indicator
		setFeedbackType(rating === 1 ? 'again' : 'good')
		setShowFeedback(true)

		// Hide feedback after a short delay (will still be visible during card transition)
		setTimeout(() => {
			setShowFeedback(false)
		}, 200)

		const timeToSend = finalTimeSpent > 0 ? finalTimeSpent : 1000

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

				if (data?.next_cards && data?.next_cards.length > 0) {
					const current = currentCard()
					const filteredNewCards = current
						? data.next_cards.filter(c => c.id !== current.id)
						: data.next_cards

					if (filteredNewCards.length > 0) {
						setCardBuffer(prev => [...prev, ...filteredNewCards])
					} else {
						setCardBuffer(prev => [...prev, ...data.next_cards])
					}
				}

			} catch (e) {
				console.error(`Exception during background review submission for card ${cardId}:`, e)
			}
		})()
	}

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

			{/* Deck name and progress */}
			<Show when={deck() && !deck.loading}>
				<div class="flex flex-row items-center justify-end gap-4 w-full mb-4">
					<div class="z-20">
						<div class="relative">
							<button
								onClick={toggleSettings}
								class="size-8 rounded-full bg-muted hover:bg-muted/90 flex items-center justify-center text-foreground"
								aria-label="Card settings"
							>
								<svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
									<path d="M12 14a2 2 0 1 0 0-4 2 2 0 0 0 0 4z" fill="currentColor" />
									<path d="M4 14a2 2 0 1 0 0-4 2 2 0 0 0 0 4z" fill="currentColor" />
									<path d="M20 14a2 2 0 1 0 0-4 2 2 0 0 0 0 4z" fill="currentColor" />
								</svg>
							</button>

							<Show when={settingsOpen()}>
								<div
									class="absolute right-0 top-8 mt-1 bg-card shadow-md rounded-md overflow-hidden border border-border w-36 z-30 settings-dropdown">
									<div class="flex flex-col">
										<button
											class="px-3 py-2 hover:bg-muted text-start text-sm w-full flex items-center"
											onClick={handleHideCard}
										>
											<svg class="w-4 h-4 mr-2" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
												<path d="M2 12C2 12 5.5 5 12 5C18.5 5 22 12 22 12C22 12 18.5 19 12 19C5.5 19 2 12 2 12Z"
															stroke="currentColor" stroke-width="2" />
												<path
													d="M12 15C13.6569 15 15 13.6569 15 12C15 10.3431 13.6569 9 12 9C10.3431 9 9 10.3431 9 12C9 13.6569 10.3431 15 12 15Z"
													stroke="currentColor" stroke-width="2" />
												<path d="M4 4L20 20" stroke="currentColor" stroke-width="2" />
											</svg>
											Hide Card
										</button>
										<button
											class="px-3 py-2 hover:bg-muted text-start text-sm w-full flex items-center"
											onClick={handleEditCard}
										>
											<svg class="w-4 h-4 mr-2" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
												<path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5L17 3Z" stroke="currentColor"
															stroke-width="2" />
											</svg>
											Edit Card
										</button>
									</div>
								</div>
							</Show>
						</div>
					</div>
				</div>
			</Show>

			<div class="w-full flex-grow flex flex-col items-center justify-start">
				<Show when={currentCard()}>
					<div class="w-full flex flex-col items-center">
						<div
							class={`w-full cursor-pointer relative perspective transition-all min-h-[500px] ${isTransitioning() ? 'pointer-events-none' : ''}`}
							onClick={handleCardFlip}
						>
							<div class={getFrontFaceClasses(flipped(), isTransitioning())}>
								<div class="text-5xl font-semibold">
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
											textSize="xl"
											language={currentCard()?.fields.language_code || 'ja'}
											transcriptionType={currentCard()?.fields.transcription_type || 'furigana'}
										/>
									</div>
								</Show>
							</div>

							<div class={getBackFaceClasses(flipped(), isTransitioning())}>
								<div class="text-5xl font-semibold flex flex-col items-center">
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
								<div class="text-center text-xl font-normal mb-8">{currentCard()?.fields.meaning_ru}</div>
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
						<p class="text-muted-foreground mb-2">No cards available for review.</p>
						<button
							onClick={() => {
								setNeedMoreCards(true)
								refetchCards()
							}}
							class="mb-4 px-4 py-2 bg-primary text-primary-foreground rounded-md"
						>
							Retry
						</button>
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
								class="justify-center flex flex-col items-center h-14 px-4 bg-error text-error-foreground rounded-md transition-opacity font-medium text-lg"
							>
								<span>
									Again
								</span>
								<span class="text-xs opacity-70">
									{currentCard()?.next_intervals.again}
								</span>
							</button>
							<button
								onClick={() => handleReview(currentCard()!.id, 2)}
								class="justify-center flex flex-col items-center h-14 px-4 bg-info text-info-foreground rounded-md transition-opacity font-medium text-lg"
							>
								<span>
									Good
								</span>
								<span class="text-xs opacity-70">
									{currentCard()?.next_intervals.good}
								</span>
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
