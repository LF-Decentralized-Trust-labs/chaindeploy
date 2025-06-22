import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Check, Code, Copy, FileText } from 'lucide-react'
import React, { useEffect, useRef, useMemo } from 'react'
import type { SyntaxHighlighterProps } from 'react-syntax-highlighter'
import SyntaxHighlighter from 'react-syntax-highlighter'
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism'
import { ToolEvent } from './ToolEventRenderer'
import { ToolSummaryCard } from './ToolSummaryCard'
import { getLanguage } from '@/lib/language'

const SyntaxHighlighterComp = SyntaxHighlighter as unknown as React.ComponentType<SyntaxHighlighterProps>

interface WriteFileUpdateProps {
	event: ToolEvent
	accumulatedArgs: any
	copyToClipboard: (content: string) => void
}

interface WriteFileResultProps {
	event: ToolEvent
	copyToClipboard: (content: string) => void
	copiedCode: string | null
}

interface WriteFileExecuteProps {
	event: ToolEvent
}

export const WriteFileExecute = ({ event }: WriteFileExecuteProps) => {
	return (
		<div className="flex items-center gap-2 text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="animate-spin h-4 w-4 border-2 border-green-500 border-t-transparent rounded-full" />
			<span className="font-medium">Executing file write...</span>
		</div>
	)
}

export const WriteFileUpdate = ({ event, accumulatedArgs, copyToClipboard }: WriteFileUpdateProps) => {
	const targetFile = accumulatedArgs.target_file || ''
	const content = accumulatedArgs.content || ''
	const language = getLanguage(targetFile)
	const scrollContainerRef = useRef<HTMLDivElement>(null)

	// Auto-scroll to bottom whenever content updates
	useEffect(() => {
		if (scrollContainerRef.current && content) {
			scrollContainerRef.current.scrollTop = scrollContainerRef.current.scrollHeight
		}
	}, [content])

	const handleCopyDelta = async () => {
		await copyToClipboard(JSON.stringify(accumulatedArgs, null, 2))
	}

	return (
		<div className="text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="flex items-center gap-2 mb-3">
				<svg className="mr-3 -ml-1 size-5 animate-spin text-blue-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
					<circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
					<path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
				<span className="font-medium">Updating {event.name.replace(/_/g, ' ')}...</span>
			</div>
			<div className="mt-2 text-xs bg-background/50 rounded border border-border">
				<div className="border-b border-border p-3">
					<div className="flex justify-between items-center">
						<div className="font-semibold text-green-600 flex items-center gap-2">
							<FileText className="w-3 h-3" />
							<span className="truncate">Writing file: {targetFile}</span>
						</div>
						<button onClick={handleCopyDelta} className="p-1.5 rounded bg-muted hover:bg-muted/80 transition-colors flex-shrink-0" title="Copy content">
							<Copy className="w-3 h-3" />
						</button>
					</div>
				</div>
				<div ref={scrollContainerRef} className="max-h-[500px] overflow-auto">
					{content ? (
						<div className="overflow-auto">
							<SyntaxHighlighterComp
								language={language}
								style={vscDarkPlus}
								PreTag="div"
								className="rounded text-xs"
								showLineNumbers={true}
								wrapLines={false}
								wrapLongLines={false}
								customStyle={{
									margin: 0,
									padding: '0.5rem',
									background: 'rgb(20, 20, 20)',
									fontSize: '11px',
									minWidth: '100%',
								}}
							>
								{content}
							</SyntaxHighlighterComp>
						</div>
					) : (
						<div className="p-3 text-muted-foreground italic flex items-center justify-center h-[200px]">Waiting for content...</div>
					)}
				</div>
			</div>
		</div>
	)
}

export const WriteFileResult = ({ event, copyToClipboard, copiedCode }: WriteFileResultProps) => {
	const resultArgs = useMemo(() => {
		const args = event.arguments && typeof event.arguments === 'string' ? JSON.parse(event.arguments) : {}
		return args
	}, [event.arguments])

	const path = resultArgs.path || ''
	const content = resultArgs.content || ''
	const result = event.result && typeof event.result === 'object' ? (event.result as any) : {}
	const resultMessage = result.result || ''
	const created = result.created || false

	const summary = created ? `New file "${path}" has been created.` : `The file "${path}" has been updated.`

	const details = (
		<Dialog>
			<DialogTrigger asChild>
				<Button variant="ghost" size="sm" className="h-6 text-xs">
					View Contents
				</Button>
			</DialogTrigger>
			<DialogContent className="max-w-2xl">
				<DialogHeader>
					<DialogTitle>File: {path}</DialogTitle>
				</DialogHeader>
				<ScrollArea className="max-h-[60vh]">
					<div className="space-y-4">
						<div className="p-3 bg-muted rounded-lg">
							<div className="font-semibold text-sm mb-2">File Info:</div>
							<div className="text-sm space-y-1">
								<div>Path: {path}</div>
								<div>Status: {created ? 'Created new file' : 'Updated existing file'}</div>
								{resultMessage && <div>Result: {resultMessage}</div>}
							</div>
						</div>
						<div className="relative group">
							<div className="absolute right-2 top-2 opacity-0 group-hover:opacity-100 transition-opacity">
								<button onClick={() => copyToClipboard(content)} className="p-1.5 rounded bg-muted hover:bg-muted/80 transition-colors" title="Copy code">
									{copiedCode === content ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
								</button>
							</div>
							<SyntaxHighlighterComp
								language={getLanguage(path)}
								style={vscDarkPlus}
								PreTag="div"
								className="rounded-lg"
								showLineNumbers={true}
								wrapLines={true}
								wrapLongLines={true}
								customStyle={{ margin: 0, padding: '1rem' }}
							>
								{content}
							</SyntaxHighlighterComp>
						</div>
					</div>
				</ScrollArea>
			</DialogContent>
		</Dialog>
	)

	return (
		<ToolSummaryCard event={event} summary={summary}>
			{details}
		</ToolSummaryCard>
	)
}
