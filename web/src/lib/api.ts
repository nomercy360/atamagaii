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
	level: string
	new_cards_per_day: number
	user_id: string
	created_at: string
	updated_at: string
	deleted_at?: string
	stats?: DeckProgress
	completed_today_cards?: number
}

export interface DeckProgress {
	completed_today_cards: number
	new_cards: number
	learning_cards: number
	review_cards: number
}

export interface CardFields {
	term: string
	transcription: string
	term_with_transcription: string
	meaning_en: string
	meaning_ru: string
	example_native: string
	example_with_transcription: string
	example_en: string
	example_ru: string
	frequency: number
	language_code: string
	transcription_type: string
	audio_word: string
	audio_example: string
	image_url?: string
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
	next_intervals: {
		good: string
		again: string
	}
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

export interface StudyStats {
	cards_studied_today: number;
	avg_time_per_card_ms: number;
	total_time_studied_ms: number;
	new_cards_today: number;
	review_cards_today: number;
	total_cards: number;
	total_time_studied: string;
	study_days: number;
	total_reviews: number;
	streak_days: number;
}

export interface StudyHistoryItem {
	date: string;
	card_count: number;
	time_spent_ms: number;
}

export interface Stats {
	due_cards: number;
	study_stats: StudyStats;
	[key: string]: any;
}

export interface StudyHistory {
	history: StudyHistoryItem[];
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
	file_name: string
}

export interface ImportDeckRequest {
	name: string
	file_name: string
}

export interface ImportDeckResponse {
	id: string
	name: string
	card_count: number
}

export interface AvailableDeck {
	id: string
	name: string
	level: string
}

export interface LanguageGroup {
	code: string
	name: string
	decks: AvailableDeck[]
}

export interface AvailableDecksResponse {
	languages: LanguageGroup[]
}

export async function getAvailableDecks(): Promise<{
	data: AvailableDecksResponse | null
	error: string | null
}> {
	return apiRequest('/decks/available', {
		method: 'GET',
	})
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
	skip_card_id?: string
}

export interface CardReviewResponse {
	stats: {
		new_cards: number
		learning_cards: number
		review_cards: number
		completed_today_cards: number
	}
	next_cards: Card[]
}

export interface UpdateDeckSettingsRequest {
	new_cards_per_day: number
	name?: string
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

export interface UpdateCardRequest {
	fields: CardFields
}

export async function updateCard(cardId: string, request: UpdateCardRequest): Promise<{
	data: Card | null
	error: string | null
}> {
	return apiRequest(`/cards/${cardId}`, {
		method: 'PUT',
		body: JSON.stringify(request),
	})
}

export async function getCard(cardId: string): Promise<{
	data: Card | null
	error: string | null
}> {
	return apiRequest(`/cards/${cardId}`, {
		method: 'GET',
	})
}

export interface GenerateCardRequest {
	card_id: string
	deck_id: string
}

export async function generateCard(request: GenerateCardRequest): Promise<{
	data: Card | null
	error: string | null
}> {
	return apiRequest(`/cards/generate`, {
		method: 'POST',
		body: JSON.stringify(request),
	})
}

export async function getStats(): Promise<{
	data: Stats | null
	error: string | null
}> {
	return apiRequest('/stats', {
		method: 'GET',
	})
}

export async function getStudyHistory(days: number = 100): Promise<{
	data: StudyHistory | null
	error: string | null
}> {
	return apiRequest(`/stats/history?days=${days}`, {
		method: 'GET',
	})
}

export interface TasksPerDeck {
	deck_id: string;
	deck_name: string;
	total_tasks: number;
}

export async function getTasksPerDeck(): Promise<{
	data: TasksPerDeck[] | null
	error: string | null
}> {
	return apiRequest('/tasks/by-deck', {
		method: 'GET',
	})
}


