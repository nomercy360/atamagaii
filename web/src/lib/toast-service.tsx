import { createSignal, JSX } from 'solid-js'
import { Toast } from '~/components/toast'
import { render } from 'solid-js/web'

type ToastType = 'success' | 'error' | 'info'

let toastSignal: ReturnType<typeof createSignal<JSX.Element | null>> | null = null

function createToastContainer() {
	if (!toastSignal) {
		const container = document.createElement('div')
		container.id = 'toast-container'
		document.body.appendChild(container)

		toastSignal = createSignal<JSX.Element | null>(null)

		render(() => toastSignal![0](), container)
	}
	return toastSignal
}

export function showToast(message: string, type: ToastType = 'info', duration: number = 3000) {
	const [_, setToast] = createToastContainer()

	setToast(
		<Toast
			message={message}
			type={type}
			duration={duration}
			onClose={() => setToast(null)}
		/>,
	)
}

showToast.success = (message: string, duration?: number) =>
	showToast(message, 'success', duration)

showToast.error = (message: string, duration?: number) =>
	showToast(message, 'error', duration)

showToast.info = (message: string, duration?: number) =>
	showToast(message, 'info', duration)
