import React, { useCallback, useMemo, useState, useRef, useEffect } from 'react'
import { Check, Copy, Edit } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import type { SyntaxHighlighterProps } from 'react-syntax-highlighter'
import SyntaxHighlighter from 'react-syntax-highlighter'
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism'
import { ToolEvent } from './ToolEventRenderer'
import { ToolSummaryCard } from './ToolSummaryCard'
import { getLanguage } from '@/lib/language'

const SyntaxHighlighterComp = SyntaxHighlighter as unknown as React.ComponentType<SyntaxHighlighterProps>

interface EditFileUpdateProps {
	event: ToolEvent
	accumulatedArgs: any
	copyToClipboard: (content: string) => void
}

interface EditFileResultProps {
	event: ToolEvent
	copyToClipboard: (content: string) => void
	copiedCode: string | null
}

interface EditFileExecuteProps {
	event: ToolEvent
}

export const EditFileExecute = ({ event }: EditFileExecuteProps) => {
	const args = useMemo(() => (event.arguments ? JSON.parse(event.arguments) : {}), [event.arguments])
	return (
		<div className="flex items-center gap-2 text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="animate-spin h-4 w-4 border-2 border-yellow-500 border-t-transparent rounded-full" />
			<span className="font-medium">Applying changes...</span>
			{event.arguments && (
				<div className="mt-2 text-xs bg-background/50 p-2 rounded">
					<div className="font-semibold mb-1">Final arguments:</div>
					<SyntaxHighlighterComp
						language="json"
						style={vscDarkPlus}
						PreTag="div"
						className="rounded text-xs"
						showLineNumbers={false}
						wrapLines={false}
						wrapLongLines={false}
						customStyle={{
							margin: 0,
							padding: '0.5rem',
							background: 'rgb(20, 20, 20)',
							fontSize: '10px',
						}}
					>
						{args.code_edit}
					</SyntaxHighlighterComp>
				</div>
			)}
		</div>
	)
}

export const EditFileUpdate = ({ event, accumulatedArgs, copyToClipboard }: EditFileUpdateProps) => {
	const targetFile = useMemo(() => accumulatedArgs.target_file || '', [accumulatedArgs.target_file])
	const instructions = useMemo(() => accumulatedArgs.instructions || '', [accumulatedArgs.instructions])
	const codeEdit = useMemo(() => accumulatedArgs.search_replace_blocks || '', [accumulatedArgs.search_replace_blocks])
	const language = getLanguage(targetFile)
	const scrollContainerRef = useRef<HTMLDivElement>(null)

	// Auto-scroll to bottom when new content is received
	useEffect(() => {
		if (scrollContainerRef.current && codeEdit) {
			scrollContainerRef.current.scrollTop = scrollContainerRef.current.scrollHeight
		}
	}, [codeEdit])

	const handleCopyDelta = async () => {
		await copyToClipboard(JSON.stringify(accumulatedArgs, null, 2))
	}

	return (
		<div className="text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border w-full">
			<div className="flex items-center gap-2 mb-3">
				<svg className="mr-3 -ml-1 size-5 animate-spin text-blue-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
					<circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
					<path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
				<span className="font-medium">Updating {event.name.replace(/_/g, ' ')}...</span>
			</div>
			<div className="mt-2 text-xs bg-background/50 rounded border border-border w-full">
				<div className="border-b border-border p-3">
					<div className="flex justify-between items-center">
						<div className="font-semibold text-yellow-600 flex items-center gap-2">
							<Edit className="w-3 h-3" />
							<span className="truncate">Editing file: {targetFile}</span>
						</div>
						<button onClick={handleCopyDelta} className="p-1.5 rounded bg-muted hover:bg-muted/80 transition-colors flex-shrink-0" title="Copy content">
							<Copy className="w-3 h-3" />
						</button>
					</div>
					{instructions && (
						<div className="text-xs text-muted-foreground mt-2">
							<div className="font-medium text-foreground mb-1">Instructions:</div>
							<div className="italic line-clamp-2">{instructions}</div>
						</div>
					)}
				</div>
				<div className="max-h-[300px] overflow-auto w-full" ref={scrollContainerRef}>
					{codeEdit ? (
						<div className="overflow-auto w-full">
							<SyntaxHighlighterComp
								language={language}
								style={vscDarkPlus}
								PreTag="div"
								className="rounded text-xs w-full"
								showLineNumbers={true}
								wrapLines={false}
								wrapLongLines={false}
								customStyle={{
									margin: 0,
									padding: '0.5rem',
									background: 'rgb(20, 20, 20)',
									fontSize: '11px',
									width: '100%',
									minWidth: '100%',
								}}
							>
								{codeEdit}
							</SyntaxHighlighterComp>
						</div>
					) : (
						<div className="p-3 text-muted-foreground italic flex items-center justify-center h-[200px] w-full">Waiting for content...</div>
					)}
				</div>
			</div>
		</div>
	)
}

