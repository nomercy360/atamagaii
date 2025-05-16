import { createSignal, For, Show, useTransition } from 'solid-js'
import { useQuery } from '@tanstack/solid-query'
import { getStats, getStudyHistory, StudyHistoryItem, StudyStats } from '~/lib/api'

export default function Statistics() {
	const [selectedMonth, setSelectedMonth] = createSignal<number>(new Date().getMonth())
	const [selectedYear, setSelectedYear] = createSignal<number>(new Date().getFullYear())
	const [isPending, startTransition] = useTransition()

	// Fetch stats data
	const statsQuery = useQuery(() => ({
		queryKey: ['study_stats'],
		queryFn: async () => {
			const { data, error } = await getStats()
			if (error) throw new Error(error)
			return data
		},
	}))

	// Fetch history data
	const historyQuery = useQuery(() => ({
		queryKey: ['study_history', selectedYear(), selectedMonth()],
		queryFn: async () => {
			// Get the last 100 days of study history
			const { data, error } = await getStudyHistory(100) // Get last 100 days of data
			if (error) throw new Error(error)
			return data
		},
	}))

	// Generate calendar data
	const calendar = () => {
		if (!historyQuery.data?.history) return []

		const year = selectedYear()
		const month = selectedMonth()

		// Create a date for the first day of the month
		const firstDay = new Date(year, month, 1)
		// Get the day of the week (0-6, 0 is Sunday) of the first day
		const startingDay = firstDay.getDay()

		// Get the number of days in the month
		const lastDay = new Date(year, month + 1, 0)
		const daysInMonth = lastDay.getDate()

		// Create an array for all days in the month
		const days = Array(daysInMonth + startingDay).fill(null)

		// Map history data to days
		const historyMap = new Map<string, StudyHistoryItem>()
		historyQuery.data.history.forEach(item => {
			historyMap.set(item.date, item)
		})

		// Fill in the calendar days
		for (let i = startingDay; i < days.length; i++) {
			const dayOfMonth = i - startingDay + 1
			const dateStr = `${year}-${String(month + 1).padStart(2, '0')}-${String(dayOfMonth).padStart(2, '0')}`
			const historyItem = historyMap.get(dateStr)

			days[i] = {
				date: dayOfMonth,
				fullDate: dateStr,
				cardCount: historyItem?.card_count || 0,
				timeSpentMs: historyItem?.time_spent_ms || 0,
			}
		}

		// Fill in the days before the first day of the month
		for (let i = 0; i < startingDay; i++) {
			days[i] = null
		}

		// Group days into weeks
		const weeks = []
		for (let i = 0; i < days.length; i += 7) {
			weeks.push(days.slice(i, i + 7))
		}

		return weeks
	}

	// Calculate max cards per day for scaling intensity
	const maxCardsPerDay = () => {
		if (!historyQuery.data?.history || historyQuery.data.history.length === 0) return 1
		return Math.max(...historyQuery.data.history.map(item => item.card_count), 1)
	}

	// Calculate intensity of color based on cards count (0-4)
	const getIntensity = (cardCount: number) => {
		if (cardCount === 0) return 0
		const max = maxCardsPerDay()
		const percentage = cardCount / max

		if (percentage <= 0.25) return 1
		if (percentage <= 0.5) return 2
		if (percentage <= 0.75) return 3
		return 4
	}

	// Format month name
	const monthName = (month: number) => {
		return new Date(2000, month, 1).toLocaleString('default', { month: 'long' })
	}

	// Navigation functions
	const previousMonth = () => {
		startTransition(() => {
			const newMonth = selectedMonth() - 1
			if (newMonth < 0) {
				setSelectedMonth(11)
				setSelectedYear(selectedYear() - 1)
			} else {
				setSelectedMonth(newMonth)
			}
		})
	}

	const nextMonth = () => {
		startTransition(() => {
			const newMonth = selectedMonth() + 1
			if (newMonth > 11) {
				setSelectedMonth(0)
				setSelectedYear(selectedYear() + 1)
			} else {
				setSelectedMonth(newMonth)
			}
		})
	}

	return (
		<div class="container mx-auto px-4 py-6 pb-24 overflow-y-auto h-screen">
			<h1 class="text-2xl font-bold mb-6">Statistics</h1>

			{/* Summary Stats */}
			<div class="mb-8 bg-card rounded-xl p-4 border border-border">
				<h2 class="text-xl font-semibold mb-2">Study Summary</h2>
				<Show when={statsQuery.data?.study_stats} fallback={<div class="text-sm">Loading stats...</div>}>
					<div class="grid grid-cols-2 md:grid-cols-3 gap-4">
						<div class="flex flex-col">
							<span class="text-sm text-neutral-400">Total Cards</span>
							<span class="text-lg font-bold">{statsQuery.data?.study_stats.total_cards}</span>
						</div>
						<div class="flex flex-col">
							<span class="text-sm text-neutral-400">Today</span>
							<span class="text-lg font-bold">{statsQuery.data?.study_stats.cards_studied_today}</span>
						</div>
						<div class="flex flex-col">
							<span class="text-sm text-neutral-400">Study Days</span>
							<span class="text-lg font-bold">{statsQuery.data?.study_stats.study_days} days</span>
						</div>
						<div class="flex flex-col">
							<span class="text-sm text-neutral-400">Total Reviews</span>
							<span class="text-lg font-bold">{statsQuery.data?.study_stats.total_reviews}</span>
						</div>
						<div class="flex flex-col">
							<span class="text-sm text-neutral-400">Current Streak</span>
							<span class="text-lg font-bold">{statsQuery.data?.study_stats.streak_days} days</span>
						</div>
						<div class="flex flex-col">
							<span class="text-sm text-neutral-400">Total Study Time</span>
							<span class="text-lg font-bold">{statsQuery.data?.study_stats.total_time_studied}</span>
						</div>
					</div>
				</Show>
			</div>

			{/* Activity Graph */}
			<div class="h-full mb-8 bg-card rounded-xl p-4 border border-border">
				<div class="flex justify-between items-center mb-4">
					<button
						class="text-neutral-400 hover:text-white"
						onClick={previousMonth}
						disabled={isPending()}
					>
						<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
							<path fill-rule="evenodd"
										d="M12.707 5.293a1 1 0 010 1.414L9.414 10l3.293 3.293a1 1 0 01-1.414 1.414l-4-4a1 1 0 010-1.414l4-4a1 1 0 011.414 0z"
										clip-rule="evenodd" />
						</svg>
					</button>

					<h2 class="text-xl font-semibold">
						{monthName(selectedMonth())} {selectedYear()}
					</h2>

					<button
						class="text-neutral-400 hover:text-white"
						onClick={nextMonth}
						disabled={isPending()}
					>
						<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
							<path fill-rule="evenodd"
										d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z"
										clip-rule="evenodd" />
						</svg>
					</button>
				</div>

				<div class="overflow-x-auto">
					<div class="min-w-full">
						<div class="grid grid-cols-7 gap-1 text-center text-xs text-neutral-400 mb-2">
							<div>S</div>
							<div>M</div>
							<div>T</div>
							<div>W</div>
							<div>T</div>
							<div>F</div>
							<div>S</div>
						</div>

						<Show when={!historyQuery.isLoading}
									fallback={<div class="text-center py-4">Loading activity data...</div>}>
							<div class="grid grid-cols-1 gap-1">
								<For each={calendar()}>
									{(week) => (
										<div class="grid grid-cols-7 gap-1">
											<For each={week}>
												{(day) => (
													<Show when={day !== null} fallback={<div class="aspect-square" />}>
														<div
															class="aspect-square rounded-sm flex items-center justify-center text-xs relative"
															classList={{
																'bg-secondary': day?.cardCount === 0,
																'bg-green-700': getIntensity(day?.cardCount) === 1 && day?.cardCount > 0,
																'bg-green-600': getIntensity(day?.cardCount) === 2,
																'bg-green-500': getIntensity(day?.cardCount) === 3,
																'bg-green-400': getIntensity(day?.cardCount) === 4,
															}}
															title={`${day?.fullDate}: ${day?.cardCount} cards`}
														>
															{day?.date}
														</div>
													</Show>
												)}
											</For>
										</div>
									)}
								</For>
							</div>
						</Show>
					</div>
				</div>

				<div class="flex items-center mt-4 text-xs space-x-2">
					<div class="text-secondary-foreground">Goal not met</div>
					<div class="w-3 h-3 rounded-sm bg-secondary"></div>
					<div class="text-secondary-foreground">Less</div>
					<div class="flex space-x-1">
						<div class="w-3 h-3 rounded-sm bg-green-700"></div>
						<div class="w-3 h-3 rounded-sm bg-green-600"></div>
						<div class="w-3 h-3 rounded-sm bg-green-500"></div>
						<div class="w-3 h-3 rounded-sm bg-green-400"></div>
					</div>
					<div class="text-secondary-foreground">More</div>
				</div>
			</div>
		</div>
	)
}
