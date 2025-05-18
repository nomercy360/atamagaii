import { createSignal, Show } from 'solid-js'
import { store } from '~/store'
import { useTranslations } from '~/i18n/locale-context'

export default function Profile() {
	const [userName, setUserName] = createSignal(store.user.name || '')
	const [saved, setSaved] = createSignal(false)
	const { t } = useTranslations()

	const saveProfile = () => {
		setSaved(true)
		setTimeout(() => setSaved(false), 2000)
	}

	return (
		<div class="container mx-auto px-4 pb-24 pt-4 h-screen overflow-y-auto">
			<div class="flex flex-col items-center py-6">
				<div class="relative mb-6">
					<div class="w-28 h-28 rounded-full overflow-hidden bg-secondary">
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
				</div>

				<div class="w-full">
					<div class="mb-6">
						<label class="block text-sm font-medium mb-2">
							{t('profile.username')}
						</label>
						<div class="text-lg p-4 rounded-md border border-border bg-card">
							{store.user.username || 'Not set'}
						</div>
					</div>

					<div class="mb-6">
						<label class="block text-sm font-medium mb-2">
							{t('profile.displayName')}
						</label>
						<input
							type="text"
							value={userName()}
							onInput={(e) => setUserName(e.target.value)}
							class="w-full p-4 rounded-md border border-border bg-card focus:outline-none focus:ring-2 focus:ring-primary"
						/>
					</div>

					<div class="mb-6">
						<label class="block text-sm font-medium mb-2">
							{t('profile.level')}
						</label>
						<div class="text-lg p-4 rounded-md border border-border bg-card">
							{store.user.level || 'N5'}
						</div>
					</div>

					<div class="mb-6">
						<label class="block text-sm font-medium mb-2">
							{t('profile.points')}
						</label>
						<div class="text-lg p-4 rounded-md border border-border bg-card">
							{store.user.points || '0'}
						</div>
					</div>

					<button
						onClick={saveProfile}
						class="w-full p-4 bg-primary text-primary-foreground rounded-md font-medium mt-4"
					>
						{saved() ? t('common.saved') : t('common.save')}
					</button>
				</div>
			</div>
		</div>
	)
}
