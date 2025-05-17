import { JSX, splitProps, For, createMemo } from 'solid-js'

export interface TranscriptionTextProps {
	text: string;
	class?: string;
	textSize?: 'xs' | 'sm' | 'base' | 'lg' | 'xl' | '2xl' | '3xl' | '4xl' | '5xl';
	rtClass?: string;
	language?: string;           // ISO 639-1 language code (e.g., "ja", "zh", "en")
	transcriptionType?: string;  // Type of transcription (furigana, pinyin, etc.)
}

// Type for parsed segments
type TranscriptionSegment =
	| { type: 'ruby'; base: string; text: string }
	| { type: 'text'; content: string }
	| { type: 'bold'; content: string; segments?: TranscriptionSegment[] }
	| { type: 'linebreak' };

// Helper function to get the appropriate regex for transcription based on language
const getTranscriptionRegex = (language: string, transcriptionType: string): RegExp => {
	// Default pattern for most languages with [pronunciation] format
	let pattern = /(\S+)\[([^\]]+)]/g

	switch (language) {
		case 'ja':
			// Japanese kanji with furigana
			pattern = /([一-龯]+)\[([^\]]+)]/g
			break
		case 'zh':
			// Chinese characters with pinyin
			pattern = /([一-龯\u3400-\u4DBF\u4E00-\u9FFF\uF900-\uFAFF]+)\[([^\]]+)]/g
			break
		case 'th':
			// Thai script with romanization (any Thai character)
			pattern = /([\u0E00-\u0E7F]+)\[([^\]]+)]/g
			break
		case 'ka':
			// Georgian with transliteration (Mkhedruli script)
			pattern = /([\u10A0-\u10FF]+)\[([^\]]+)]/g
			break
	}

	return pattern
}

// Helper function to parse the text into segments
const parseTextToSegments = (text: string, language = 'ja', transcriptionType = 'furigana'): TranscriptionSegment[] => {
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
	const transcriptionRegex = getTranscriptionRegex(language, transcriptionType)
	let match

	while ((match = transcriptionRegex.exec(processedText)) !== null) {
		// Add text before the current transcription match
		if (match.index > lastIndex) {
			const precedingText = processedText.substring(lastIndex, match.index)
			processPlainTextWithTags(precedingText, segments, htmlTags, language, transcriptionType)
		}

		// Add the transcription segment
		segments.push({ type: 'ruby', base: match[1], text: match[2] })
		lastIndex = transcriptionRegex.lastIndex
	}

	// Add any remaining text after the last transcription match
	if (lastIndex < processedText.length) {
		const remainingText = processedText.substring(lastIndex)
		processPlainTextWithTags(remainingText, segments, htmlTags, language, transcriptionType)
	}

	return segments
}

// Handle plain text that might contain HTML tag placeholders
const processPlainTextWithTags = (
	text: string,
	segments: TranscriptionSegment[],
	htmlTags: Map<string, { type: string, content: string }>,
	language = 'ja',
	transcriptionType = 'furigana',
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
				const transcriptionRegex = getTranscriptionRegex(language, transcriptionType)
				if (tagInfo.content.includes('[') && tagInfo.content.match(transcriptionRegex)) {
					// Parse the bold content separately to handle nested transcription
					const boldSegments = parseTextToSegments(tagInfo.content, language, transcriptionType)
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

// Get the appropriate font class based on language
const getFontClass = (language: string): string => {
	switch (language) {
		case 'ja':
			return 'font-jp' // Japanese font
		case 'zh':
			return 'font-zh' // Chinese font
		case 'th':
			return 'font-th' // Thai font
		case 'ka':
			return 'font-ka' // Georgian font
		default:
			return '' // Default font
	}
}

/**
 * TranscriptionText component for displaying text with reading aids for multiple languages
 * Supports:
 * - Transcription format: text[reading] (e.g., Japanese: "会[あ]う", Chinese: "你[nǐ]好[hǎo]")
 * - HTML tags: Currently supports <b> for bold text and <br/> for line breaks
 * - Can handle nested tags and transcriptions
 * - Supports multiple languages (Japanese, Chinese, Thai, Georgian, etc.)
 */
export default function TranscriptionText(props: TranscriptionTextProps): JSX.Element {
	const [local, others] = splitProps(props, ['text', 'class', 'textSize', 'rtClass', 'language', 'transcriptionType'])

	const currentTextSize = createMemo(() => {
		return local.textSize || 'base'
	})

	const defaultRtClass = createMemo(() => {
		return RT_SIZE_MAP[currentTextSize()] || 'text-xs'
	})

	const finalRtClasses = createMemo(() => {
		return `font-normal ${defaultRtClass()} ${local.rtClass || ''}`
	})

	const language = createMemo(() => {
		return local.language || 'ja'
	})

	const transcriptionType = createMemo(() => {
		return local.transcriptionType || 'furigana'
	})

	const fontClass = createMemo(() => {
		return getFontClass(language())
	})

	// Parse the input text into segments
	const segments = createMemo(() =>
		parseTextToSegments(local.text, language(), transcriptionType()),
	)

	// Recursive renderer for segments (to handle nested structures)
	const renderSegment = (segment: TranscriptionSegment): JSX.Element => {
		if (segment.type === 'ruby') {
			return (
				<ruby class={`font-normal ${language() == 'th' ? 'ruby-under' : ''}`}>
					{segment.base}
					<rt class={finalRtClasses()}>{segment.text}</rt>
				</ruby>
			)
		} else if (segment.type === 'bold') {
			if (segment.segments) {
				// If it has nested segments, render those within the bold context
				return (
					<span class="font-semibold">
						<For each={segment.segments}>
							{(nestedSegment) => renderSegment(nestedSegment)}
						</For>
					</span>
				)
			}
			// Simple bold text
			return <span class="font-medium">{segment.content}</span>
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
			class={`text-${currentTextSize()} leading-relaxed ${fontClass()} ${local.class || ''}`}
			{...others}
		>
			<For each={segments()}>
				{(segment) => renderSegment(segment)}
			</For>
		</p>
	)
}
