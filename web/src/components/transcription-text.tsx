import { JSX, splitProps, For, createMemo } from 'solid-js'
import { cn } from '~/lib/utils'

export interface TranscriptionTextProps {
	text: string;
	class?: string;
	textSize?: 'xs' | 'sm' | 'base' | 'lg' | 'xl' | '2xl' | '3xl' | '4xl' | '5xl';
	rtClass?: string;
	language?: string;           // ISO 639-1 language code (e.g., "jp", "zh", "en")
}

// Type for parsed segments
type TranscriptionSegment =
	| { type: 'ruby'; base: string; text: string }
	| { type: 'text'; content: string }
	| { type: 'bold'; content: string; segments?: TranscriptionSegment[] }
	| { type: 'linebreak' };

// Helper function to get the appropriate regex for transcription based on language
const getTranscriptionRegex = (language: string): RegExp => {
	// Default pattern for most languages with [pronunciation] format
	let pattern = /(\S+)\[([^\]]+)]/g

	switch (language) {
		case 'jp':
			// japanese kanji with furigana
			pattern = /([一-龯]+)\[([^\]]+)]/g
			break
		case 'th':
			// Thai script with romanization (any Thai character)
			pattern = /([\u0E00-\u0E7F]+)\[([^\]]+)]/g
			break
	}

	return pattern
}

// Helper function to parse the text into segments
const parseTextToSegments = (text: string, language = 'jp'): TranscriptionSegment[] => {
	const segments: TranscriptionSegment[] = []
	if (!text) {
		return segments
	}

	// Step 1: Handle HTML tags like <b> and <br/> separately to prevent interference with transcription parsing
	const htmlTags = new Map<string, { type: string, content: string }>()
	let tagCounter = 0

	// Replace <br/> tags with placeholders
	let processedText = text.replace(/<br\s*\/?>|<br\s*>/g, () => {
		const placeholder = `__HTML_TAG_${tagCounter}__`
		htmlTags.set(placeholder, { type: 'linebreak', content: '' })
		tagCounter++
		return placeholder
	})

	// Replace bold tags with placeholders
	processedText = processedText.replace(/<b>(.+?)<\/b>/g, (_, content) => {
		const placeholder = `__HTML_TAG_${tagCounter}__`
		htmlTags.set(placeholder, { type: 'bold', content })
		tagCounter++
		return placeholder
	})

	// Step 2: Process transcription notation in the text
	let lastIndex = 0
	// Get the appropriate regex for this language and transcription type
	const transcriptionRegex = getTranscriptionRegex(language)
	let match

	while ((match = transcriptionRegex.exec(processedText)) !== null) {
		// Add text before the current transcription match
		if (match.index > lastIndex) {
			const precedingText = processedText.substring(lastIndex, match.index)
			processPlainTextWithTags(precedingText, segments, htmlTags, language)
		}

		// Add the transcription segment
		segments.push({ type: 'ruby', base: match[1], text: match[2] })
		lastIndex = transcriptionRegex.lastIndex
	}

	// Add any remaining text after the last transcription match
	if (lastIndex < processedText.length) {
		const remainingText = processedText.substring(lastIndex)
		processPlainTextWithTags(remainingText, segments, htmlTags, language)
	}

	return segments
}

// Handle plain text that might contain HTML tag placeholders
const processPlainTextWithTags = (
	text: string,
	segments: TranscriptionSegment[],
	htmlTags: Map<string, { type: string, content: string }>,
	language = 'jp',
) => {
	const tagRegex = /__HTML_TAG_(\d+)__/g
	let lastIndex = 0
	let match

	while ((match = tagRegex.exec(text)) !== null) {
		// Add plain text before the tag placeholder
		if (match.index > lastIndex) {
			segments.push({
				type: 'text',
				content: text.substring(lastIndex, match.index),
			})
		}

		// Process the tag
		const placeholder = match[0]
		const tagInfo = htmlTags.get(placeholder)

		if (tagInfo) {
			if (tagInfo.type === 'linebreak') {
				// Add a linebreak segment
				segments.push({ type: 'linebreak' })
			} else if (tagInfo.type === 'bold') {
				// For bold tags, check if there's transcription inside
				const transcriptionRegex = getTranscriptionRegex(language)
				if (tagInfo.content.includes('[') && tagInfo.content.match(transcriptionRegex)) {
					// Parse the bold content separately to handle nested transcription
					const boldSegments = parseTextToSegments(tagInfo.content, language)
					segments.push({
						type: 'bold',
						content: '', // Empty as we'll use the segments instead
						segments: boldSegments,
					})
				} else {
					// Regular bold text without transcription
					segments.push({
						type: 'bold',
						content: tagInfo.content,
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
			content: text.substring(lastIndex),
		})
	}
}

// Get the appropriate font class based on language
const getFontClass = (language: string): string => {
	switch (language) {
		case 'jp':
			return 'font-jp' // japanese font
		case 'zh':
			return 'font-zh' // Chinese font
		case 'th':
			return 'font-th' // Thai font
		case 'ge':
			return 'font-ge' // Georgian font
		default:
			return '' // Default font
	}
}

/**
 * TranscriptionText component for displaying text with reading aids for multiple languages
 * Supports:
 * - Transcription format: text[reading] (e.g., jppanese: "会[あ]う", Chinese: "你[nǐ]好[hǎo]")
 * - HTML tags: Currently supports <b> for bold text and <br/> for line breaks
 * - Can handle nested tags and transcriptions
 * - Supports multiple languages (japanese, Chinese, Thai, Georgian, etc.)
 */
export default function TranscriptionText(props: TranscriptionTextProps): JSX.Element {
	const [local, others] = splitProps(props, ['text', 'class', 'rtClass', 'language'])

	const language = createMemo(() => {
		return local.language || 'jp'
	})


	const fontClass = createMemo(() => {
		return getFontClass(language())
	})

	// Parse the input text into segments
	const segments = createMemo(() =>
		parseTextToSegments(local.text, language()),
	)

	// Recursive renderer for segments (to handle nested structures)
	const renderSegment = (segment: TranscriptionSegment): JSX.Element => {
		if (segment.type === 'ruby') {
			return (
				<ruby class={`${language() == 'th' ? 'ruby-under' : ''}`}>
					{segment.base}
					<rt class={local.rtClass}>{segment.text}</rt>
				</ruby>
			)
			// } else if (segment.type === 'bold') {
			// 	if (segment.segments) {
			// 		// If it has nested segments, render those within the bold context
			// 		return (
			// 			<span class="font-semibold">
			// 				<For each={segment.segments}>
			// 					{(nestedSegment) => renderSegment(nestedSegment)}
			// 				</For>
			// 			</span>
			// 		)
			// 	}
			// 	// Simple bold text
			//	return <span class="font-medium">{segment.content}</span>
		} else if (segment.type === 'linebreak') {
			// Render a line break
			return <br />
		} else {
			// Plain text
			return <>{segment.content}</>
		}
	}

	// Default rendering for other languages
	return (
		<p
			class={cn('leading-relaxed', fontClass(), local.class)}
			{...others}
		>
			<For each={segments()}>
				{(segment) => renderSegment(segment)}
			</For>
		</p>
	)
}
