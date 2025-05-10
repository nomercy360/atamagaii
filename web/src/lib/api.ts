import { store } from '~/store'

export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL as string

export async function apiRequest<T = any>(endpoint: string, options: RequestInit = {}): Promise<{
	data: T | null
	error: string | null
}> {
	try {
		const response = await fetch(`${API_BASE_URL}/v1${endpoint}`, {
			...options,
			headers: {
				'Content-Type': 'application/json',
				Authorization: `Bearer ${store.token}`,
				...(options.headers || {}),
			},
		})

		let data
		try {
			data = await response.json()
		} catch {
			return { error: 'Failed to get response from server', data: null }
		}

		if (!response.ok) {
			const errorMessage =
				Array.isArray(data?.error)
					? data.error.join('\n')
					: typeof data?.error === 'string'
						? data.error
						: 'An error occurred'

			return { error: errorMessage, data: null }
		}

		return { data, error: null }
	} catch (error) {
		const errorMessage = error instanceof Error ? error.message : 'An unexpected error occurred'
		return { error: errorMessage, data: null }
	}
}

export interface User {
	id: string
	telegram_id: number
	username: string
	avatar_url: string
	name: string
	level: string
	points: number
	created_at: string
	updated_at: string
}

export interface Deck {
	id: string
	name: string
	description: string
	level: string
	new_cards_per_day: number
	user_id: string
	created_at: string
	updated_at: string
	deleted_at?: string
	due_cards?: number
	new_cards?: number
	learning_cards?: number
	review_cards?: number
}

export interface CardFields {
	// Core terminology fields (language agnostic)
	term: string                       // Primary term in native script (was "word")
	transcription: string              // Reading aid (pinyin, romaji, etc.) (was "reading")
	term_with_transcription: string    // Term with reading aids embedded (was "word_furigana")

	// Meanings in different languages
	meaning_en: string
	meaning_ru: string

	// Example sentences
	example_native: string             // Example in native script (was "example_ja")
	example_with_transcription: string // Example with reading aids (was "example_furigana")
	example_en: string
	example_ru: string

	// Metadata
	frequency: number
	language_code: string              // ISO 639-1 language code (e.g., "ja", "zh", "en")
	transcription_type: string         // Type of transcription (furigana, pinyin, etc.)

	// Media
	audio_word: string                 // Audio for term pronunciation
	audio_example: string              // Audio for example sentence
	image_url?: string                 // Illustration image

	// Legacy field names (for backward compatibility)
	word?: string
	reading?: string
	word_furigana?: string
	example_ja?: string
	example_furigana?: string
}


export interface Card {
	id: string
	deck_id: string
	fields: CardFields
	created_at: string
	updated_at: string
	deleted_at?: string
	next_review?: string
	interval?: number
	ease?: number
	review_count?: number
	laps_count?: number
	last_reviewed_at?: string
	first_reviewed_at?: string
	state?: string
	learning_step?: number
}

export interface CardProgress {
	user_id: string
	card_id: string
	next_review?: string
	interval: number
	ease: number
	review_count: number
	laps_count: number
	last_reviewed_at?: string
}

export interface Stats {
	due_cards: number

	[key: string]: any
}

export interface AuthTelegramRequest {
	query: string
}

export interface AuthTelegramResponse {
	token: string
	user: User
}

export interface CreateDeckRequest {
	name: string
	description: string
	file_name: string
}

export interface ImportDeckRequest {
	name: string
	description: string
	file_name: string
}

export interface ImportDeckResponse {
	id: string
	name: string
	card_count: number
}

export async function importDeck(request: ImportDeckRequest): Promise<{
	data: ImportDeckResponse | null
	error: string | null
}> {
	return apiRequest('/decks/import', {
		method: 'POST',
		body: JSON.stringify(request),
	})
}

export interface CardReviewRequest {
	card_id: string
	rating: number
	time_spent_ms: number
}

export interface CardReviewResponse {
	stats: {
		new_cards: number
		learning_cards: number
		review_cards: number
	}
	// The backend now returns a Card object directly instead of CardProgress
}

export interface UpdateDeckSettingsRequest {
	new_cards_per_day: number
}

export async function updateDeckSettings(deckId: string, settings: UpdateDeckSettingsRequest): Promise<{
	data: Deck | null
	error: string | null
}> {
	return apiRequest(`/decks/${deckId}/settings`, {
		method: 'PUT',
		body: JSON.stringify(settings),
	})
}

export async function deleteDeck(deckId: string): Promise<{
	data: null
	error: string | null
}> {
	return apiRequest(`/decks/${deckId}`, {
		method: 'DELETE',
	})
}


