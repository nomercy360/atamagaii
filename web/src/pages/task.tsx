import { createSignal, createResource, Show, createEffect, onMount, onCleanup } from 'solid-js'
import { apiRequest } from '~/lib/api'
import { useNavigate, useParams } from '@solidjs/router'
import { cn, hapticFeedback } from '~/lib/utils'
import Animation from '~/components/all-done-animation'
import { useMainButton } from '~/lib/useMainButton'
import { useSecondaryButton } from '~/lib/useSecondaryButton'
import ProgressBar from '~/components/progress-bar'
import AudioButton from '~/components/audio-button'
import TranscriptionText from '~/components/transcription-text'
import { useTranslations } from '~/i18n/locale-context'

// Task interfaces
interface TaskOption {
	a: string;
	b: string;
	c: string;
	d: string;
}

// Interface for vocab recall task content
interface VocabRecallContent {
	question: string;
	options: TaskOption;
}

// Interface for sentence translation task content
interface SentenceTranslationContent {
	sentence_ru: string;
}

// Interface for audio task content
interface AudioTaskContent {
	story: string;
	question: string;
	options: TaskOption;
	audio_url: string;
}

// Union type for all task content types
type TaskContent = VocabRecallContent | SentenceTranslationContent | AudioTaskContent;

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
	feedback: string | null;
}

