import { createSignal, onCleanup, onMount } from 'solid-js'
import { importDeck } from '~/lib/api'
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
	const [isLoading, setIsLoading] = createSignal(false)
	const [error, setError] = createSignal<string | null>(null)

	// Predefined decks available for import
	const availableDecks = [
		{ id: 'vocab_n5.json', name: 'N5 Vocabulary', description: 'Basic Japanese vocabulary for JLPT N5 level' },
		{ id: 'vocab_n4.json', name: 'N4 Vocabulary', description: 'Japanese vocabulary for JLPT N4 level' },
		{ id: 'vocab_n3.json', name: 'N3 Vocabulary', description: 'Intermediate Japanese vocabulary for JLPT N3 level' },
	]

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

	const selectPredefinedDeck = (fileId: string) => {
		setSelectedFile(fileId)
		const deck = availableDecks.find(d => d.id === fileId)
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

				<div class="space-y-2">
					<label class="text-sm font-medium mb-2 block">
						Select Deck to Import
					</label>
					<div class="space-y-2">
						{availableDecks.map(deck => (
							<button
								type="button"
								onClick={() => selectPredefinedDeck(deck.id)}
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
								</div>
							</button>
						))}
					</div>
				</div>
			</div>

			<div class="text-sm text-muted-foreground mt-8">
				<p>Select a predefined deck to import. These decks contain vocabulary organized by JLPT level.</p>
			</div>
		</div>
	)
}
