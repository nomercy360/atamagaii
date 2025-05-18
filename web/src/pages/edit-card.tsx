import { createSignal, createResource, Show, onMount, onCleanup } from 'solid-js'
import { useParams, useNavigate } from '@solidjs/router'
import { Card, CardFields, updateCard, UpdateCardRequest, getCard, generateCard } from '~/lib/api'
import AudioButton from '~/components/audio-button'
import { useMainButton } from '~/lib/useMainButton'
import { showToast } from '~/lib/toast-service'

export default function EditCard() {
	const params = useParams()
	const navigate = useNavigate()
	const mainButton = useMainButton()

	const [loading, setLoading] = createSignal(false)
	const [error, setError] = createSignal<string | null>(null)
	const [_, setFormValid] = createSignal(false)
	const [generatingAI, setGeneratingAI] = createSignal(false)

	// Default empty card fields
	const defaultCardFields: CardFields = {
		term: '',
		transcription: '',
		term_with_transcription: '',
		meaning_en: '',
		meaning_ru: '',
		example_native: '',
		example_with_transcription: '',
		example_en: '',
		example_ru: '',
		frequency: 0,
		language_code: 'jp',
		transcription_type: 'furigana',
		audio_word: '',
		audio_example: '',
	}

	const [cardFields, setCardFields] = createSignal<CardFields>(defaultCardFields)

	// Fetch the card data when the component mounts
	const [card] = createResource<Card | null>(
		async () => {
			if (!params.cardId) return null

			setLoading(true)
			setError(null)

			try {
				const { data, error } = await getCard(params.cardId)

				if (error) {
					setError(error)
					return null
				}

				if (data) {
					setCardFields(data.fields)
					validateForm()
					return data
				}

				return null
			} catch (err) {
				setError('Failed to fetch card details')
				return null
			} finally {
				setLoading(false)
			}
		},
	)

	const validateForm = () => {
		// Check required fields
		const fields = cardFields()
		const isValid = !!fields.term && !!fields.meaning_ru

		setFormValid(isValid)

		// Update main button state
		if (isValid) {
			mainButton.enable('Save Changes')
		} else {
			mainButton.disable('Save Changes')
		}

		return isValid
	}

	// Handler for input changes
	const handleInputChange = (e: Event, field: keyof CardFields) => {
		const target = e.target as HTMLInputElement | HTMLTextAreaElement
		const value = field === 'frequency' ? Number(target.value) : target.value

		setCardFields((prev) => ({
			...prev,
			[field]: value,
		}))

		// Validate the form after input change
		setTimeout(validateForm, 0)
	}

	// Handle AI generation for card content
	const handleGenerateAI = async () => {
		if (!params.cardId || !params.deckId) {
			setError('Missing card or deck ID')
			return
		}

		// Ensure the term field is filled
		if (!cardFields().term) {
			setError('Term field is required for AI generation')
			showToast.error('Please add a term before using AI generation')
			return
		}

		setGeneratingAI(true)
		setError(null)

		try {
			const { data, error } = await generateCard({
				card_id: params.cardId,
				deck_id: params.deckId,
			})

			if (error) {
				setError(error)
				showToast.error(`AI generation failed: ${error}`)
				return
			}

			if (data) {
				setCardFields(data.fields)
				validateForm()
				showToast.success('Card content generated successfully!')
			}
		} catch (err) {
			setError('Failed to generate card content')
			showToast.error('AI generation failed')
		} finally {
			setGeneratingAI(false)
		}
	}

	// Save card changes
	const handleSave = async () => {
		if (!validateForm()) {
			setError('Please fill in all required fields')
			return
		}

		if (!params.cardId || !params.deckId) {
			setError('Missing card or deck ID')
			return
		}

		setLoading(true)
		setError(null)
		mainButton.showProgress(true)

		try {
			const updateRequest: UpdateCardRequest = {
				fields: cardFields(),
			}

			const { data, error } = await updateCard(params.cardId, updateRequest)

			if (error) {
				setError(error)
				showToast.error(error)
				mainButton.hideProgress()
				mainButton.enable('Try Again')
				return
			}

			showToast.success('Card updated successfully!')

			// Navigate back to the cards view after successful save
			setTimeout(() => {
				navigate(`/cards/${params.deckId}`)
			}, 500)
		} catch (err) {
			setError('Failed to save card changes')
			showToast.error('Failed to save card changes')
		} finally {
			setLoading(false)
			mainButton.hideProgress()
		}
	}

	onMount(() => {
		// Set up Telegram main button
		mainButton.setVisible('Save Changes')
		mainButton.disable()
		mainButton.onClick(handleSave)

		// Initial form validation
		validateForm()
	})

	onCleanup(() => {
		// Clean up Telegram buttons
		mainButton.hide()
		mainButton.offClick(handleSave)
	})

	return (
		<div class="container mx-auto px-4 py-6 max-w-lg h-screen overflow-y-auto pb-20">
			<div class="flex flex-row justify-between items-center mb-4">
				<h1 class="text-2xl font-bold">Edit Card</h1>
				<button
					type="button"
					onClick={handleGenerateAI}
					disabled={generatingAI() || !cardFields().term}
					title="Generate card content with AI (requires term field)"
					class="size-8 rounded-md bg-purple-700 text-white flex items-center justify-center disabled:opacity-40 disabled:cursor-not-allowed"
				>
					<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="currentColor"
							 stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
							 class="size-4">
						<path
							d="M9.937 15.5A2 2 0 0 0 8.5 14.063l-6.135-1.582a.5.5 0 0 1 0-.962L8.5 9.936A2 2 0 0 0 9.937 8.5l1.582-6.135a.5.5 0 0 1 .963 0L14.063 8.5A2 2 0 0 0 15.5 9.937l6.135 1.581a.5.5 0 0 1 0 .964L15.5 14.063a2 2 0 0 0-1.437 1.437l-1.582 6.135a.5.5 0 0 1-.963 0z" />
						<path d="M20 3v4" />
						<path d="M22 5h-4" />
						<path d="M4 17v2" />
						<path d="M5 18H3" />
					</svg>
				</button>
			</div>

			<Show when={error()}>
				<div class="bg-error/10 text-error p-3 rounded-md mb-4">
					{error()}
				</div>
			</Show>

			<Show when={loading() && !card()}>
				<div class="flex justify-center items-center h-40">
					<p class="text-muted-foreground">Loading card details...</p>
				</div>
			</Show>

			<Show when={generatingAI()}>
				<div class="flex justify-center items-center h-20 mb-4">
					<div class="flex items-center gap-2 text-primary">
						<div class="animate-spin h-5 w-5 border-2 border-primary border-t-transparent rounded-full"></div>
						<p>Generating with AI...</p>
					</div>
				</div>
			</Show>

			<Show when={!loading() || card()}>
				<form class="space-y-6" onSubmit={(e) => e.preventDefault()}>
					{/* Term section */}
					<div class="space-y-4">
						<h2 class="text-xl font-semibold">Term</h2>

						<div>
							<label class="block text-sm font-medium mb-1" for="term">
								Term <span class="text-error">*</span>
							</label>
							<input
								id="term"
								type="text"
								value={cardFields().term}
								onInput={(e) => handleInputChange(e, 'term')}
								class="bg-secondary w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary"
								required
							/>
						</div>

						<div>
							<label class="block text-sm font-medium mb-1" for="transcription">
								Transcription
							</label>
							<input
								id="transcription"
								type="text"
								value={cardFields().transcription}
								onInput={(e) => handleInputChange(e, 'transcription')}
								class="bg-secondary w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary"
							/>
						</div>

						<div>
							<label class="block text-sm font-medium mb-1" for="term_with_transcription">
								Term with Transcription
							</label>
							<input
								id="term_with_transcription"
								type="text"
								value={cardFields().term_with_transcription}
								onInput={(e) => handleInputChange(e, 'term_with_transcription')}
								class="bg-secondary w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary"
							/>
						</div>

						<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
							<div>
								<label class="block text-sm font-medium mb-1" for="meaning_en">
									Meaning (English)
								</label>
								<input
									id="meaning_en"
									type="text"
									value={cardFields().meaning_en}
									onInput={(e) => handleInputChange(e, 'meaning_en')}
									class="bg-secondary w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary"
								/>
							</div>

							<div>
								<label class="block text-sm font-medium mb-1" for="meaning_ru">
									Meaning (Russian) <span class="text-error">*</span>
								</label>
								<input
									id="meaning_ru"
									type="text"
									value={cardFields().meaning_ru}
									onInput={(e) => handleInputChange(e, 'meaning_ru')}
									class="bg-secondary w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary"
									required
								/>
							</div>
						</div>

						<div>
							<label class="block text-sm font-medium mb-1" for="audio_word">
								Word Audio URL
							</label>
							<div class="flex items-center gap-2">
								<input
									id="audio_word"
									type="text"
									value={cardFields().audio_word}
									onInput={(e) => handleInputChange(e, 'audio_word')}
									class="bg-secondary w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary"
								/>
								<Show when={cardFields().audio_word}>
									<AudioButton
										audioUrl={cardFields().audio_word}
										size="sm"
										label="Play word audio"
										type="word"
									/>
								</Show>
							</div>
						</div>
					</div>

					{/* Example section */}
					<div class="space-y-4">
						<h2 class="text-xl font-semibold">Example</h2>

						<div>
							<label class="block text-sm font-medium mb-1" for="example_native">
								Example (Native)
							</label>
							<textarea
								id="example_native"
								value={cardFields().example_native}
								onInput={(e) => handleInputChange(e, 'example_native')}
								class="bg-secondary w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary min-h-[80px]"
							/>
						</div>

						<div>
							<label class="block text-sm font-medium mb-1" for="example_with_transcription">
								Example with Transcription
							</label>
							<textarea
								id="example_with_transcription"
								value={cardFields().example_with_transcription}
								onInput={(e) => handleInputChange(e, 'example_with_transcription')}
								class="bg-secondary w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary min-h-[80px]"
							/>
						</div>

						<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
							<div>
								<label class="block text-sm font-medium mb-1" for="example_en">
									Example Translation (English)
								</label>
								<textarea
									id="example_en"
									value={cardFields().example_en}
									onInput={(e) => handleInputChange(e, 'example_en')}
									class="bg-secondary w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary min-h-[80px]"
								/>
							</div>

							<div>
								<label class="block text-sm font-medium mb-1" for="example_ru">
									Example Translation (Russian)
								</label>
								<textarea
									id="example_ru"
									value={cardFields().example_ru}
									onInput={(e) => handleInputChange(e, 'example_ru')}
									class="bg-secondary w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary min-h-[80px]"
								/>
							</div>
						</div>

						<div>
							<label class="block text-sm font-medium mb-1" for="audio_example">
								Example Audio URL
							</label>
							<div class="flex items-center gap-2">
								<input
									id="audio_example"
									type="text"
									value={cardFields().audio_example}
									onInput={(e) => handleInputChange(e, 'audio_example')}
									class="bg-secondary w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary"
								/>
								<Show when={cardFields().audio_example}>
									<AudioButton
										audioUrl={cardFields().audio_example}
										size="sm"
										label="Play example audio"
										type="example"
									/>
								</Show>
							</div>
						</div>
					</div>
					<div class="h-16"></div>
				</form>
			</Show>
		</div>
	)
}
