import { Code, Copy } from 'lucide-react'
import { useMemo } from 'react'
import type { SyntaxHighlighterProps } from 'react-syntax-highlighter'
import SyntaxHighlighter from 'react-syntax-highlighter'
import { ToolEvent } from './ToolEventRenderer'
import { ToolSummaryCard } from './ToolSummaryCard'

const SyntaxHighlighterComp = SyntaxHighlighter as unknown as React.ComponentType<SyntaxHighlighterProps>

interface ReadFileUpdateProps {
	event: ToolEvent
	accumulatedArgs: any
	copyToClipboard: (content: string) => void
}

interface ReadFileResultProps {
	event: ToolEvent
	copyToClipboard: (content: string) => void
	copiedCode: string | null
}

interface ReadFileExecuteProps {
	event: ToolEvent
}

export const ReadFileExecute = ({ event }: ReadFileExecuteProps) => {
	return (
		<div className="flex items-center gap-2 text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="animate-spin h-4 w-4 border-2 border-blue-500 border-t-transparent rounded-full" />
			<span className="font-medium">Executing file read...</span>
		</div>
	)
}

export const ReadFileUpdate = ({ event, accumulatedArgs, copyToClipboard }: ReadFileUpdateProps) => {
	const path = useMemo(() => accumulatedArgs.target_file || '', [accumulatedArgs.target_file])
	const explanation = useMemo(() => accumulatedArgs.explanation || '', [accumulatedArgs.explanation])
	const shouldReadEntireFile = useMemo(() => accumulatedArgs.should_read_entire_file ?? true, [accumulatedArgs.should_read_entire_file])
	const startLine = useMemo(() => accumulatedArgs.start_line_one_indexed, [accumulatedArgs.start_line_one_indexed])
	const endLine = useMemo(() => accumulatedArgs.end_line_one_indexed, [accumulatedArgs.end_line_one_indexed])

	const handleCopyDelta = async () => {
		await copyToClipboard(JSON.stringify(accumulatedArgs, null, 2))
	}

	const getReadingDescription = () => {
		if (shouldReadEntireFile) {
			return 'Reading entire file'
		}
		if (startLine && endLine) {
			return `Reading lines ${startLine}-${endLine}`
		}
		if (startLine) {
			return `Reading from line ${startLine}`
		}
		return 'Reading file'
	}

	return (
		<div className="text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="flex items-center gap-2 mb-3">
				<svg className="mr-3 -ml-1 size-5 animate-spin text-blue-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
					<circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
					<path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
				<span className="font-medium">
					{getReadingDescription()} {path}
				</span>
			</div>
			<div className="mt-2 text-xs bg-background/50 p-2 rounded border border-border relative">
				<button onClick={handleCopyDelta} className="absolute top-2 right-2 p-1.5 rounded bg-muted hover:bg-muted/80 transition-colors" title="Copy arguments">
					<Copy className="w-3 h-3" />
				</button>
				<div className="font-semibold text-blue-600 flex items-center gap-2">
					<Code className="w-3 h-3" />
					{getReadingDescription()}: {path || 'Unknown file'}
				</div>
				{explanation && (
					<div className="mt-2 text-xs text-muted-foreground">
						<strong>Reason:</strong> {explanation}
					</div>
				)}
			</div>
		</div>
	)
}

export const ReadFileResult = ({ event, copyToClipboard, copiedCode }: ReadFileResultProps) => {
	const resultArgs = useMemo(() => {
		const args = event.arguments && typeof event.arguments === 'string' ? JSON.parse(event.arguments) : {}
		return args
	}, [event.arguments])
	const path = useMemo(() => resultArgs.target_file || '', [resultArgs.target_file])
	const explanation = useMemo(() => resultArgs.explanation || '', [resultArgs.explanation])
	const shouldReadEntireFile = useMemo(() => resultArgs.should_read_entire_file ?? true, [resultArgs.should_read_entire_file])
	const startLine = useMemo(() => resultArgs.start_line_one_indexed, [resultArgs.start_line_one_indexed])
	const endLine = useMemo(() => resultArgs.end_line_one_indexed, [resultArgs.end_line_one_indexed])

	const getReadingDescription = () => {
		if (shouldReadEntireFile) {
			return 'Entire file'
		}
		if (startLine && endLine) {
			return `Lines ${startLine}-${endLine}`
		}
		if (startLine) {
			return `From line ${startLine}`
		}
		return 'File content'
	}

	const summary = useMemo(() => `The file "${path}" has been read successfully.`, [path])

	return (
		<ToolSummaryCard event={event}>
			<div className="space-y-3 ">
				{/* Summary Section */}
				<div className="text-sm text-muted-foreground">{summary}</div>

				{/* Explanation */}
				{explanation && (
					<div className="p-3 m-2 bg-blue-50 dark:bg-blue-950/20 rounded-lg border border-blue-200 dark:border-blue-800">
						<div className="font-semibold text-sm mb-2 text-blue-700 dark:text-blue-300">Explanation:</div>
						<div className="text-sm text-blue-600 dark:text-blue-200">{explanation}</div>
					</div>
				)}

				{/* File Details */}
				<div className="bg-background/50 p-3 m-2 rounded border border-border">
					<div className="font-semibold text-sm mb-2">File:</div>
					<div className="text-sm">{path}</div>
					<div className="font-semibold text-sm mb-2 mt-3">Content Read:</div>
					<div className="text-sm text-muted-foreground">{getReadingDescription()}</div>
				</div>
			</div>
		</ToolSummaryCard>
	)
}
