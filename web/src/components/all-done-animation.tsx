import { onMount, onCleanup, createSignal } from 'solid-js'
import { DotLottie } from '@lottiefiles/dotlottie-web'

interface AnimationProps {
	class?: string;
	width?: number;
	height?: number;
	src?: string;
	loop?: boolean;
	canvasClass?: string;
}

export default function Animation(props: AnimationProps) {
	const [canvasRef, setCanvasRef] = createSignal<HTMLCanvasElement | null>(null)
	const [containerRef, setContainerRef] = createSignal<HTMLDivElement | null>(null)
	let animation: DotLottie | undefined

	const devicePixelRatio = window.devicePixelRatio || 1
	const width = props.width || 200
	const height = props.height || 200
	const src = props.src || '/all-done.json'
	const loop = props.loop !== undefined ? props.loop : true

	const initAnimation = () => {
		if (!canvasRef()) return

		const canvas = canvasRef()!
		
		// Set canvas dimensions to be higher resolution
		canvas.width = width * devicePixelRatio
		canvas.height = height * devicePixelRatio
		
		// Scale the rendering context
		const ctx = canvas.getContext('2d')
		if (ctx) {
			ctx.scale(devicePixelRatio, devicePixelRatio)
		}
		
		// Apply CSS size for display
		canvas.style.width = `${width}px`
		canvas.style.height = `${height}px`

		animation = new DotLottie({
			autoplay: true,
			loop: loop,
			canvas: canvas,
			src: src
		})
	}

	onMount(() => {
		initAnimation()
	})

	onCleanup(() => {
		animation?.destroy()
	})

	return (
		<div 
			ref={setContainerRef} 
			class={`flex flex-col items-center justify-center ${props.class || ''}`}
		>
			<canvas
				ref={setCanvasRef}
				class={props.canvasClass || "mb-4"}
				style={{"image-rendering": "crisp-edges"}}
			/>
		</div>
	)
}
