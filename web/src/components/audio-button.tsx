import { createSignal, Show } from 'solid-js'

interface AudioButtonProps {
  audioUrl: string
  size?: 'sm' | 'md' | 'lg'
  label?: string
}

export default function AudioButton(props: AudioButtonProps) {
  const [isPlaying, setIsPlaying] = createSignal(false)

  const handlePlay = (e: MouseEvent) => {
    e.stopPropagation()
    e.preventDefault()

    if (isPlaying()) {
      setIsPlaying(false)
      return
    }

    const audio = new Audio(props.audioUrl)
    setIsPlaying(true)

    audio.onended = () => setIsPlaying(false)
    audio.play().catch(error => {
      console.error('Error playing audio:', error)
      setIsPlaying(false)
    })
  }

  const sizeClasses = {
    sm: 'w-5 h-5 text-xs',
    md: 'w-6 h-6 text-sm',
    lg: 'w-8 h-8 text-base'
  }

  const size = props.size || 'md'

  return (
    <button
      onClick={handlePlay}
      onMouseDown={(e) => e.stopPropagation()}
      onTouchStart={(e) => e.stopPropagation()}
      class={`${sizeClasses[size]} p-1 rounded-full bg-primary text-primary-foreground flex items-center justify-center hover:bg-primary/90 transition-colors relative z-10`}
      aria-label={props.label || 'Play audio'}
      disabled={!props.audioUrl}
      type="button"
    >
      <Show when={!isPlaying()} fallback={
        <svg class="w-full h-full" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
          <rect x="7" y="7" width="3" height="10" rx="1.5" fill="currentColor" />
          <rect x="14" y="7" width="3" height="10" rx="1.5" fill="currentColor" />
        </svg>
      }>
        <svg class="w-full h-full" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M8 6.82001V17.18C8 17.97 8.87 18.45 9.54 18.02L17.68 12.84C18.3 12.45 18.3 11.55 17.68 11.15L9.54 5.98001C8.87 5.55001 8 6.03001 8 6.82001Z" fill="currentColor" />
        </svg>
      </Show>
    </button>
  )
}
