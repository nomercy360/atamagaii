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
		<div class="w-full mb-2 flex items-center justify-center">
			<div class="w-[140px] bg-muted rounded-full h-1.5">
				<div
					class="bg-primary h-1.5 rounded-full transition-all duration-200"
					style={{ width: `${percentage()}%` }}
				/>
			</div>
		</div>
	)
}
