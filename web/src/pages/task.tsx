import { createSignal, createResource, Show, createEffect } from 'solid-js'
import { apiRequest } from '~/lib/api'
import { useNavigate, useParams } from '@solidjs/router'
import { cn, hapticFeedback } from '~/lib/utils'
import ProgressBar from '~/components/progress-bar'
import Animation from '~/components/all-done-animation'

// Task interfaces
interface TaskOption {
	a: string;
	b: string;
	c: string;
	d: string;
}

interface TaskContent {
	question: string;
	options: TaskOption;
}

interface Task {
	id: string;
	type: string;
	content: TaskContent;
	completed_at?: string;
	user_response?: string;
	is_correct?: boolean;
	created_at: string;
}

interface SubmitTaskResponse {
	task: Task;
	is_correct: boolean;
}

export default function Task() {
	const navigate = useNavigate()
	const params = useParams()
	const [taskIndex, setTaskIndex] = createSignal(0)
	const [selectedOption, setSelectedOption] = createSignal<string | null>(null)
	const [isSubmitting, setIsSubmitting] = createSignal(false)
	const [showFeedback, setShowFeedback] = createSignal(false)
	const [feedbackType, setFeedbackType] = createSignal<'correct' | 'incorrect' | null>(null)
	const [taskBuffer, setTaskBuffer] = createSignal<Task[]>([])
	const [needMoreTasks, setNeedMoreTasks] = createSignal(true)

	// Function to fetch tasks
	const fetchTasks = async () => {
		const deckId = params.deckId
		const endpoint = deckId ? `/tasks?limit=10&deck_id=${deckId}` : '/tasks?limit=10'
		
		const { data, error } = await apiRequest<Task[]>(endpoint)

		if (error) {
			console.error('Failed to fetch tasks:', error)
			return []
		}

		if (data && data.length > 0) {
			setTaskBuffer(prev => [...prev, ...data])
			return data
		} else {
			setNeedMoreTasks(false)
			return []
		}
	}

	// Resource for tasks
	const [tasks, { refetch: refetchTasks }] = createResource<Task[], boolean>(
		() => needMoreTasks() && taskBuffer().length === 0,
		async (shouldFetch) => {
			if (!shouldFetch) {
				return taskBuffer()
			}
			return fetchTasks()
		},
	)

	// Current task
	const currentTask = () => {
		const buffer = taskBuffer()
		const idx = taskIndex()
		if (buffer.length === 0 || idx >= buffer.length) return null
		return buffer[idx]
	}

	// Check if we need to fetch more tasks
	createEffect(() => {
		const buffer = taskBuffer()
		const idx = taskIndex()

		// If we're approaching the end of our buffer, fetch more tasks
		if (buffer.length > 0 && idx >= buffer.length - 2 && needMoreTasks()) {
			fetchTasks()
		}
	})

	// Handle option selection
	const handleOptionSelect = (option: string) => {
		setSelectedOption(option)
	}

	// Handle task submission
	const handleSubmitTask = async () => {
		const task = currentTask()
		const option = selectedOption()

		if (!task || !option || isSubmitting() || showFeedback()) return

		setIsSubmitting(true)

		const { data, error } = await apiRequest<SubmitTaskResponse>('/tasks/submit', {
			method: 'POST',
			body: JSON.stringify({
				task_id: task.id,
				response: option,
			}),
		})

		if (error) {
			console.error('Failed to submit task:', error)
			setIsSubmitting(false)
			return
		}

		// Show feedback
		setFeedbackType(data?.is_correct ? 'correct' : 'incorrect')
		setShowFeedback(true)
		hapticFeedback('impact', data?.is_correct ? 'light' : 'medium')
		setIsSubmitting(false)

		// Hide feedback after delay
		setTimeout(() => {
			setShowFeedback(false)

			// Move to next task
			setSelectedOption(null)
			setTaskIndex(prev => prev + 1)
		}, 500)
	}

	// Check again for tasks
	const handleCheckAgain = () => {
		setNeedMoreTasks(true)
		refetchTasks()
	}

	return (
		<div class="container mx-auto px-4 py-10 max-w-md flex flex-col items-center min-h-screen">
			{/* Feedback animation */}
			<Show when={showFeedback()}>
				<div
					class={`fixed top-4 left-1/2 z-50 flex items-center justify-center pointer-events-none transition-opacity duration-200 transform -translate-x-1/2 ${showFeedback() ? 'opacity-100' : 'opacity-0'}`}
				>
					<div
						class={`rounded-full size-6 flex items-center justify-center ${
							feedbackType() === 'correct' ? 'bg-success/90 text-info-foreground' : 'bg-error/90 text-error-foreground'
						}`}
					>
						{feedbackType() === 'correct' ? (
							<svg
								width="24"
								height="24"
								viewBox="0 0 24 24"
								fill="none"
								class="size-4"
								xmlns="http://www.w3.org/2000/svg"
							>
								<path d="M5 13L9 17L19 7" stroke="currentColor" stroke-width="2" stroke-linecap="round"
											stroke-linejoin="round" />
							</svg>
						) : (
							<svg
								width="24"
								height="24"
								viewBox="0 0 24 24"
								fill="none"
								xmlns="http://www.w3.org/2000/svg"
								class="size-4"
							>
								<path d="M6 16.5L18 7.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
								<path d="M6 7.5L18 16.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
							</svg>
						)}
					</div>
				</div>
			</Show>

			<div class="w-full flex-grow flex flex-col items-center justify-start">
				<Show when={currentTask()}>
					<div class="w-full flex flex-col items-center">
						<div class="w-full p-6 mb-6">
							<h2 class="text-xl font-semibold mb-4 text-center">{currentTask()?.content.question}</h2>

							<div class="space-y-3">
								{Object.entries(currentTask()?.content.options || {}).map(([key, value]) => (
									<button
										onClick={() => handleOptionSelect(key)}
										disabled={showFeedback()}
										class={cn(
											'w-full text-left p-3 rounded-md border transition-colors',
											showFeedback() && selectedOption() === key && feedbackType() === 'correct'
												? 'bg-success/20 border-success'
												: showFeedback() && selectedOption() === key && feedbackType() === 'incorrect'
													? 'bg-error/20 border-error'
													: selectedOption() === key
														? 'bg-secondary border-secondary'
														: 'border-border',
										)}
									>
										<div class="flex items-center">
											<div class={cn(
												'w-6 h-6 rounded-full flex items-center justify-center border mr-3',
												showFeedback() && selectedOption() === key && feedbackType() === 'correct'
													? 'bg-success border-success text-success-foreground'
													: showFeedback() && selectedOption() === key && feedbackType() === 'incorrect'
														? 'bg-error border-error text-error-foreground'
														: selectedOption() === key
															? 'bg-primary border-primary text-primary-foreground'
															: 'border-muted-foreground',
											)}>
												<span class="text-sm font-medium uppercase">{key}</span>
											</div>
											<span>{value}</span>
										</div>
									</button>
								))}
							</div>

							<button
								onClick={handleSubmitTask}
								disabled={!selectedOption() || isSubmitting() || showFeedback()}
								class={cn(
									'w-full mt-6 py-3 rounded-md font-medium transition-colors',
									showFeedback() && feedbackType() === 'correct'
										? 'bg-success text-success-foreground cursor-not-allowed'
										: showFeedback() && feedbackType() === 'incorrect'
											? 'bg-error text-error-foreground cursor-not-allowed'
											: selectedOption() && !isSubmitting()
												? 'bg-primary text-primary-foreground'
												: 'bg-muted text-muted-foreground cursor-not-allowed',
								)}
							>
								{isSubmitting()
									? 'Submitting...'
									: showFeedback() && feedbackType() === 'correct'
										? 'Correct!'
										: showFeedback() && feedbackType() === 'incorrect'
											? 'Incorrect!'
											: 'Submit Answer'}
							</button>
						</div>
					</div>
				</Show>

				<Show when={tasks.loading}>
					<div class="w-full flex flex-col items-center justify-center h-[300px]">
						<p class="text-muted-foreground">Loading tasks...</p>
					</div>
				</Show>

				<Show when={!tasks.loading && !currentTask()}>
					<div class="w-full flex flex-col items-center justify-center h-[400px] px-4">
						<Animation width={100} height={100} class="mb-2" />
						<p class="text-xl font-medium text-center mb-4">No tasks available!</p>
						<p class="text-muted-foreground mb-4 text-center">
							Practice cards in this deck to generate tasks. Tasks appear when cards move to review stage.
						</p>
						<button
							onClick={handleCheckAgain}
							class="mb-4 px-4 py-2 bg-primary text-primary-foreground rounded-md"
						>
							Check Again
						</button>
						<button
							onClick={() => navigate('/tasks')}
							class="mt-2 text-primary"
						>
							Back to tasks
						</button>
					</div>
				</Show>
			</div>
		</div>
	)
}