import { createStore } from 'solid-js/store'
import { User as ApiUser, Stats as ApiStats } from '~/lib/api'

// Use the same User type as defined in the API
export type User = ApiUser

// Use the same Stats type as defined in the API
export type Stats = ApiStats

export const [store, setStore] = createStore<{
	user: User
	token: string | null
	stats: Stats
}>({
	user: {} as User,
	token: null,
	stats: {
		due_cards: 0
	}
})

export const setUser = (user: User) => setStore('user', user)

export const setToken = (token: string) => setStore('token', token)

export const updateStats = (stats: Stats) => setStore('stats', stats)