export default function Task() {
	const navigate = useNavigate()
	const params = useParams()
	const mainButton = useMainButton()
	const secondaryButton = useSecondaryButton()
	const { t } = useTranslations()
	const [taskIndex, setTaskIndex] = createSignal(0)
	const [selectedOption, setSelectedOption] = createSignal<string | null>(null)
	const [translationInput, setTranslationInput] = createSignal('')
	const [isSubmitting, setIsSubmitting] = createSignal(false)
	const [showFeedback, setShowFeedback] = createSignal(false)
	const [feedbackType, setFeedbackType] = createSignal<'correct' | 'incorrect' | null>(null)
	const [feedbackMessage, setFeedbackMessage] = createSignal<string | null>(null)
	const [taskBuffer, setTaskBuffer] = createSignal<Task[]>([])
	const [needMoreTasks, setNeedMoreTasks] = createSignal(true)

	// Function to fetch tasks
	const fetchTasks = async () => {
		const deckId = params.deckId
		const endpoint = deckId ? `/tasks?limit=30&deck_id=${deckId}` : '/tasks?limit=30'

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

	// Initialize and clean up buttons
	onMount(() => {
		mainButton.hide()
		mainButton.onClick(handleSubmitTask)
		secondaryButton.hide()
		secondaryButton.onClick(handleTryAgain)
	})

	onCleanup(() => {
		mainButton.hide()
		mainButton.offClick(handleSubmitTask)
		secondaryButton.hide()
		secondaryButton.offClick(handleTryAgain)
	})

	// Update mainButton state based on current selection or input
	createEffect(() => {
		const option = selectedOption()
		const translation = translationInput()
		const task = currentTask()
		const isTaskSubmitting = isSubmitting()
		const showingFeedback = showFeedback()

		// If showing feedback for an incorrect answer in sentence translation
		if (showingFeedback && feedbackType() === 'incorrect' && task?.type === 'sentence_translation') {
			mainButton.enable(t('task.nextTask'))
		} else if (task && (task.type === 'vocab_recall' || task.type === 'audio') && option && !isTaskSubmitting && !showingFeedback) {
			// For vocab recall and audio tasks, need an option selected
			mainButton.enable(t('task.submit'))
		} else if (task && task.type === 'sentence_translation' && translation.trim() && !isTaskSubmitting && !showingFeedback) {
			// For sentence translation tasks, need non-empty input
			mainButton.enable(t('task.submit'))
		} else if (isTaskSubmitting) {
			mainButton.showProgress(true)
		} else if (showingFeedback && feedbackType() === 'correct') {
			// Feedback handling for correct answers is done in the submission handler
		} else if (task && (task.type === 'vocab_recall' || task.type === 'audio')) {
			mainButton.disable(t('task.selectAnswer'))
		} else if (task && task.type === 'sentence_translation') {
			mainButton.disable(t('task.enterTranslation'))
		} else {
			mainButton.hide()
		}
	})

	// Handle option selection
	const handleOptionSelect = (option: string) => {
		setSelectedOption(option)
	}

	// Handle task submission
	const handleSubmitTask = async () => {
		const task = currentTask()
		if (!task || isSubmitting()) return

		// If we're showing feedback for an incorrect answer and the user clicks "Next Task",
		// move to the next task
		if (showFeedback() && feedbackType() === 'incorrect') {
			setShowFeedback(false)
			setFeedbackMessage(null)
			setSelectedOption(null)
			setTranslationInput('')
			setTaskIndex(prev => prev + 1)
			secondaryButton.hide()
			return
		}

		// Don't proceed if already showing feedback
		if (showFeedback()) return

		// Get the appropriate response based on task type
		let response: string
		if (task.type === 'vocab_recall' || task.type === 'audio') {
			const option = selectedOption()
			if (!option) return
			response = option
		} else if (task.type === 'sentence_translation') {
			response = translationInput().trim()
			if (!response) return
		} else {
			return // Unsupported task type
		}

		setIsSubmitting(true)
		mainButton.showProgress(true)

		const { data, error } = await apiRequest<SubmitTaskResponse>('/tasks/submit', {
			method: 'POST',
			body: JSON.stringify({
				task_id: task.id,
				response: response,
			}),
		})

		if (error) {
			console.error('Failed to submit task:', error)
			setIsSubmitting(false)
			mainButton.hideProgress()
			return
		}

		// Show feedback
		setFeedbackType(data?.is_correct ? 'correct' : 'incorrect')
		setShowFeedback(true)
		hapticFeedback('impact', data?.is_correct ? 'light' : 'medium')
		setIsSubmitting(false)
		mainButton.hideProgress()

		// Set feedback message if available
		if (data?.feedback) {
			setFeedbackMessage(data.feedback)
		}

		// Update mainButton with color based on feedback
		if (data?.is_correct) {
			// Green for correct answers
			mainButton.setParams({
				text: t('task.correct'),
				color: '#4CAF50', // success green
				textColor: '#FFFFFF',
				isEnabled: false,
			})

			// For correct answers, move to next task after a delay
			setTimeout(() => {
				setShowFeedback(false)
				setFeedbackMessage(null)

				// Move to next task, resetting inputs
				setSelectedOption(null)
				setTranslationInput('')
				setTaskIndex(prev => prev + 1)
			}, 500)
		} else {
			// Red for incorrect answers
			mainButton.setParams({
				text: t('task.incorrect'),
				color: '#F44336', // error red
				textColor: '#FFFFFF',
				isEnabled: false,
			})

			// For incorrect answers, show the feedback and update buttons
			setTimeout(() => {
				mainButton.enable(t('task.nextTask'))

				// Show the secondary "Try Again" button
				secondaryButton.enable(t('task.tryAgain'))
			}, 500)
		}
	}

	// Handle trying the task again
	const handleTryAgain = () => {
		setShowFeedback(false)
		setFeedbackMessage(null)
		if (currentTask()?.type === 'sentence_translation') {
			// Keep the translation input text for editing
		} else {
			// Reset selection for multiple choice tasks
			setSelectedOption(null)
		}

		// Reset the main button state
		mainButton.setParams({
			text: t('task.submit'),
			color: '#101012', // primary blue
			textColor: '#FFFFFF',
			isEnabled: !!selectedOption() || (currentTask()?.type === 'sentence_translation' && !!translationInput().trim()),
		})

		// Hide the secondary button
		secondaryButton.hide()
	}

	// Check again for tasks
	const handleCheckAgain = () => {
		setNeedMoreTasks(true)
		refetchTasks()
	}

	return (
		<div class="bg-card container mx-auto px-4 py-10 max-w-md flex flex-col items-center h-screen overflow-y-auto">
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
			<Show when={taskBuffer().length > 0}>
				<ProgressBar
					total={taskBuffer().length}
					completed={taskIndex()}
				/>
			</Show>

			<div class="w-full flex-grow flex flex-col items-center justify-start">
				<Show when={currentTask()}>
					<div class="w-full flex flex-col items-center">
						<div class="w-full p-6 mb-6">
							{/* Vocab Recall Task */}
							<Show when={currentTask()?.type === 'vocab_recall'}>
								<h2 class="text-xl font-semibold mb-8 text-center">
									{(currentTask()?.content as VocabRecallContent)?.question}
								</h2>

								<div class="space-y-3">
									{Object.entries((currentTask()?.content as VocabRecallContent)?.options || {}).map(([key, value]) => (
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

							</Show>

							{/* Sentence Translation Task */}
							<Show when={currentTask()?.type === 'sentence_translation'}>
								<div class="space-y-6">
									<h2 class="text-start text-sm text-secondary-foreground font-semibold mb-2">
										{t('task.translateToJapanese')}
									</h2>
									<p class="text-lg italic">
										{(currentTask()?.content as SentenceTranslationContent)?.sentence_ru}
									</p>

									<div class="space-y-1">
										<label for="translation" class="text-xs font-medium">
											{t('task.yourTranslation')}
										</label>
										<textarea
											id="translation"
											value={translationInput()}
											onInput={(e) => setTranslationInput(e.target.value)}
											rows={3}
											disabled={showFeedback() && feedbackType() === 'correct'}
											ref={(el) => {
												if (el && !showFeedback()) {
													setTimeout(() => el.focus(), 100)
												}
											}}
											class={cn(
												'w-full p-3 bg-transparent text-3xl rounded-md transition-colors focus:outline-none text-center resize-none',
												showFeedback() && feedbackType() === 'correct'
													? 'bg-success/10 border-success'
													: showFeedback() && feedbackType() === 'incorrect'
														? 'bg-error/10 border-error'
														: 'border-border',
											)}
										/>
									</div>

									<Show when={showFeedback() && feedbackType() === 'incorrect' && feedbackMessage()}>
										<div class="p-4 bg-error/10 border border-error rounded-md text-sm">
											<h3 class="font-semibold mb-1">{t('task.feedback')}</h3>
											<p>{feedbackMessage()}</p>
										</div>
									</Show>
								</div>
							</Show>

							{/* Audio Task */}
							<Show when={currentTask()?.type === 'audio'}>
								<div class="space-y-6">
									<div class="flex items-center justify-center gap-2 mb-4">
										<span class="text-xs text-center">
											{t('task.listenToAudio')}
										</span>
										<AudioButton
											audioUrl={(currentTask()?.content as AudioTaskContent)?.audio_url}
											size="md"
										/>
									</div>

									<h2 class="text-xl font-semibold mb-4 text-center">
										<TranscriptionText
											class="font-semibold text-xl text-foreground"
											language="jp"
											rtClass="text-secondary-foreground font-semibold"
											text={(currentTask()?.content as AudioTaskContent)?.question} />
									</h2>

									<div class="space-y-3">
										{Object.entries((currentTask()?.content as AudioTaskContent)?.options || {}).map(([key, value]) => (
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
													<span>
														<TranscriptionText
															language="jp"
															class="text-lg font-normal text-foreground"
															rtClass="text-secondary-foreground font-semibold"
															text={value} />
													</span>
												</div>
											</button>
										))}
									</div>

								</div>
							</Show>
						</div>
					</div>
				</Show>

				<Show when={tasks.loading}>
					<div class="w-full flex flex-col items-center justify-center h-[300px]">
						<p class="text-muted-foreground">{t('task.loadingTasks')}</p>
					</div>
				</Show>

				<Show when={!tasks.loading && !currentTask()}>
					<div class="w-full flex flex-col items-center justify-center h-[400px] px-4">
						<Animation width={100} height={100} class="mb-2" />
						<p class="text-xl font-medium text-center mb-4">{t('task.noTasksAvailable')}</p>
						<p class="text-muted-foreground mb-4 text-center">
							{t('task.practiceToGenerateTasks')}
						</p>
						<button
							onClick={handleCheckAgain}
							class="mb-4 px-4 py-2 bg-primary text-primary-foreground rounded-md"
						>
							{t('task.checkAgain')}
						</button>
						<button
							onClick={() => navigate('/tasks')}
							class="mt-2 text-primary"
						>
							{t('task.backToTasks')}
						</button>
					</div>
				</Show>
			</div>
		</div>
	)
}
