import { createSignal, Show, onMount, onCleanup, createEffect } from 'solid-js'
import { audioService } from '~/lib/audio-service'

interface AudioButtonProps {
	audioUrl: string
	secondaryAudioUrl?: string
	size?: 'sm' | 'md' | 'lg'
	label?: string
	type?: 'word' | 'example'
	delayBetweenAudios?: number
}

export default function AudioButton(props: AudioButtonProps) {
	const [isPlaying, setIsPlaying] = createSignal(false)
	const [primaryPlaying, setPrimaryPlaying] = createSignal(false)
	const [secondaryPlaying, setSecondaryPlaying] = createSignal(false)
	const type = props.type || 'word'

	// Update the overall playing state based on either audio playing
	createEffect(() => {
		setIsPlaying(primaryPlaying() || secondaryPlaying())
	})

	const handlePlay = (e: MouseEvent) => {
		e.stopPropagation()
		e.preventDefault()

		if (!props.audioUrl) return

		if (props.secondaryAudioUrl) {
			audioService.stopAll()
			audioService.playSequence(props.audioUrl, props.secondaryAudioUrl, props.delayBetweenAudios || 0)
		} else {
			audioService.toggleAudio(props.audioUrl, type)
		}
	}

	const [currentAudioUrl, setCurrentAudioUrl] = createSignal(props.audioUrl)
	const [currentSecondaryUrl, setCurrentSecondaryUrl] = createSignal(props.secondaryAudioUrl)
	let unregisterCallback: (() => void) | null = null

	const registerCallback = () => {
		if (unregisterCallback) {
			unregisterCallback()
		}

		const url = props.audioUrl
		if (!url) return

		setCurrentAudioUrl(url)
		setCurrentSecondaryUrl(props.secondaryAudioUrl)

		// Create array to hold all unregister functions
		const unregisterFunctions: Array<() => void> = []

		// Primary audio callback
		const componentId = `audio-button-${Math.random().toString(36).substring(2, 9)}`
		const primaryUnregister = audioService.registerStateCallback(
			componentId,
			url,
			(playing, callbackUrl) => {
				if (callbackUrl === url) {
					setPrimaryPlaying(playing)
				}
			}
		)
		unregisterFunctions.push(primaryUnregister)

		// Secondary audio callback (if available)
		if (props.secondaryAudioUrl) {
			const secondaryComponentId = `audio-button-secondary-${Math.random().toString(36).substring(2, 9)}`
			const secondaryUnregister = audioService.registerStateCallback(
				secondaryComponentId,
				props.secondaryAudioUrl,
				(playing, callbackUrl) => {
					if (callbackUrl === props.secondaryAudioUrl) {
						setSecondaryPlaying(playing)
					}
				}
			)
			unregisterFunctions.push(secondaryUnregister)
		}

		// Create a combined unregister function
		unregisterCallback = () => {
			unregisterFunctions.forEach(fn => fn())
		}
	}

	onMount(() => {
		registerCallback()

		onCleanup(() => {
			if (unregisterCallback) {
				unregisterCallback()
			}
		})
	})

	createEffect(() => {
		// Re-register callbacks when either primary or secondary audio URL changes
		if (props.audioUrl !== currentAudioUrl() || props.secondaryAudioUrl !== currentSecondaryUrl()) {
			registerCallback()
		}
	})

	const sizeClasses = {
		sm: 'w-5 h-5 text-xs',
		md: 'w-6 h-6 text-sm',
		lg: 'size-12 p-3.5 text-base',
	}

	const size = props.size || 'md'

	return (
		<button
			onClick={handlePlay}
			onMouseDown={(e) => e.stopPropagation()}
			onTouchStart={(e) => e.stopPropagation()}
			class={`${sizeClasses[size]} shrink-0 p-1 rounded-full bg-primary text-primary-foreground flex items-center justify-center hover:bg-primary/90 transition-colors relative z-[999]`}
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
					<path
						d="M8 6.82001V17.18C8 17.97 8.87 18.45 9.54 18.02L17.68 12.84C18.3 12.45 18.3 11.55 17.68 11.15L9.54 5.98001C8.87 5.55001 8 6.03001 8 6.82001Z"
						fill="currentColor" />
				</svg>
			</Show>
		</button>
	)
}
