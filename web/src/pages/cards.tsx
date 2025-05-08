import { createSignal, createResource, For, Show } from 'solid-js'
import { apiRequest, Card } from '~/lib/api'
import { useParams, useNavigate } from '@solidjs/router'
import AudioButton from '~/components/audio-button'

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

	const [cards] = createResource<Card[]>(
		async () => {
			if (!params.deckId) return []
			const { data, error } = await apiRequest<Card[]>(`/cards/due?deck_id=${params.deckId}`)
			if (error) {
				console.error(`Failed to fetch cards for deck ${params.deckId}:`, error)
				return []
			}
			return data || []
		},
	)

	const currentCard = () => {
		const cardList = cards() || []
		if (cardList.length === 0) return null
		return cardList[cardIndex() % cardList.length]
	}

	const handleCardFlip = () => {
		if (isTransitioning()) return;
		setFlipped(!flipped())
	}

	const handleNextCard = () => {
		setIsTransitioning(true);
		setTimeout(() => {
			setFlipped(false);
			setCardIndex(prevIndex => prevIndex + 1);
			setTimeout(() => {
				setIsTransitioning(false);
			}, 50);
		}, 300);
	}

	const handleReview = async (cardId: string, rating: number) => {
		const timeSpent = 5000;
		const { error } = await apiRequest(`/cards/${cardId}/review`, {
			method: 'POST',
			body: JSON.stringify({
				card_id: cardId,
				rating,
				time_spent_ms: timeSpent,
			}),
		});
		if (error) {
			console.error('Failed to submit review:', error);
			return;
		}
		handleNextCard();
	}

	return (
		<div class="container mx-auto px-2 py-6 max-w-md flex flex-col items-center min-h-screen">
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

				{/* ... rest of your component (Loading, No cards, Review buttons) */}
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
