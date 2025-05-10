import { Show } from 'solid-js'

interface ProgressBarProps {
  completed: number
  total: number
  showPercentage?: boolean
}

export default function ProgressBar(props: ProgressBarProps) {
  const percentage = () => {
    if (props.total === 0) return 0
    return Math.round((props.completed / props.total) * 100)
  }

  return (
    <div class="w-full mb-2">
      <div class="flex items-center justify-between mb-1">
        <div class="text-xs text-muted-foreground">
          <span class="font-medium">{props.completed}</span> / {props.total} cards
          <Show when={props.showPercentage}>
            <span class="ml-1">({percentage()}%)</span>
          </Show>
        </div>
      </div>
      <div class="w-full bg-muted rounded-full h-2.5">
        <div 
          class="bg-primary h-2.5 rounded-full transition-all duration-300" 
          style={{ width: `${percentage()}%` }}
        />
      </div>
    </div>
  )
}