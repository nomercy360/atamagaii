import { createSignal } from 'solid-js'

type AudioWithMetadata = {
	audio: HTMLAudioElement
	type: 'word' | 'example'
	url: string
}

// Create a singleton for global audio state management
const [activeAudios, setActiveAudios] = createSignal<AudioWithMetadata[]>([])
const [isPlaying, setIsPlaying] = createSignal(false)

// Audio cache to store preloaded audio elements
const audioCache = new Map<string, HTMLAudioElement>()

// Create a callback registry for components that need to react to audio state changes
type AudioStateCallback = (playing: boolean, url: string) => void
const audioStateCallbacks = new Map<string, AudioStateCallback>()

/**
 * Audio service for managing audio playback across the application
 */
export const audioService = {
	/**
	 * Preload an audio file without playing it
	 */
	preloadAudio(url: string): void {
		if (!url) {
			console.log('Attempted to preload audio with empty URL')
			return
		}

		// If already in cache, don't recreate
		if (audioCache.has(url)) {
			return
		}

		const audio = new Audio(url)

		// Listen for the canplaythrough event to know when it's loaded
		audio.addEventListener('canplaythrough', () => {
			console.log('Audio preloaded:', url)
		}, { once: true })

		// Load the audio file
		audio.load()

		// Store in cache
		audioCache.set(url, audio)
	},

	/**
	 * Preload multiple audio files
	 */
	preloadMultipleAudio(urls: string[]): void {
		urls.forEach(url => {
			if (url) this.preloadAudio(url)
		})
	},
	/**
	 * Register a callback to be notified of audio state changes for a specific URL
	 */
	registerStateCallback(id: string, url: string, callback: AudioStateCallback): () => void {
		const callbackId = `${id}:${url}`
		audioStateCallbacks.set(callbackId, callback)

		// Return a function to unregister the callback
		return () => {
			audioStateCallbacks.delete(callbackId)
		}
	},

	/**
	 * Stop all currently playing audio
	 */
	stopAll(): void {
		// Save URLs before stopping to ensure we can notify the correct callbacks
		const playingUrls = activeAudios().map(item => item.url)

		activeAudios().forEach(item => {
			try {
				item.audio.pause()
				item.audio.currentTime = 0
				console.log('Stopping audio:', item.url)
			} catch (error) {
				console.error('Error stopping audio:', error, item.url)
			}
		})

		// Clear active audios and set global playing state to false
		setActiveAudios([])
		setIsPlaying(false)

		// Notify all callbacks for previously playing audios that audio has stopped
		playingUrls.forEach(url => {
			audioStateCallbacks.forEach((callback, key) => {
				if (key.includes(url)) {
					console.log(`Notifying callback that ${url} has stopped`)
					callback(false, url)
				}
			})
		})
	},

	/**
	 * Play a single audio file
	 */
	playAudio(url: string, type: 'word' | 'example' = 'word'): Promise<void> {
		if (!url) {
			console.error('Attempted to play audio with empty URL')
			return Promise.reject(new Error('Audio URL is empty'))
		}

		// Stop any currently playing audio
		this.stopAll()

		// Create a fresh audio instance to avoid potential issues with cached instances
		let audio: HTMLAudioElement;
		if (audioCache.has(url)) {
			// If in cache, create a clone to avoid state issues
			const cachedAudio = audioCache.get(url)!;
			audio = new Audio(url);
			// Use the cached audio to handle preloading but avoid state issues
			if (cachedAudio.readyState >= 2) {
				console.log('Using preloaded audio from cache:', url);
			}
		} else {
			audio = new Audio(url);
			audioCache.set(url, audio);
		}

		const audioItem: AudioWithMetadata = { audio, type, url }
		setActiveAudios([audioItem])

		// Update playing state and notify callbacks
		setIsPlaying(true)
		audioStateCallbacks.forEach((callback, key) => {
			if (key.includes(url)) {
				callback(true, url)
			}
		})

		// Reset to start in case it was played before
		audio.currentTime = 0

		// Set up onended handler to update state when audio finishes
		audio.onended = () => {
			setActiveAudios(audios => audios.filter(a => a.audio !== audio))

			// If this was the last audio, update the playing state
			if (activeAudios().length === 0) {
				setIsPlaying(false)

				// Notify callbacks that this audio has ended
				audioStateCallbacks.forEach((callback, key) => {
					if (key.includes(url)) {
						callback(false, url)
					}
				})
			}
		}

		return audio.play().catch(error => {
			console.error('Error playing audio:', error)
			setActiveAudios(audios => audios.filter(a => a.audio !== audio))
			setIsPlaying(false)

			// Notify callbacks of failure
			audioStateCallbacks.forEach((callback, key) => {
				if (key.includes(url)) {
					callback(false, url)
				}
			})

			throw error
		})
	},

	/**
	 * Play word audio followed by example audio
	 */
	playSequence(wordUrl: string, exampleUrl?: string, delayMs: number = 0): Promise<void> {
		if (!wordUrl) {
			return Promise.reject(new Error('Word audio URL is empty'))
		}

		// Stop any currently playing audio
		this.stopAll()

		// Create fresh audio instances to avoid potential issues with cached instances
		let wordAudio: HTMLAudioElement;
		if (audioCache.has(wordUrl)) {
			// If in cache, create a clone to avoid state issues
			const cachedAudio = audioCache.get(wordUrl)!;
			wordAudio = new Audio(wordUrl);
			// Use the cached audio to handle preloading but avoid state issues
			if (cachedAudio.readyState >= 2) {
				console.log('Using preloaded word audio from cache:', wordUrl);
			}
		} else {
			wordAudio = new Audio(wordUrl);
			audioCache.set(wordUrl, wordAudio);
		}

		const wordItem: AudioWithMetadata = { audio: wordAudio, type: 'word', url: wordUrl }
		setActiveAudios([wordItem])
		setIsPlaying(true)

		// Reset to start in case it was played before
		wordAudio.currentTime = 0

		// Notify callbacks for word audio
		audioStateCallbacks.forEach((callback, key) => {
			if (key.includes(wordUrl)) {
				callback(true, wordUrl)
			}
		})

		// If we have an example audio, set up the chain to play it after the word audio
		if (exampleUrl) {
			wordAudio.onended = () => {
				// Update the active audio list to remove the word audio
				setActiveAudios(audios => audios.filter(a => a.audio !== wordAudio))

				// Notify callbacks that word audio has ended
				audioStateCallbacks.forEach((callback, key) => {
					if (key.includes(wordUrl)) {
						callback(false, wordUrl)
					}
				})

				setTimeout(() => {
					// Create a fresh audio instance for example audio
					let exampleAudio: HTMLAudioElement;
					if (audioCache.has(exampleUrl)) {
						const cachedAudio = audioCache.get(exampleUrl)!;
						exampleAudio = new Audio(exampleUrl);
						if (cachedAudio.readyState >= 2) {
							console.log('Using preloaded example audio from cache:', exampleUrl);
						}
					} else {
						exampleAudio = new Audio(exampleUrl);
						audioCache.set(exampleUrl, exampleAudio);
					}

					const exampleItem: AudioWithMetadata = { audio: exampleAudio, type: 'example', url: exampleUrl }
					setActiveAudios([exampleItem])
					exampleAudio.currentTime = 0

					audioStateCallbacks.forEach((callback, key) => {
						if (key.includes(exampleUrl)) {
							callback(true, exampleUrl)
						}
					})

					exampleAudio.onended = () => {
						setActiveAudios(audios => audios.filter(a => a.audio !== exampleAudio))

						// If this was the last audio, update the playing state
						if (activeAudios().length === 0) {
							setIsPlaying(false)
						}

						// Notify callbacks that example audio has ended
						audioStateCallbacks.forEach((callback, key) => {
							if (key.includes(exampleUrl)) {
								callback(false, exampleUrl)
							}
						})
					}

					exampleAudio.play().catch(error => {
						console.error('Error playing example audio:', error)
						setActiveAudios(audios => audios.filter(a => a.audio !== exampleAudio))
						setIsPlaying(false)

						// Notify callbacks of failure
						audioStateCallbacks.forEach((callback, key) => {
							if (key.includes(exampleUrl)) {
								callback(false, exampleUrl)
							}
						})
					})
				}, delayMs)
			}
		} else {
			// If no example audio, just set up onended for the word audio
			wordAudio.onended = () => {
				setActiveAudios(audios => audios.filter(a => a.audio !== wordAudio))

				// If this was the last audio, update the playing state
				if (activeAudios().length === 0) {
					setIsPlaying(false)
				}

				// Notify callbacks that audio has ended
				audioStateCallbacks.forEach((callback, key) => {
					if (key.includes(wordUrl)) {
						callback(false, wordUrl)
					}
				})
			}
		}

		return wordAudio.play().catch(error => {
			console.error('Error playing word audio:', error)
			setActiveAudios(audios => audios.filter(a => a.audio !== wordAudio))
			setIsPlaying(false)

			// Notify callbacks of failure
			audioStateCallbacks.forEach((callback, key) => {
				if (key.includes(wordUrl)) {
					callback(false, wordUrl)
				}
			})

			throw error
		})
	},

	/**
	 * Toggle audio playback (play/pause)
	 */
	toggleAudio(url: string, type: 'word' | 'example' = 'word'): Promise<void> {
		// Check if this audio is already playing
		const isThisPlaying = activeAudios().some(item => item.url === url && item.audio.paused === false)

		if (isThisPlaying) {
			// If playing, stop it
			this.stopAll()

			// Make sure to explicitly notify callbacks for this specific URL
			audioStateCallbacks.forEach((callback, key) => {
				if (key.includes(url)) {
					callback(false, url)
				}
			})

			return Promise.resolve()
		} else {
			// If not playing, start it
			return this.playAudio(url, type)
		}
	},

	/**
	 * Check if a specific audio URL is currently playing
	 */
	isAudioPlaying(url: string): boolean {
		return activeAudios().some(item => item.url === url && item.audio.paused === false)
	},

	/**
	 * Get the global playing state
	 */
	getIsPlaying(): boolean {
		return isPlaying()
	},
}
