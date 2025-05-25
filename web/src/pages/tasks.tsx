import { Show, For } from 'solid-js'
import { useNavigate } from '@solidjs/router'
import { getTasksPerDeck } from '~/lib/api'
import Animation from '~/components/all-done-animation'
import { useQuery } from '@tanstack/solid-query'
import { FlagIcon } from '~/pages/import-deck'
import { useTranslations } from '~/i18n/locale-context'

const TaskSkeleton = () => (
  <div class="w-full space-y-3">
    {Array(3).fill(0).map(() => (
      <div class="w-full flex items-center justify-between p-4 bg-card rounded-lg border border-border">
        <div class="flex-1">
          <div class="h-5 bg-muted rounded animate-pulse w-2/3 mb-2"></div>
          <div class="h-3 bg-muted rounded animate-pulse w-1/3"></div>
        </div>
        <div class="flex-shrink-0 h-5 w-5 bg-muted rounded animate-pulse"></div>
      </div>
    ))}
  </div>
)

export default function Tasks() {
  const navigate = useNavigate()
  const { t } = useTranslations()

  const tasksQuery = useQuery(() => ({
    queryKey: ['tasks'],
    queryFn: async () => {
      const { data, error } = await getTasksPerDeck()
      if (error) {
        console.error('Failed to fetch tasks:', error)
        return []
      }
      return data || []
    },
  }))

  const handleSelectDeck = (deckId: string) => {
    navigate(`/tasks/${deckId}`)
  }

  const handleRefresh = () => {
    tasksQuery.refetch()
  }

  return (
    <div class="container mx-auto px-4 py-10 max-w-md flex flex-col items-center min-h-screen">
      <Show when={!tasksQuery.isPending} fallback={<TaskSkeleton />}>
        <Show when={tasksQuery.data?.length}>
          <div class="w-full grid grid-cols-2 gap-2">
            <For each={tasksQuery.data}>
              {(item) => (
                <button
                  onClick={() => handleSelectDeck(item.deck_id)}
                  class="flex-row w-full flex items-center justify-between p-3 bg-card rounded-lg border border-border hover:bg-secondary/50 transition-colors"
                >
                  <div class="flex flex-col items-start justify-start">
                    <div class="flex-shrink-0 mb-2">
                      <FlagIcon code={item.language_code} clsName="size-5 rounded-full" />
                    </div>
                    <div class="flex-1">
                      <h3 class="text-sm font-medium text-left">{item.deck_name}</h3>
                      <div class="flex gap-1 text-xs text-info-foreground/80 mt-2">
                        <svg xmlns="http://www.w3.org/2000/svg"
                             height="24px"
                             class="size-4"
                             viewBox="0 -960 960 960"
                             width="24px"
                             fill="currentColor">
                          <path
                            d="M240-80q-33 0-56.5-23.5T160-160v-640q0-33 23.5-56.5T240-880h480q33 0 56.5 23.5T800-800v640q0 33-23.5 56.5T720-80H240Zm0-80h480v-640h-80v245q0 12-10 17.5t-20-.5l-49-30q-10-6-20.5-6t-20.5 6l-49 30q-10 6-20.5.5T440-555v-245H240v640Zm0 0v-640 640Zm200-395q0 12 10.5 17.5t20.5-.5l49-30q10-6 20.5-6t20.5 6l49 30q10 6 20 .5t10-17.5q0 12-10 17.5t-20-.5l-49-30q-10-6-20.5-6t-20.5 6l-49 30q-10 6-20.5.5T440-555Z" />
                        </svg>
                        {item.total_tasks}
                      </div>
                    </div>
                  </div>
                  <div class="flex-shrink-0">
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      width="24"
                      height="24"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      class="h-5 w-5 text-muted-foreground"
                    >
                      <path d="m9 18 6-6-6-6" />
                    </svg>
                  </div>
                </button>
              )}
            </For>
          </div>
        </Show>

        <Show when={!tasksQuery.data || tasksQuery.data.length === 0}>
          <div class="w-full flex flex-col items-center justify-center h-[400px] px-4">
            <Animation width={100} height={100} class="mb-2" src="/study-more.json" />
            <p class="text-xl font-medium text-center mb-4">{t('task.noTasksAvailableDecks')}</p>
            <p class="text-muted-foreground mb-4 text-center">
              {t('task.practiceToGenerateTasksDecks')}
            </p>
            <button
              onClick={handleRefresh}
              class="text-sm mb-4 px-4 py-2 bg-primary text-primary-foreground rounded-md"
            >
              {t('task.checkAgainDecks')}
            </button>
            <button
              onClick={() => navigate('/')}
              class="text-foreground text-sm"
            >
              {t('task.backToDecks')}
            </button>
          </div>
        </Show>
      </Show>
    </div>
  )
}