export const EditFileResult = ({ event }: EditFileResultProps) => {
	const resultArgs = useMemo(() => {
		const args = event.arguments && typeof event.arguments === 'string' ? JSON.parse(event.arguments) : {}
		return args
	}, [event.arguments])
	console.log('resultArgs', resultArgs, event)
	const [copiedCode, setCopiedCode] = useState<string | null>(null)
	const scrollContainerRef = useRef<HTMLDivElement>(null)

	// Auto-scroll to bottom when new content is received
	useEffect(() => {
		if (scrollContainerRef.current && resultArgs.code_edit) {
			scrollContainerRef.current.scrollTop = scrollContainerRef.current.scrollHeight
		}
	}, [resultArgs.code_edit])

	const filePath = useMemo(() => resultArgs.target_file || '', [resultArgs.target_file])
	const instructions = useMemo(() => resultArgs.instructions || '', [resultArgs.instructions])
	const summary = useMemo(() => `File "${filePath}" edited successfully.`, [filePath])
	const copyToClipboard = useCallback((content: string) => {
		navigator.clipboard.writeText(content)
		setCopiedCode(content)
		setTimeout(() => setCopiedCode(null), 2000)
	}, [])
	const details = (
		<Dialog>
			<DialogTrigger>
				<>
					{instructions && (
						<div className="p-3 bg-blue-50 dark:bg-blue-950/20 rounded-lg border border-blue-200 dark:border-blue-800 mb-3">
							<div className="font-semibold text-sm mb-2 text-blue-700 dark:text-blue-300">Explanation:</div>
							<div className="text-sm text-blue-600 dark:text-blue-200">{instructions}</div>
						</div>
					)}
					<Button variant="ghost" size="sm" className="h-6 text-xs">
						View Details
					</Button>
				</>
			</DialogTrigger>
			<DialogContent className="max-w-4xl h-[90vh] flex flex-col p-0">
				<DialogHeader className="px-6 pt-6 pb-2 border-b">
					<DialogTitle>File Edit Details</DialogTitle>
				</DialogHeader>
				<div className="flex-1 overflow-auto px-6 py-4">
					<div className="space-y-4">
						<div className="p-3 bg-muted rounded-lg">
							<div className="font-semibold text-sm mb-2">File:</div>
							<div className="text-sm break-all">{filePath}</div>
						</div>
						<div className="p-3 bg-muted rounded-lg">
							<div className="font-semibold text-sm mb-2">Modified Content:</div>
							<div className="relative">
								<button
									onClick={() => copyToClipboard(resultArgs.code_edit)}
									className="absolute top-2 right-2 p-1.5 rounded bg-background hover:bg-background/80 transition-colors z-10"
									title="Copy code"
								>
									{copiedCode === resultArgs.code_edit ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
								</button>
								<div className="overflow-auto" ref={scrollContainerRef}>
									<SyntaxHighlighterComp
										language={getLanguage(filePath)}
										style={vscDarkPlus}
										wrapLines={false}
										wrapLongLines={false}
										showLineNumbers={true}
										customStyle={{
											margin: 0,
											padding: '0.5rem',
											background: 'rgb(20, 20, 20)',
											fontSize: '11px',
											minWidth: '100%',
										}}
									>
										{resultArgs.code_edit}
									</SyntaxHighlighterComp>
								</div>
							</div>
						</div>
						{instructions && (
							<div className="p-3 bg-muted rounded-lg">
								<div className="font-semibold text-sm mb-2">Instructions:</div>
								<div className="text-sm whitespace-pre-wrap">{instructions}</div>
							</div>
						)}
						<div className="p-3 bg-muted rounded-lg">
							<div className="font-semibold text-sm mb-2">Status:</div>
							<div className="text-sm">File modified</div>
						</div>
					</div>
				</div>
			</DialogContent>
		</Dialog>
	)

	return (
		<ToolSummaryCard event={event} summary={summary}>
			{details}
		</ToolSummaryCard>
	)
}
