import { createSignal, onCleanup, onMount, For, createResource, Show } from 'solid-js'
import { importDeck, getAvailableDecks, AvailableDeck } from '~/lib/api'
import { useNavigate } from '@solidjs/router'
import { useMainButton } from '~/lib/useMainButton'
import { useBackButton } from '~/lib/useBackButton'
import { showToast } from '~/lib/toast-service'
import { queryClient } from '~/App'

type FlagProps = {
	code: string;
	clsName?: string;
	width?: number;
	height?: number;
}

export function FlagIcon({ code, width = 512, height = 512, clsName }: FlagProps) {
	const flags = {
		th: (
			<svg width={width}
					 height={height}
					 viewBox="0 0 512 512"
					 fill="none"
					 xmlns="http://www.w3.org/2000/svg"
					 class={clsName}>
				<rect width="512" height="512" fill="var(--flag-palette-white, #eeeeee)" />
				<path
					fill-rule="evenodd"
					clip-rule="evenodd"
					d="M512 0H0V85.3333H512V0ZM512 426.666H0V511.999H512V426.666Z"
					fill="var(--flag-palette-bright-red, #d80027)"
				/>
				<rect y="170.668" width="512" height="170.667" fill="var(--flag-palette-navy, #002266)" />
			</svg>
		),
		ge: (
			<svg width={width}
					 height={height}
					 viewBox="0 0 512 512"
					 fill="none"
					 xmlns="http://www.w3.org/2000/svg"
					 class={clsName}>
				<rect width="512" height="512" fill="var(--flag-palette-white, #eeeeee)" />
				<path
					d="M512 288V224H288V0H224V224H0V288H224V512H288V288H512Z"
					fill="var(--flag-palette-bright-red, #d80027)"
				/>
				<path
					fill-rule="evenodd"
					clip-rule="evenodd"
					d="M384 64V96H352V128H384V160H416V128H448V96H416V64H384ZM96 384V352H128V384H160V416H128V448H96V416H64V384H96ZM384 384V352H416V384H448V416H416V448H384V416H352V384H384ZM96 96V64H128V96H160V128H128V160H96V128H64V96H96Z"
					fill="var(--flag-palette-bright-red, #d80027)"
				/>
			</svg>
		),
		jp: (
			<svg
				width={width}
				height={height}
				viewBox="0 0 512 512"
				fill="none"
				xmlns="http://www.w3.org/2000/svg"
				class={clsName}>
				<rect width="512" height="512" fill="var(--flag-palette-white, #eeeeee)" />
				<path
					d="M256 368C317.856 368 368 317.856 368 256C368 194.144 317.856 144 256 144C194.144 144 144 194.144 144 256C144 317.856 194.144 368 256 368Z"
					fill="var(--flag-palette-bright-red, #d80027)"
				/>
			</svg>
		),
	}

	return flags[code as keyof typeof flags] || null
}

export default function ImportDeck() {
	const navigate = useNavigate()
	const mainButton = useMainButton()
	const backButton = useBackButton()

	const [name, setName] = createSignal('')
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
		mainButton.setVisible('Add Deck')
		mainButton.disable()

		mainButton.onClick(handleImport)
	})

	onCleanup(() => {
		mainButton.hide()
		mainButton.offClick(handleImport)
	})

	const updateMainButton = () => {
		if (name() && selectedFile()) {
			mainButton.enable('Add Deck')
		} else {
			mainButton.disable('Add Deck')
		}
	}

	const selectPredefinedDeck = (fileId: string, deck: AvailableDeck) => {
		setSelectedFile(fileId)
		if (deck) {
			if (!name()) setName(deck.name)
		}
		updateMainButton()
	}

	const handleImport = async () => {
		if (!name() || !selectedFile()) {
			setError('Please fill all fields and select a deck')
			return
		}

		setIsLoading(true)
		setError(null)
		mainButton.showProgress(true)

		try {
			const { data, error } = await importDeck({
				name: name(),
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
			queryClient.invalidateQueries({ queryKey: ['decks'] })

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
		<div class=" container mx-auto px-4 py-6 max-w-md overflow-y-auto h-screen">
			<h1 class=" text-2xl font-bold mb-6">New Deck</h1>

			<div class=" space-y-4">
				{error() && (
					<div class=" p-3 rounded-md bg-error/20 text-error-foreground text-sm">
						{error()}
					</div>)
				}

				<div class="space-y-4">
					<Show when={!availableDecksData.loading} fallback={<div class="text-sm">Loading available decks...</div>}>
						<div class="space-y-4">
							<div class="flex flex-wrap gap-2">
								<For each={availableDecksData()?.languages || []}>
									{(language) => (
										<button
											type="button"
											onClick={() => setSelectedLanguage(language.code)}
											class={`px-3 py-2 rounded-md text-sm font-medium flex items-center gap-2
												${selectedLanguage() === language.code ? 'bg-secondary text-secondary-foreground border' : 'bg-card text-foreground border border-border'}`}
											disabled={isLoading()}
										>
											<span class="size-4 inline-block">
												<FlagIcon clsName="rounded-full" code={language.code} width={16} height={16} />
											</span>
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
