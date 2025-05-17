import { createSignal, createEffect, Show } from 'solid-js'
import { Deck, updateDeckSettings, deleteDeck } from '~/lib/api'

interface DeckSettingsProps {
	deck: Deck;
	onUpdate: (updatedDeck: Deck) => void;
	onClose: () => void;
	onDelete?: () => void;
}

export default function DeckSettings(props: DeckSettingsProps) {
	const [newCardsPerDay, setNewCardsPerDay] = createSignal(props.deck.new_cards_per_day)
	const [deckName, setDeckName] = createSignal(props.deck.name)
	const [loading, setLoading] = createSignal(false)
	const [error, setError] = createSignal<string | null>(null)
	const [showDeleteConfirm, setShowDeleteConfirm] = createSignal(false)

	createEffect(() => {
		setNewCardsPerDay(props.deck.new_cards_per_day)
		setDeckName(props.deck.name)
	})

	const handleSubmit = async (e: Event) => {
		e.preventDefault()
		setLoading(true)
		setError(null)

		// Validate deck name is not empty
		if (!deckName().trim()) {
			setError("Deck name cannot be empty")
			setLoading(false)
			return
		}

		const { data, error } = await updateDeckSettings(props.deck.id, {
			new_cards_per_day: newCardsPerDay(),
			name: deckName()
		})

		setLoading(false)

		if (error) {
			setError(error)
			return
		}

		if (data) {
			props.onUpdate(data)
			props.onClose()
		}
	}

	const handleDelete = async () => {
		setLoading(true)
		setError(null)

		const { error } = await deleteDeck(props.deck.id)

		setLoading(false)

		if (error) {
			setError(error)
			return
		}

		// Call the onDelete callback to refresh data
		if (props.onDelete) {
			props.onDelete()
		}

		// Always close the settings dialog after successful deletion
		props.onClose()
	}

	const handleOutsideClick = (e: MouseEvent) => {
		// Only close if not in delete confirmation mode and the click was on the backdrop
		if (!showDeleteConfirm() && e.target === e.currentTarget) {
			props.onClose()
		}
	}

	return (
		<div
			class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
			onClick={handleOutsideClick}
		>
			<div class="bg-card text-card-foreground rounded-lg shadow-lg p-6 w-full max-w-md mx-4 relative">
				<Show when={!showDeleteConfirm()} fallback={
					<div>
						<h2 class="text-xl font-bold mb-4">Delete Deck</h2>
						<p class="mb-4">Are you sure you want to delete this deck? This action cannot be undone. All cards and
							progress in this deck will be deleted.</p>

						{error() && (
							<div class="mb-4 p-2 bg-error/10 text-error rounded-md text-sm">
								{error()}
							</div>
						)}

						<div class="flex justify-end gap-2">
							<button
								type="button"
								onClick={() => setShowDeleteConfirm(false)}
								class="h-8 px-2 border border-border rounded-md text-xs font-medium"
								disabled={loading()}
							>
								Cancel
							</button>
							<button
								type="button"
								onClick={handleDelete}
								class="h-8 px-2 bg-destructive text-destructive-foreground rounded-md text-xs font-medium"
								disabled={loading()}
							>
								{loading() ? 'Deleting...' : 'Delete Deck'}
							</button>
						</div>
					</div>
				}>
					<h2 class="text-xl font-bold mb-4">Deck Settings</h2>
					<form onSubmit={handleSubmit}>
						<div class="mb-4">
							<label class="block text-sm font-medium mb-1">
								Deck Name
							</label>
							<input
								type="text"
								value={deckName()}
								onInput={(e) => setDeckName(e.currentTarget.value)}
								class="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary text-foreground bg-background"
								placeholder="Enter deck name"
							/>
						</div>

						<div class="mb-4">
							<label class="block text-sm font-medium mb-1">
								New Cards Per Day
							</label>
							<input
								type="number"
								min="1"
								max="500"
								value={newCardsPerDay()}
								onInput={(e) => setNewCardsPerDay(parseInt(e.currentTarget.value) || 20)}
								class="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary text-foreground bg-background"
							/>
							<p class="text-xs text-muted-foreground mt-1">
								Maximum number of new cards you'll receive each day from this deck
							</p>
						</div>

						{error() && (
							<div class="mb-4 p-2 bg-error/10 text-error rounded-md text-sm">
								{error()}
							</div>
						)}

						<div class="flex justify-end gap-2">
							<button
								type="button"
								onClick={() => setShowDeleteConfirm(true)}
								class="px-2 h-8 bg-destructive text-destructive-foreground rounded-md text-xs font-medium"
								disabled={loading()}
							>
								Delete Deck
							</button>
							<button
								type="button"
								onClick={props.onClose}
								class="px-2 h-8 border border-border rounded-md text-xs font-medium"
								disabled={loading()}
							>
								Cancel
							</button>
							<button
								type="submit"
								class="px-2 h-8 bg-primary text-primary-foreground rounded-md text-xs font-medium"
								disabled={loading()}
							>
								{loading() ? 'Saving...' : 'Save Settings'}
							</button>
						</div>
					</form>
				</Show>
			</div>
		</div>
	)
}
