import { createSignal, onCleanup, createEffect, Show } from 'solid-js'
import { Portal } from 'solid-js/web'
import { cn } from '~/lib/utils'

type ToastType = 'success' | 'error' | 'info'

type ToastProps = {
	message: string
	type?: ToastType
	duration?: number
	onClose?: () => void
}

export function Toast(props: ToastProps) {
	const [isVisible, setIsVisible] = createSignal(true)
	const [isExiting, setIsExiting] = createSignal(false)

	const duration = props.duration || 3000

	createEffect(() => {
		const timeout = setTimeout(() => {
			setIsExiting(true)

			// Add a small delay to allow for the exit animation
			setTimeout(() => {
				setIsVisible(false)
				if (props.onClose) props.onClose()
			}, 200)
		}, duration)

		onCleanup(() => clearTimeout(timeout))
	})

	const getTypeClasses = () => {
		switch (props.type) {
			default:
				return ''
		}
	}

	const renderIcon = () => {
		switch (props.type) {
			case 'success':
				return (
					<div class="w-5 h-5 rounded-full bg-green-500 flex items-center justify-center flex-shrink-0">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="12"
							height="12"
							viewBox="0 0 24 24"
							fill="none"
							stroke="white"
							stroke-width="3"
							stroke-linecap="round"
							stroke-linejoin="round"
						>
							<polyline points="20 6 9 17 4 12"></polyline>
						</svg>
					</div>
				)
			case 'error':
				return (
					<div class="w-5 h-5 rounded-full bg-red-500 flex items-center justify-center flex-shrink-0">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="12"
							height="12"
							viewBox="0 0 24 24"
							fill="none"
							stroke="white"
							stroke-width="3"
							stroke-linecap="round"
							stroke-linejoin="round"
						>
							<line x1="18" y1="6" x2="6" y2="18"></line>
							<line x1="6" y1="6" x2="18" y2="18"></line>
						</svg>
					</div>
				)
			default:
				return null
		}
	}

	return (
		<Show when={isVisible()}>
			<Portal>
				<div
					class={cn(
						'rounded-lg bg-secondary text-secondary-foreground border-secondary fixed top-0 left-0 right-0 mx-auto h-10 flex items-center justify-between px-4 max-w-xs z-50 transform transition-all duration-200',
						isExiting() ? 'translate-y-0 opacity-0' : 'translate-y-4 opacity-100',
					)}
					style="margin-top: env(safe-area-inset-top, 0px);"
				>
					<div class="flex items-center gap-2">
						{renderIcon()}
						<p class="text-sm font-medium">{props.message}</p>
					</div>
					<button
						class="ml-2 text-current opacity-70"
						onClick={() => {
							setIsExiting(true)
							setTimeout(() => {
								setIsVisible(false)
								if (props.onClose) props.onClose()
							}, 200)
						}}
					>
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="16"
							height="16"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
							stroke-linecap="round"
							stroke-linejoin="round"
						>
							<line x1="18" y1="6" x2="6" y2="18"></line>
							<line x1="6" y1="6" x2="18" y2="18"></line>
						</svg>
					</button>
				</div>
			</Portal>
		</Show>
	)
}
