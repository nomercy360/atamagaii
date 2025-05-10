import { createSignal, onCleanup, onMount, For, createResource, Show } from 'solid-js'
import { importDeck, getAvailableDecks, LanguageGroup, AvailableDeck } from '~/lib/api'
import { useNavigate } from '@solidjs/router'
import { useMainButton } from '~/lib/useMainButton'
import { useBackButton } from '~/lib/useBackButton'
import { showToast } from '~/lib/toast-service'

export default function ImportDeck() {
	const navigate = useNavigate()
	const mainButton = useMainButton()
	const backButton = useBackButton()

	const [name, setName] = createSignal('')
	const [description, setDescription] = createSignal('')
	const [selectedFile, setSelectedFile] = createSignal<string | null>(null)
	const [selectedLanguage, setSelectedLanguage] = createSignal<string | null>(null)
	const [isLoading, setIsLoading] = createSignal(false)
	const [error, setError] = createSignal<string | null>(null)

	// Fetch available decks from API
	const [availableDecksData] = createResource(async () => {
		const response = await getAvailableDecks()
		if (response.error) {
			setError(response.error)
			return { languages: [] }
		}
		// If there's only one language, preselect it
		if (response.data && response.data.languages.length === 1) {
			setSelectedLanguage(response.data.languages[0].code)
		}
		return response.data
	})

	onMount(() => {
		mainButton.setVisible('Import Deck')
		mainButton.disable()

		mainButton.onClick(handleImport)
	})

	onCleanup(() => {
		mainButton.hide()
		mainButton.offClick(handleImport)
	})

	const updateMainButton = () => {
		if (name() && description() && selectedFile()) {
			mainButton.enable('Import Deck')
		} else {
			mainButton.disable('Import Deck')
		}
	}

	const selectPredefinedDeck = (fileId: string, deck: AvailableDeck) => {
		setSelectedFile(fileId)
		if (deck) {
			if (!name()) setName(deck.name)
			if (!description()) setDescription(deck.description)
		}
		updateMainButton()
	}

	const handleImport = async () => {
		if (!name() || !description() || !selectedFile()) {
			setError('Please fill all fields and select a deck')
			return
		}

		setIsLoading(true)
		setError(null)
		mainButton.showProgress(true)

		try {
			const { data, error } = await importDeck({
				name: name(),
				description: description(),
				file_name: selectedFile()!,
			})

			if (error) {
				setError(error)
				mainButton.hideProgress()
				mainButton.enable('Try Again')
				showToast.error(error)
				return
			}

			showToast.success('Deck imported successfully!')

			setTimeout(() => {
				navigate('/')
			}, 500)
		} catch (err) {
			console.error('Error importing deck:', err)
			setError('An unexpected error occurred')
		} finally {
			setIsLoading(false)
			mainButton.hideProgress()
		}
	}

	return (
		<div class="container mx-auto px-4 py-6 max-w-md">
			<h1 class="text-2xl font-bold mb-6">Import Deck</h1>

			<div class="space-y-4">
				{error() && (
					<div class="p-3 rounded-md bg-error/20 text-error-foreground text-sm">
						{error()}
					</div>
				)}

				<div class="space-y-2">
					<label for="name" class="text-sm font-medium">
						Deck Name
					</label>
					<input
						id="name"
						type="text"
						value={name()}
						onInput={(e) => {
							setName(e.currentTarget.value)
							updateMainButton()
						}}
						class="w-full px-3 py-2 border border-border rounded-md bg-card text-foreground"
						placeholder="Enter deck name"
						disabled={isLoading()}
					/>
				</div>

				<div class="space-y-2">
					<label for="description" class="text-sm font-medium">
						Description
					</label>
					<textarea
						id="description"
						value={description()}
						onInput={(e) => {
							setDescription(e.currentTarget.value)
							updateMainButton()
						}}
						class="w-full px-3 py-2 border border-border rounded-md bg-card text-foreground h-20 resize-none"
						placeholder="Enter deck description"
						disabled={isLoading()}
					/>
				</div>

				<div class="space-y-4">
					<label class="text-sm font-medium mb-2 block">
						Select Language
					</label>
					<Show when={!availableDecksData.loading} fallback={<div class="text-sm">Loading available decks...</div>}>
						<div class="space-y-4">
							<div class="flex flex-wrap gap-2">
								<For each={availableDecksData()?.languages || []}>
									{(language) => (
										<button
											type="button"
											onClick={() => setSelectedLanguage(language.code)}
											class={`px-3 py-2 rounded-md text-sm font-medium
												${selectedLanguage() === language.code ? 'bg-primary text-primary-foreground' : 'bg-card text-foreground border border-border'}`}
											disabled={isLoading()}
										>
											{language.name}
										</button>
									)}
								</For>
							</div>

							<Show when={selectedLanguage()}>
								<div class="space-y-2">
									<label class="text-sm font-medium mb-2 block">
										Select Deck to Import
									</label>
									<div class="space-y-2">
										<For each={availableDecksData()?.languages.find(l => l.code === selectedLanguage())?.decks || []}>
											{(deck) => (
												<button
													type="button"
													onClick={() => selectPredefinedDeck(deck.id, deck)}
													class={`w-full text-left p-3 border ${selectedFile() === deck.id ? 'border-primary bg-primary/10' : 'border-border'} rounded-md bg-card flex items-center`}
													disabled={isLoading()}
												>
													<div
														class={`w-4 h-4 rounded-full border mr-2 flex items-center justify-center ${selectedFile() === deck.id ? 'border-primary' : 'border-muted-foreground'}`}>
														{selectedFile() === deck.id && <div class="w-2 h-2 rounded-full bg-primary"></div>}
													</div>
													<div>
														<p class="font-medium">{deck.name}</p>
														<p class="text-xs text-muted-foreground">{deck.description}</p>
														<p class="text-xs text-muted-foreground mt-1">Level: {deck.level}</p>
													</div>
												</button>
											)}
										</For>
									</div>
								</div>
							</Show>
						</div>
					</Show>
				</div>
			</div>

			<div class="text-sm text-muted-foreground mt-8">
				<p>Select a language and predefined deck to import. These decks contain vocabulary organized by level.</p>
			</div>
		</div>
	)
}
