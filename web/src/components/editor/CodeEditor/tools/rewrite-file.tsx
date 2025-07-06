import { getLanguage } from '@/lib/language'
import { Check, Copy, RefreshCw } from 'lucide-react'
import React, { useEffect, useMemo, useRef } from 'react'
import type { SyntaxHighlighterProps } from 'react-syntax-highlighter'
import SyntaxHighlighter from 'react-syntax-highlighter'
import { ToolEvent } from './ToolEventRenderer'
import { ToolSummaryCard } from './ToolSummaryCard'
import { LazyCodeBlock } from './lazy-code-block'

interface RewriteFileUpdateProps {
	event: ToolEvent
	accumulatedArgs: any
	copyToClipboard: (content: string) => void
}

interface RewriteFileResultProps {
	event: ToolEvent
	copyToClipboard: (content: string) => void
	copiedCode: string | null
}

interface RewriteFileExecuteProps {
	event: ToolEvent
}

export const RewriteFileExecute = ({ event }: RewriteFileExecuteProps) => {
	const args = useMemo(() => (event.arguments ? JSON.parse(event.arguments) : {}), [event.arguments])
	return (
		<div className="flex items-center gap-2 text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="animate-spin h-4 w-4 border-2 border-purple-500 border-t-transparent rounded-full" />
			<span className="font-medium">Rewriting file...</span>
			{args.file && (
				<span className="text-xs bg-background/50 px-2 py-1 rounded">
					{args.file}
				</span>
			)}
		</div>
	)
}

export const RewriteFileUpdate = ({ event, accumulatedArgs, copyToClipboard }: RewriteFileUpdateProps) => {
	const targetFile = accumulatedArgs.file || ''
	const newContent = accumulatedArgs.new_content || ''
	const explanation = accumulatedArgs.explanation || ''
	const language = getLanguage(targetFile)
	const scrollContainerRef = useRef<HTMLDivElement>(null)

	// Auto-scroll to bottom whenever content updates
	useEffect(() => {
		if (scrollContainerRef.current && newContent) {
			scrollContainerRef.current.scrollTop = scrollContainerRef.current.scrollHeight
		}
	}, [newContent])

	const handleCopyDelta = async () => {
		await copyToClipboard(JSON.stringify(accumulatedArgs, null, 2))
	}

	return (
		<div className="text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="flex items-center gap-2 mb-3">
				<svg className="mr-3 -ml-1 size-5 animate-spin text-purple-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
					<circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
					<path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
				<span className="font-medium">Rewriting {event.name.replace(/_/g, ' ')}...</span>
			</div>
			<div className="mt-2 text-xs bg-background/50 rounded border border-border">
				<div className="border-b border-border p-3">
					<div className="flex justify-between items-center">
						<div className="font-semibold text-purple-600 flex items-center gap-2">
							<RefreshCw className="w-3 h-3" />
							<span className="truncate">Rewriting file: {targetFile}</span>
						</div>
						<button onClick={handleCopyDelta} className="p-1.5 rounded bg-muted hover:bg-muted/80 transition-colors flex-shrink-0" title="Copy content">
							<Copy className="w-3 h-3" />
						</button>
					</div>
					{explanation && (
						<div className="text-xs text-muted-foreground mt-2">
							<div className="font-medium text-foreground mb-1">Explanation:</div>
							<div className="italic line-clamp-2">{explanation}</div>
						</div>
					)}
				</div>
				<div ref={scrollContainerRef} className="max-h-[500px] overflow-auto">
					{newContent ? (
						<div className="overflow-auto">
							<LazyCodeBlock
								code={newContent}
								language={language}
							/>
						</div>
					) : (
						<div className="p-3 text-muted-foreground italic flex items-center justify-center h-[200px]">Waiting for content...</div>
					)}
				</div>
			</div>
		</div>
	)
}

export const RewriteFileResult = ({ event, copyToClipboard, copiedCode }: RewriteFileResultProps) => {
	const resultArgs = useMemo(() => {
		const args = event.arguments && typeof event.arguments === 'string' ? JSON.parse(event.arguments) : {}
		return args
	}, [event.arguments])

	const filePath = resultArgs.file || ''
	const newContent = resultArgs.new_content || ''
	const explanation = resultArgs.explanation || ''
	const result = event.result && typeof event.result === 'object' ? (event.result as any) : {}
	const resultMessage = result.result || ''

	const summary = `File "${filePath}" has been completely rewritten.`

	return (
		<ToolSummaryCard event={event}>
			<div className="space-y-3">
				{/* Summary Section */}
				<div className="text-sm text-muted-foreground mb-3">{summary}</div>

				{/* Explanation */}
				{explanation && (
					<div className="p-3 bg-purple-50 dark:bg-purple-950/20 rounded-lg border border-purple-200 dark:border-purple-800">
						<div className="font-semibold text-sm mb-2 text-purple-700 dark:text-purple-300">Explanation:</div>
						<div className="text-sm text-purple-600 dark:text-purple-200">{explanation}</div>
					</div>
				)}

				{/* File Info */}
				<div className="bg-background/50 p-3 rounded border border-border">
					<div className="font-semibold text-sm mb-2">File Info:</div>
					<div className="text-sm space-y-1">
						<div>Path: {filePath}</div>
						<div>Status: File completely rewritten</div>
						{resultMessage && <div>Result: {resultMessage}</div>}
					</div>
				</div>

				{/* New File Content */}
				{newContent && (
					<div className="bg-background/50 p-3 rounded border border-border">
						<div className="font-semibold text-sm mb-2 flex items-center justify-between">
							<span>New File Content:</span>
							<button onClick={() => copyToClipboard(newContent)} className="p-1.5 rounded bg-muted hover:bg-muted/80 transition-colors" title="Copy code">
								{copiedCode === newContent ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
							</button>
						</div>
						<div className="max-h-[400px] overflow-auto">
							<LazyCodeBlock
								code={newContent}
								language={getLanguage(filePath)}
							/>
						</div>
					</div>
				)}
			</div>
		</ToolSummaryCard>
	)
} 