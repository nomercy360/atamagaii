import { createSignal, onCleanup, createEffect, Show } from 'solid-js'
import { Portal } from 'solid-js/web'
import { cn } from '~/lib/utils'

type ToastType = 'success' | 'error' | 'info'

type ToastProps = {
  message: string
  type?: ToastType
  duration?: number
  onClose?: () => void
}

export function Toast(props: ToastProps) {
  const [isVisible, setIsVisible] = createSignal(true)
  const [isExiting, setIsExiting] = createSignal(false)
  
  const duration = props.duration || 3000
  
  createEffect(() => {
    const timeout = setTimeout(() => {
      setIsExiting(true)
      
      // Add a small delay to allow for the exit animation
      setTimeout(() => {
        setIsVisible(false)
        if (props.onClose) props.onClose()
      }, 300)
    }, duration)
    
    onCleanup(() => clearTimeout(timeout))
  })
  
  const getTypeClasses = () => {
    switch (props.type) {
      case 'success':
        return 'bg-success text-success-foreground'
      case 'error':
        return 'bg-error text-error-foreground'
      case 'info':
      default:
        return 'bg-primary text-primary-foreground'
    }
  }
  
  return (
    <Show when={isVisible()}>
      <Portal>
        <div 
          class={cn(
            'fixed top-0 left-0 right-0 mx-auto p-4 max-w-xs z-50 transform transition-all duration-300',
            getTypeClasses(),
            isExiting() ? 'translate-y-0 opacity-0' : 'translate-y-4 opacity-100'
          )}
          style="margin-top: env(safe-area-inset-top, 0px);"
        >
          <div class="flex justify-between items-center">
            <p class="text-sm font-medium">{props.message}</p>
            <button 
              class="ml-2 text-current opacity-70"
              onClick={() => {
                setIsExiting(true)
                setTimeout(() => {
                  setIsVisible(false)
                  if (props.onClose) props.onClose()
                }, 300)
              }}
            >
              <svg 
                xmlns="http://www.w3.org/2000/svg" 
                width="16" 
                height="16" 
                viewBox="0 0 24 24" 
                fill="none" 
                stroke="currentColor" 
                stroke-width="2" 
                stroke-linecap="round" 
                stroke-linejoin="round"
              >
                <line x1="18" y1="6" x2="6" y2="18"></line>
                <line x1="6" y1="6" x2="18" y2="18"></line>
              </svg>
            </button>
          </div>
        </div>
      </Portal>
    </Show>
  )
}