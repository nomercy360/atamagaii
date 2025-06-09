import { createSignal } from 'solid-js'

type AudioWithMetadata = {
  audio: HTMLAudioElement
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
    audio.addEventListener(
      'canplaythrough',
      () => {
        console.log('Audio preloaded:', url)
      },
      { once: true },
    )

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
  registerStateCallback(
    id: string,
    url: string,
    callback: AudioStateCallback,
  ): () => void {
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
  playAudio(url: string): Promise<void> {
    if (!url) {
      console.error('Attempted to play audio with empty URL')
      return Promise.reject(new Error('Audio URL is empty'))
    }

    // Stop any currently playing audio
    this.stopAll()

    // Use cached audio if available, otherwise create new
    const audio = audioCache.get(url) || new Audio(url)
    const audioItem: AudioWithMetadata = { audio, url }

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
   * Toggle audio playback (play/pause)
   */
  toggleAudio(url: string): Promise<void> {
    // Check if this audio is already playing
    const isThisPlaying = activeAudios().some(
      item => item.url === url && item.audio.paused === false,
    )

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
      return this.playAudio(url)
    }
  },

  /**
   * Check if a specific audio URL is currently playing
   */
  isAudioPlaying(url: string): boolean {
    return activeAudios().some(item => item.url === url && !item.audio.paused)
  },

  /**
   * Get the global playing state
   */
  getIsPlaying(): boolean {
    return isPlaying()
  },
}
