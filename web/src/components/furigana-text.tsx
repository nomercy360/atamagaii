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
	| { type: 'bold'; content: string; segments?: FuriganaSegment[] };

// Helper function to parse the text into segments
const parseTextToSegments = (text: string): FuriganaSegment[] => {
	const segments: FuriganaSegment[] = []
	if (!text) {
		return segments
	}

	// Step 1: Handle HTML tags like <b> separately to prevent interference with furigana parsing
	const htmlTags = new Map<string, { type: string, content: string }>()
	let tagCounter = 0

	// Replace bold tags with placeholders
	let processedText = text.replace(/<b>(.+?)<\/b>/g, (_, content) => {
		const placeholder = `__HTML_TAG_${tagCounter}__`
		htmlTags.set(placeholder, { type: 'bold', content })
		tagCounter++
		return placeholder
	})

	// Step 2: Process furigana notation in the text
	let lastIndex = 0
	// Match kanji characters followed by readings in brackets
	const furiganaRegex = /([一-龯]+)\[([^\]]+)]/g
	let match

	while ((match = furiganaRegex.exec(processedText)) !== null) {
		// Add text before the current furigana match
		if (match.index > lastIndex) {
			const precedingText = processedText.substring(lastIndex, match.index)
			processPlainTextWithTags(precedingText, segments, htmlTags)
		}

		// Add the furigana segment
		segments.push({ type: 'ruby', base: match[1], text: match[2] })
		lastIndex = furiganaRegex.lastIndex
	}

	// Add any remaining text after the last furigana match
	if (lastIndex < processedText.length) {
		const remainingText = processedText.substring(lastIndex)
		processPlainTextWithTags(remainingText, segments, htmlTags)
	}

	return segments
}

// Handle plain text that might contain HTML tag placeholders
const processPlainTextWithTags = (
	text: string,
	segments: FuriganaSegment[],
	htmlTags: Map<string, { type: string, content: string }>
) => {
	const tagRegex = /__HTML_TAG_(\d+)__/g
	let lastIndex = 0
	let match

	while ((match = tagRegex.exec(text)) !== null) {
		// Add plain text before the tag placeholder
		if (match.index > lastIndex) {
			segments.push({
				type: 'text',
				content: text.substring(lastIndex, match.index)
			})
		}

		// Process the tag
		const placeholder = match[0]
		const tagInfo = htmlTags.get(placeholder)

		if (tagInfo) {
			if (tagInfo.type === 'bold') {
				// For bold tags, check if there's furigana inside
				if (tagInfo.content.includes('[') && tagInfo.content.match(/([一-龯]+)\[([^\]]+)]/)) {
					// Parse the bold content separately to handle nested furigana
					const boldSegments = parseTextToSegments(tagInfo.content)
					segments.push({
						type: 'bold',
						content: '', // Empty as we'll use the segments instead
						segments: boldSegments
					})
				} else {
					// Regular bold text without furigana
					segments.push({
						type: 'bold',
						content: tagInfo.content
					})
				}
			}
		}

		lastIndex = match.index + placeholder.length
	}

	// Add any remaining text
	if (lastIndex < text.length) {
		segments.push({
			type: 'text',
			content: text.substring(lastIndex)
		})
	}
}

// Size map for furigana rt elements
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
 * Supports both furigana notation and HTML tags:
 * - Furigana format: kanji[reading] (e.g., "会[あ]う", "明日[あした]")
 * - HTML tags: Currently supports <b> for bold text
 * - Can handle nested tags and furigana
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

	// Parse the input text into segments
	const segments = createMemo(() => parseTextToSegments(local.text))

	// Recursive renderer for segments (to handle nested structures)
	const renderSegment = (segment: FuriganaSegment): JSX.Element => {
		if (segment.type === 'ruby') {
			return (
				<ruby>
					{segment.base}
					<rt class={finalRtClasses()}>{segment.text}</rt>
				</ruby>
			)
		} else if (segment.type === 'bold') {
			if (segment.segments) {
				// If it has nested segments, render those within the bold context
				return (
					<span class="font-bold">
						<For each={segment.segments}>
							{(nestedSegment) => renderSegment(nestedSegment)}
						</For>
					</span>
				)
			}
			// Simple bold text
			return <span class="font-bold">{segment.content}</span>
		} else {
			// Plain text
			return <>{segment.content}</>
		}
	}

	return (
		<p
			class={`text-${currentTextSize()} leading-relaxed font-jp ${local.class || ''}`}
			{...others}
		>
			<For each={segments()}>
				{(segment) => renderSegment(segment)}
			</For>
		</p>
	)
}