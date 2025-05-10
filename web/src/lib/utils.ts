import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs))
}

export function hapticFeedback(
	type: 'impact' | 'notification' | 'selection',
	style?: 'light' | 'medium' | 'heavy' | 'soft' | 'rigid' | 'error' | 'success' | 'warning',
): void {
	try {
		// Check if we're in a Telegram WebApp environment
		if (typeof window !== 'undefined' && window.Telegram?.WebApp?.HapticFeedback) {
			const haptic = window.Telegram.WebApp.HapticFeedback

			switch (type) {
				case 'impact':
					if (style && ['light', 'medium', 'heavy', 'soft', 'rigid'].includes(style)) {
						haptic.impactOccurred(style as 'light' | 'medium' | 'heavy' | 'soft' | 'rigid')
					} else {
						// Default to light impact if no valid style provided
						haptic.impactOccurred('light')
					}
					break

				case 'notification':
					if (style && ['error', 'success', 'warning'].includes(style)) {
						haptic.notificationOccurred(style as 'error' | 'success' | 'warning')
					} else {
						// Default to success if no valid style provided
						haptic.notificationOccurred('success')
					}
					break

				case 'selection':
					haptic.selectionChanged()
					break

				default:
					console.warn('Invalid haptic feedback type')
			}
		}
	} catch (error) {
		console.error('Error triggering haptic feedback:', error)
	}
}

