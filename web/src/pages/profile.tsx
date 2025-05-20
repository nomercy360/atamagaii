import { createSignal, onMount, onCleanup, Show, For } from 'solid-js'
import { store, setUser } from '~/store'
import { useTranslations } from '~/i18n/locale-context'
import { useMainButton } from '~/lib/useMainButton'
import { updateUser } from '~/lib/api'
import { showToast } from '~/lib/toast-service'

export default function Profile() {
	const [userName, setUserName] = createSignal(store.user.name || '')
	const defaultTaskTypes = ['vocab_recall', 'sentence_translation', 'audio']
	const [maxTasksPerDay, setMaxTasksPerDay] = createSignal(store.user.settings?.max_tasks_per_day || 10)
	const [selectedTaskTypes, setSelectedTaskTypes] = createSignal<string[]>(
		store.user.settings?.task_types && store.user.settings.task_types.length > 0
			? store.user.settings.task_types
			: defaultTaskTypes
	)
	const [loading, setLoading] = createSignal(false)
	const { t } = useTranslations()
	const mainButton = useMainButton()

	const taskTypeOptions = [
		{ id: 'vocab_recall', label: t('profile.taskTypeOptions.vocabRecall') },
		{ id: 'sentence_translation', label: t('profile.taskTypeOptions.sentenceTranslation') },
		{ id: 'audio', label: t('profile.taskTypeOptions.audio') }
	]

	const toggleTaskType = (type: string) => {
		if (selectedTaskTypes().includes(type)) {
			// Don't allow removing the last task type
			if (selectedTaskTypes().length <= 1) return
			setSelectedTaskTypes((prev: string[]) => prev.filter((t: string) => t !== type))
		} else {
			setSelectedTaskTypes((prev: string[]) => [...prev, type])
		}
	}

	// Function to save user profile
	const saveProfile = async () => {
		setLoading(true)
		mainButton.showProgress(true)

		try {
			const { data, error } = await updateUser({
				name: userName(),
				settings: {
					max_tasks_per_day: maxTasksPerDay(),
					task_types: selectedTaskTypes()
				}
			})

			if (error) {
				showToast(error, 'error')
				return
			}

			if (data) {
				setUser(data)
				showToast(t('common.saved'), 'success')
			}
		} catch (e) {
			showToast(t('common.error'), 'error')
		} finally {
			setLoading(false)
			mainButton.hideProgress()
		}
	}

	// Setup main button when component mounts
	onMount(() => {
		mainButton.enable(t('common.save'))
		mainButton.onClick(saveProfile)
	})

	// Clean up main button when component unmounts
	onCleanup(() => {
		mainButton.hide()
		mainButton.offClick(saveProfile)
	})

	return (
		<div class="container mx-auto px-4 pb-24 pt-4 h-screen overflow-y-auto">
			<div class="flex flex-col items-center py-6">
				<div class="relative mb-6">
					<div class="size-24 rounded-full overflow-hidden bg-secondary">
						<Show when={store.user.avatar_url} fallback={
							<div class="w-full h-full flex items-center justify-center bg-primary">
                <span class="text-2xl text-primary-foreground font-bold">
                  {store.user.name?.substring(0, 1) || '?'}
                </span>
							</div>
						}>
							<img
								src={store.user.avatar_url}
								alt="Profile"
								class="w-full h-full object-cover"
							/>
						</Show>
					</div>
					<p class="h-7 bg-muted rounded-xl flex items-center justify-center w-full text-sm text-secondary-foreground text-center mt-4">
						@{store.user.username || 'Not set'}
					</p>
				</div>

				<div class="w-full">
					<div class="mb-6">
						<label class="block text-sm font-medium mb-1">
							{t('profile.displayName')}
						</label>
						<input
							type="text"
							value={userName()}
							onInput={(e) => setUserName(e.currentTarget.value)}
							class="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary text-foreground bg-background border-border"
							disabled={loading()}
							placeholder={t('profile.enterName')}
						/>
					</div>

					<div class="mb-6">
						<label class="block text-sm font-medium mb-1">
							{t('profile.maxTasksPerDay')}
						</label>
						<input
							type="number"
							min="1"
							max="50"
							value={maxTasksPerDay()}
							onInput={(e) => setMaxTasksPerDay(parseInt(e.currentTarget.value) || 10)}
							class="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary text-foreground bg-background border-border"
							disabled={loading()}
						/>
						<p class="text-xs text-muted-foreground mt-1">
							{t('profile.maxTasksHelp')}
						</p>
					</div>

					<div class="mb-6">
						<label class="block text-sm font-medium mb-1">
							{t('profile.taskTypes')}
						</label>
						<div class="grid grid-cols-1 gap-2 mt-2">
							<For each={taskTypeOptions}>
								{option => {
									const isSelected = () => selectedTaskTypes().includes(option.id);
									const isDisabled = () => loading() || (isSelected() && selectedTaskTypes().length <= 1);

									return (
										<button
											type="button"
											onClick={() => !isDisabled() && toggleTaskType(option.id)}
											class={`flex items-center justify-between p-3 rounded-md 
												${isSelected() ? 'bg-primary/10' : 'bg-card'} 
												${isDisabled() ? 'opacity-60' : ''}`}
											disabled={isDisabled()}
										>
											<span class="flex items-center">
												<span class="text-sm font-medium">
													{option.label}
												</span>
											</span>

											<span class={`w-5 h-5 flex items-center justify-center rounded-full 
												${isSelected() ? 'bg-primary text-primary-foreground' : 'border border-muted-foreground'}`}
											>
												{isSelected() && (
													<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
														<polyline points="20 6 9 17 4 12"></polyline>
													</svg>
												)}
											</span>
										</button>
									)
								}}
							</For>
						</div>
						<p class="text-xs text-muted-foreground mt-2">
							{t('profile.taskTypesHelp')}
						</p>
					</div>
				</div>
			</div>
		</div>
	)
}
