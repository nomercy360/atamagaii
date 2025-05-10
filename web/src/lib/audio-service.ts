import { createSignal } from 'solid-js'

type AudioWithMetadata = {
  audio: HTMLAudioElement
  type: 'word' | 'example'
  url: string
}

// Create a singleton for global audio state management
const [activeAudios, setActiveAudios] = createSignal<AudioWithMetadata[]>([])
const [isPlaying, setIsPlaying] = createSignal(false)

// Create a callback registry for components that need to react to audio state changes
type AudioStateCallback = (playing: boolean, url: string) => void
const audioStateCallbacks = new Map<string, AudioStateCallback>()

/**
 * Audio service for managing audio playback across the application
 */
export const audioService = {
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
      item.audio.pause()
      item.audio.currentTime = 0
      console.log('Stopping audio:', item.url)
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
    
    const audio = new Audio(url)
    const audioItem: AudioWithMetadata = { audio, type, url }
    
    setActiveAudios([audioItem])
    
    // Update playing state and notify callbacks
    setIsPlaying(true)
    audioStateCallbacks.forEach((callback, key) => {
      if (key.includes(url)) {
        callback(true, url)
      }
    })
    
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
   * Play word audio followed by example audio with a delay
   */
  playSequence(wordUrl: string, exampleUrl?: string, delayMs: number = 300): Promise<void> {
    if (!wordUrl) {
      return Promise.reject(new Error('Word audio URL is empty'))
    }
    
    // Stop any currently playing audio
    this.stopAll()
    
    const wordAudio = new Audio(wordUrl)
    const wordItem: AudioWithMetadata = { audio: wordAudio, type: 'word', url: wordUrl }
    
    setActiveAudios([wordItem])
    setIsPlaying(true)
    
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
        
        // Schedule example audio to play after delay
        setTimeout(() => {
          // Only play if we're still in a valid state (e.g., user didn't navigate away)
          if (document.body.contains(wordAudio)) {
            const exampleAudio = new Audio(exampleUrl)
            const exampleItem: AudioWithMetadata = { audio: exampleAudio, type: 'example', url: exampleUrl }
            
            setActiveAudios([exampleItem])
            
            // Notify callbacks for example audio
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
              
              // Notify callbacks of failure
              audioStateCallbacks.forEach((callback, key) => {
                if (key.includes(exampleUrl)) {
                  callback(false, exampleUrl)
                }
              })
            })
          }
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
  }
}