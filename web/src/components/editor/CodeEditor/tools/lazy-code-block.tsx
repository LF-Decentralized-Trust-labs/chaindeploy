import { useState, useMemo } from 'react'
import type { SyntaxHighlighterProps } from 'react-syntax-highlighter'
import SyntaxHighlighter from 'react-syntax-highlighter'
const SyntaxHighlighterComp = SyntaxHighlighter as unknown as React.ComponentType<SyntaxHighlighterProps>

// Lazy code block: only renders SyntaxHighlighter when expanded
export function LazyCodeBlock({
	code,
	language,
	previewLines = 10,
	className = '',
	style = {},
}: {
	code: string
	language?: string
	previewLines?: number
	className?: string
	style?: React.CSSProperties
}) {
	const [expanded, setExpanded] = useState(false)

	const { lines, preview, isLong } = useMemo(() => {
		const lines = code.split('\n')
		const preview = lines.slice(0, previewLines).join('\n')
		const isLong = lines.length > previewLines
		return { lines, preview, isLong }
	}, [code, previewLines])

	if (!expanded && isLong) {
		return (
			<div className={className} style={style}>
				<pre className="text-xs bg-muted p-2 rounded overflow-x-auto max-h-40">
					{preview}\n{isLong ? '... (truncated)' : ''}
				</pre>
				<button className="text-xs underline mt-1" onClick={() => setExpanded(true)}>
					Show full code ({lines.length} lines)
				</button>
			</div>
		)
	}
	return (
		<SyntaxHighlighterComp language={language} PreTag="div" className={className} customStyle={style}>
			{code}
		</SyntaxHighlighterComp>
	)
}
