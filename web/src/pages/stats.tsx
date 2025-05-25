import { createEffect, createSignal, For, Show } from 'solid-js'
import { useQuery } from '@tanstack/solid-query'
import { getStats, getStudyHistory, StudyHistoryItem, StudyStats } from '~/lib/api'
import { useTranslations } from '~/i18n/locale-context'

export default function Statistics() {
  const { t } = useTranslations()

  // Fetch stats data
  const statsQuery = useQuery(() => ({
    queryKey: ['study_stats'],
    queryFn: async () => {
      const { data, error } = await getStats()
      if (error) throw new Error(error)
      return data
    },
  }))

  // Fetch history data for last (100 days)
  const historyQuery = useQuery(() => ({
    queryKey: ['study_history'],
    queryFn: async () => {
      // Get the last 100 days of study history
      const { data, error } = await getStudyHistory(100)
      if (error) throw new Error(error)
      return data
    },
  }))

  // Create and prepare calendar data
  const [calendarData, setCalendarData] = createSignal<{
    weeks: Record<string, { date: Date; count: number }[]>;
    maxCount: number;
  }>({ weeks: {}, maxCount: 0 })

  // Calculate calendar data when history is loaded
  createEffect(() => {
    if (!historyQuery.data?.history) return

    // Generate dates for the last 120 days
    const endDate = new Date()
    const startDate = new Date()
    startDate.setDate(endDate.getDate() - 100)

    // Create map for history data
    const historyMap = new Map<string, StudyHistoryItem>()
    historyQuery.data.history.forEach(item => {
      historyMap.set(item.date, item)
    })

    // Prepare data structure for the calendar
    const weeks: Record<string, { date: Date; count: number }[]> = {}
    let maxCount = 0

    // Iterate through each day in the range
    const currentDate = new Date(startDate)
    while (currentDate <= endDate) {
      const dateStr = currentDate.toISOString().split('T')[0]
      const historyItem = historyMap.get(dateStr)
      const count = historyItem?.card_count || 0

      // Update max count
      if (count > maxCount) {
        maxCount = count
      }

      // Get week number (Sunday-based)
      const weekNum = getWeekNumber(currentDate)

      // Initialize week array if needed
      if (!weeks[weekNum]) {
        weeks[weekNum] = new Array(7).fill(null).map(() => ({ date: new Date(0), count: 0 }))
      }

      // Add day data to the week
      const dayOfWeek = currentDate.getDay()
      weeks[weekNum][dayOfWeek] = {
        date: new Date(currentDate),
        count,
      }

      // Move to next day
      currentDate.setDate(currentDate.getDate() + 1)
    }

    setCalendarData({ weeks, maxCount })
  })

  // Helper function to get week number (custom format "YYYY-WW")
  function getWeekNumber(date: Date): string {
    const startOfYear = new Date(date.getFullYear(), 0, 1)
    const pastDaysOfYear = (date.getTime() - startOfYear.getTime()) / 86400000
    const weekNumber = Math.ceil((pastDaysOfYear + startOfYear.getDay() + 1) / 7)
    return `${date.getFullYear()}-${weekNumber.toString().padStart(2, '0')}`
  }

  // Calculate intensity of color based on cards count (0-4)
  const getIntensity = (count: number) => {
    if (count === 0) return 0
    const max = calendarData().maxCount || 1
    const percentage = count / max

    if (percentage <= 0.25) return 1
    if (percentage <= 0.5) return 2
    if (percentage <= 0.75) return 3
    return 4
  }

  // Format date for tooltip
  const formatDate = (date: Date) => {
    return date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
  }

  // Get day names (abbreviated)
  const dayNames = ['S', 'M', 'T', 'W', 'T', 'F', 'S']

  return (
    <div class="container mx-auto px-4 py-6 pb-24 overflow-y-auto h-screen">
      <h1 class="text-2xl font-bold mb-6">{t('stats.title')}</h1>

      {/* Summary Stats */}
      <div class="mb-8 bg-card rounded-xl p-4 border border-border">
        <h2 class="text-lg font-semibold mb-2">{t('stats.summary')}</h2>
        <Show when={statsQuery.data?.study_stats} fallback={<div class="text-xs">{t('stats.loading')}</div>}>
          <div class="grid grid-cols-2 md:grid-cols-3 gap-4">
            <div class="flex flex-col">
              <span class="text-xs text-muted-foreground">{t('stats.totalCards')}</span>
              <span class="font-bold">{statsQuery.data?.study_stats.total_cards}</span>
            </div>
            <div class="flex flex-col">
              <span class="text-xs text-muted-foreground">{t('stats.today')}</span>
              <span class="font-bold">{statsQuery.data?.study_stats.cards_studied_today}</span>
            </div>
            <div class="flex flex-col">
              <span class="text-xs text-muted-foreground">{t('stats.studyDays')}</span>
              <span class="font-bold">{statsQuery.data?.study_stats.study_days} {t('stats.days')}</span>
            </div>
            <div class="flex flex-col">
              <span class="text-xs text-muted-foreground">{t('stats.totalReviews')}</span>
              <span class="font-bold">{statsQuery.data?.study_stats.total_reviews}</span>
            </div>
            <div class="flex flex-col">
              <span class="text-xs text-muted-foreground">{t('stats.currentStreak')}</span>
              <span class="font-bold">{statsQuery.data?.study_stats.streak_days} {t('stats.days')}</span>
            </div>
            <div class="flex flex-col">
              <span class="text-xs text-muted-foreground">{t('stats.totalStudyTime')}</span>
              <span class="font-bold">{statsQuery.data?.study_stats.total_time_studied}</span>
            </div>
          </div>
        </Show>
      </div>

      {/* GitHub-style Activity Heatmap */}
      <div class="mb-8 bg-card rounded-xl p-4 border border-border">
        <h2 class="text-xl font-semibold mb-4">{t('stats.activityHistory')}</h2>

        <Show when={!historyQuery.isLoading}
              fallback={<div class="text-center py-4">{t('stats.loadingActivity')}</div>}>
          <div class="flex overflow-auto pb-2">
            {/* Day labels (vertical) */}
            <div class="pr-2 pt-5">
              <div class="grid grid-rows-7 h-[126px] gap-[3px] text-center text-xs text-muted-foreground">
                <For each={dayNames}>
                  {(day, index) => (
                    <div class="flex items-center justify-end h-[15px]">
                      {index() % 2 === 0 ? day : ''}
                    </div>
                  )}
                </For>
              </div>
            </div>

            {/* Calendar grid */}
            <div class="overflow-auto">
              <div class="flex">
                <For each={Object.keys(calendarData().weeks).sort()}>
                  {(weekKey) => (
                    <div class="flex flex-col mr-[3px]">
                      {/* Show week number at top for every 4th week */}
                      <div class="text-xs text-center mb-1 h-4 text-muted-foreground">
                        {weekKey.endsWith('-01') || weekKey.endsWith('-05') ||
                        weekKey.endsWith('-09') || weekKey.endsWith('-13') ?
                          weekKey.split('-')[1] : ''}
                      </div>
                      <div class="grid grid-rows-7 gap-[3px]">
                        <For each={calendarData().weeks[weekKey]}>
                          {(day) => (
                            <div
                              class="w-[15px] h-[15px] rounded-sm"
                              classList={{
                                'bg-secondary': day.count === 0,
                                'bg-green-900': getIntensity(day.count) === 1 && day.count > 0,
                                'bg-green-700': getIntensity(day.count) === 2,
                                'bg-green-600': getIntensity(day.count) === 3,
                                'bg-green-500': getIntensity(day.count) === 4,
                              }}
                              title={`${formatDate(day.date)}: ${day.count} cards`}
                            />
                          )}
                        </For>
                      </div>
                    </div>
                  )}
                </For>
              </div>
            </div>
          </div>

          {/* Legend */}
          <div class="flex items-center mt-4 text-xs space-x-2">
            <div class="text-muted-foreground">{t('stats.noActivity')}</div>
            <div class="w-3 h-3 rounded-sm bg-secondary"></div>
            <div class="text-muted-foreground">{t('stats.less')}</div>
            <div class="flex space-x-1">
              <div class="w-3 h-3 rounded-sm bg-green-900"></div>
              <div class="w-3 h-3 rounded-sm bg-green-700"></div>
              <div class="w-3 h-3 rounded-sm bg-green-600"></div>
              <div class="w-3 h-3 rounded-sm bg-green-500"></div>
            </div>
            <div class="text-muted-foreground">{t('stats.more')}</div>
          </div>
        </Show>
      </div>
    </div>
  )
}
