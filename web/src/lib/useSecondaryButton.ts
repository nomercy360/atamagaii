export function useSecondaryButton() {
	return {
		setVisible: (text: string) => {
			window.Telegram.WebApp.SecondaryButton.setParams({
				is_visible: true,
				text_color: '#EEEEEE',
				color: '#333333',
				text,
			})
		},
		hide: () => {
			window.Telegram.WebApp.SecondaryButton.isVisible = false
		},
		enable: (text?: string) => {
			return window.Telegram.WebApp.SecondaryButton.setParams({
				is_active: true,
				is_visible: true,
				text_color: '#EEEEEE',
				color: '#333333',
				text,
			})
		},
		disable: (text?: string) => {
			return window.Telegram.WebApp.SecondaryButton.setParams({
				is_active: false,
				color: '#555555',
				text_color: '#AAAAAA',
				is_visible: true,
				text,
			})
		},
		setParams: (params: {
			text?: string
			isVisible?: boolean
			color?: string
			textColor?: string
			isEnabled?: boolean
		}) => {
			return window.Telegram.WebApp.SecondaryButton.setParams({
				is_visible: params.isVisible,
				text: params.text,
				color: params.color,
				text_color: params.textColor,
				is_active: params.isEnabled,
			})
		},
		onClick: (callback: () => void) => {
			window.Telegram.WebApp.SecondaryButton.onClick(callback)
		},
		offClick: (callback: () => void) => {
			window.Telegram.WebApp.SecondaryButton.offClick(callback)
		},
		showProgress: (leaveActive = false) => {
			window.Telegram.WebApp.SecondaryButton.showProgress(leaveActive)
		},
		hideProgress: () => {
			window.Telegram.WebApp.SecondaryButton.hideProgress()
		},
	}
}