import { JSX, splitProps, For, createMemo } from 'solid-js'

export interface FuriganaTextProps {
	text: string;
	class?: string;
	textSize?: 'xs' | 'sm' | 'base' | 'lg' | 'xl' | '2xl' | '3xl' | '4xl' | '5xl';
	rtClass?: string;
}

// Type for parsed segments
type FuriganaSegment =
	| { type: 'ruby'; base: string; text: string }
	| { type: 'text'; content: string }
	| { type: 'bold'; content: string };

// Helper function to parse the text into segments
const parseTextToSegments = (text: string): FuriganaSegment[] => {
	const segments: FuriganaSegment[] = []
	if (!text) {
		return segments
	}

	let lastIndex = 0
	// Regex: Match a non-greedy base text (one or more chars not '[')
	// followed by reading in brackets '[reading]'
	const regex = /([一-龯]+)\[([^\]]+)]/g // Non-greedy base: ([^\[]+?)
	let match

	while ((match = regex.exec(text)) !== null) {
		// Add preceding non-furigana text if any
		// This is text from the end of the last match to the start of the current match
		if (match.index > lastIndex) {
			segments.push({ type: 'text', content: text.substring(lastIndex, match.index) })
		}
		// Add furigana part
		segments.push({ type: 'ruby', base: match[1], text: match[2] })
		lastIndex = regex.lastIndex // Update lastIndex to the end of the current match
	}

	// Add any remaining non-furigana text from the end of the string
	if (lastIndex < text.length) {
		segments.push({ type: 'text', content: text.substring(lastIndex) })
	}

	return segments
}

// Constant map for rt sizes based on text size
const RT_SIZE_MAP = {
	'xs': 'text-[0.5rem]',
	'sm': 'text-[0.55rem]',
	'base': 'text-[0.6rem]',
	'lg': 'text-xs',
	'xl': 'text-xs',
	'2xl': 'text-xs',
	'3xl': 'text-sm',
	'4xl': 'text-sm',
	'5xl': 'text-base',
}

/**
 * FuriganaText component for displaying Japanese text with furigana readings
 *
 * Format should be: kanji[reading] (e.g., "会[あ]う", "明日[あした]")
 * Example: "明日[あした]、友[とも]だちに会[あ]います。"
 */
export default function FuriganaText(props: FuriganaTextProps): JSX.Element {
	const [local, others] = splitProps(props, ['text', 'class', 'textSize', 'rtClass'])

	const currentTextSize = createMemo(() => {
		return local.textSize || 'base'
	})

	const defaultRtClass = createMemo(() => {
		return RT_SIZE_MAP[currentTextSize()] || 'text-xs'
	})

	const finalRtClasses = createMemo(() => {
		return `${defaultRtClass()} ${local.rtClass || ''}`
	})

	// Parse the input text into segments. Reactive.
	const segments = createMemo(() => parseTextToSegments(local.text))

	return (
		<p
			class={`text-${currentTextSize()} leading-relaxed font-jp ${local.class || ''}`}
			{...others}
		>
			<For each={segments()}>
				{(segment) => {
					if (segment.type === 'ruby') {
						return (
							<ruby>
								{segment.base}
								<rt class={finalRtClasses()}>{segment.text}</rt>
							</ruby>
						)
					} else {
						// Plain text content, needs to be wrapped in <> for JSX fragment if it's just a string
						return <>{segment.content}</>
					}
				}}
			</For>
		</p>
	)
}
