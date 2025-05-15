import { onMount, onCleanup, createSignal } from 'solid-js'
import { DotLottie } from '@lottiefiles/dotlottie-web'

interface AllDoneAnimationProps {
	class?: string;
	width?: number;
	height?: number;
}

export default function AllDoneAnimation(props: AllDoneAnimationProps) {
	const [canvasRef, setCanvasRef] = createSignal<HTMLCanvasElement | null>(null)
	let animation: DotLottie | undefined

	const initAnimation = () => {
		if (!canvasRef()) return

		animation = new DotLottie({
			autoplay: true,
			loop: true,
			canvas: canvasRef()!,
			src: '/all-done.json',
		})
	}

	onMount(() => {
		initAnimation()
	})

	onCleanup(() => {
		animation?.destroy()
	})

	return (
		<div class={`flex flex-col items-center justify-center ${props.class || ''}`}>
			<canvas
				ref={setCanvasRef}
				width={props.width || 200}
				height={props.height || 200}
				class="mb-4"
			/>
		</div>
	)
}
