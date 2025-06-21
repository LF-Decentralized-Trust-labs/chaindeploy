import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { jsonrepair } from 'jsonrepair'
import { Copy } from 'lucide-react'
import React, { useCallback, useEffect, useMemo, useState, useRef } from 'react'
import { toast } from 'sonner'
import { ToolExecuteComponent } from './ToolExecuteComponent'
import { ToolStartComponent } from './ToolStartComponent'
import { ToolSummaryCard } from './ToolSummaryCard'

// Import all tool components
import { CodebaseSearchResult, CodebaseSearchUpdate, CodebaseSearchExecute } from './codebase-search'
import { DeleteFileResult, DeleteFileUpdate, DeleteFileExecute } from './delete-file'
import { EditFileResult, EditFileUpdate, EditFileExecute } from './edit-file'
import { FileSearchResult, FileSearchUpdate, FileSearchExecute } from './file-search'
import { GrepSearchResult, GrepSearchUpdate, GrepSearchExecute } from './grep-search'
import { ListDirResult, ListDirUpdate, ListDirExecute } from './list-dir'
import { ReadFileResult, ReadFileUpdate, ReadFileExecute } from './read-file'
import { ReadFileEnhancedResult, ReadFileEnhancedUpdate, ReadFileEnhancedExecute } from './read-file-enhanced'
import { ReapplyResult, ReapplyUpdate, ReapplyExecute } from './reapply'
import { RunTerminalCmdResult, RunTerminalCmdUpdate, RunTerminalCmdExecute } from './run-terminal-cmd'
import { SearchReplaceResult, SearchReplaceUpdate, SearchReplaceExecute } from './search-replace'
import { WriteFileResult, WriteFileUpdate, WriteFileExecute } from './write-file'

export interface ToolEvent {
	type: 'start' | 'update' | 'execute' | 'result'
	toolCallID: string
	name: string
	arguments?: string
	args?: Record<string, unknown>
	result?: unknown
	error?: string
}

interface ToolEventProps {
	event: ToolEvent
}

// Tool component mapping
const toolComponents = {
	read_file: { update: ReadFileUpdate, result: ReadFileResult, execute: ReadFileExecute },
	write_file: { update: WriteFileUpdate, result: WriteFileResult, execute: WriteFileExecute },
	codebase_search: { update: CodebaseSearchUpdate, result: CodebaseSearchResult, execute: CodebaseSearchExecute },
	read_file_enhanced: { update: ReadFileEnhancedUpdate, result: ReadFileEnhancedResult, execute: ReadFileEnhancedExecute },
	run_terminal_cmd: { update: RunTerminalCmdUpdate, result: RunTerminalCmdResult, execute: RunTerminalCmdExecute },
	list_dir: { update: ListDirUpdate, result: ListDirResult, execute: ListDirExecute },
	grep_search: { update: GrepSearchUpdate, result: GrepSearchResult, execute: GrepSearchExecute },
	edit_file: { update: EditFileUpdate, result: EditFileResult, execute: EditFileExecute },
	search_replace: { update: SearchReplaceUpdate, result: SearchReplaceResult, execute: SearchReplaceExecute },
	file_search: { update: FileSearchUpdate, result: FileSearchResult, execute: FileSearchExecute },
	delete_file: { update: DeleteFileUpdate, result: DeleteFileResult, execute: DeleteFileExecute },
	reapply: { update: ReapplyUpdate, result: ReapplyResult, execute: ReapplyExecute },
}

export const ToolEventRenderer = React.memo(({ event }: ToolEventProps) => {
	const [copiedCode, setCopiedCode] = useState<string | null>(null)
	const previousDeltaRef = useRef<any>(null)
	// Reset previous delta when tool event changes (new tool call starts)
	useEffect(() => {
		if (event.type === 'start') {
			previousDeltaRef.current = null
		}
	}, [event.toolCallID, event.type])

	// Function to parse delta JSON without updating state
	const parseDelta = useCallback((deltaString: string) => {
		try {
			const parsed = JSON.parse(deltaString)
			return parsed
		} catch (e) {
			try {
				const repaired = jsonrepair(deltaString)
				const parsed = JSON.parse(repaired)
				return parsed
			} catch (e) {
				// Return null to indicate parsing failure
				return null
			}
		}
	}, [])

	// Function to copy content to clipboard
	const copyToClipboard = useCallback(async (content: string) => {
		try {
			await navigator.clipboard.writeText(content)
			setCopiedCode(content)
			setTimeout(() => setCopiedCode(null), 2000)
			toast.success('Copied to clipboard')
		} catch (err) {
			console.error('Failed to copy:', err)
			toast.error('Failed to copy to clipboard')
		}
	}, [])

	const content = useMemo(() => {
		// Handle result events
		if (event.type === 'result') {
			const toolComponent = toolComponents[event.name as keyof typeof toolComponents]
			if (toolComponent?.result) {
				const ResultComponent = toolComponent.result
				return <ResultComponent event={event} copyToClipboard={copyToClipboard} copiedCode={copiedCode} />
			}

			// Fallback for unknown tools
			return <DefaultResultComponent event={event} copyToClipboard={copyToClipboard} copiedCode={copiedCode} />
		}

		// Handle start events
		if (event.type === 'start') {
			return <ToolStartComponent event={event} />
		}

		// Handle update events
		if (event.type === 'update') {
			const toolComponent = toolComponents[event.name as keyof typeof toolComponents]
			if (toolComponent?.update) {
				const UpdateComponent = toolComponent.update
				let accumulatedArgs = previousDeltaRef.current || {}

				if (event.arguments) {
					const parsed = parseDelta(event.arguments)
					if (parsed !== null) {
						// Only update ref and accumulated args if parsing succeeded
						previousDeltaRef.current = parsed
						accumulatedArgs = parsed
					}
					// If parsing failed, keep using previousDeltaRef.current (accumulatedArgs already set above)
				}

				return <UpdateComponent event={event} accumulatedArgs={accumulatedArgs} copyToClipboard={copyToClipboard} />
			}

			// Fallback for unknown tools
			let accumulatedArgs = previousDeltaRef.current || {}
			if (event.arguments) {
				const parsed = parseDelta(event.arguments)
				if (parsed !== null) {
					// Only update ref and accumulated args if parsing succeeded
					previousDeltaRef.current = parsed
					accumulatedArgs = parsed
				}
				// If parsing failed, keep using previousDeltaRef.current (accumulatedArgs already set above)
			}
			return <DefaultUpdateComponent event={event} accumulatedArgs={accumulatedArgs} copyToClipboard={copyToClipboard} />
		}

		// Handle execute events
		if (event.type === 'execute') {
			const toolComponent = toolComponents[event.name as keyof typeof toolComponents]
			if (toolComponent?.execute) {
				const ExecuteComponent = toolComponent.execute
				return <ExecuteComponent event={event} />
			}

			// Fallback for unknown tools
			return <ToolExecuteComponent event={event} />
		}

		return null
	}, [event, parseDelta, copyToClipboard, copiedCode])

	return content
})

// Default components for unknown tools
const DefaultResultComponent = ({ event, copyToClipboard, copiedCode }: { event: ToolEvent; copyToClipboard: (content: string) => void; copiedCode: string | null }) => {
	const summary = `${event.name.replace(/_/g, ' ')} completed successfully.`

	return (
		<ToolSummaryCard event={event} summary={summary}>
			{event.result && (
				<Dialog>
					<DialogTrigger asChild>
						<Button variant="ghost" size="sm" className="h-6 text-xs">
							View Details
						</Button>
					</DialogTrigger>
					<DialogContent className="max-w-2xl">
						<DialogHeader>
							<DialogTitle>{event.name} Result</DialogTitle>
						</DialogHeader>
						<ScrollArea className="max-h-[60vh]">
							<pre className="p-4 bg-muted rounded-lg overflow-x-auto">{JSON.stringify(event.result, null, 2)}</pre>
						</ScrollArea>
					</DialogContent>
				</Dialog>
			)}
		</ToolSummaryCard>
	)
}

const DefaultUpdateComponent = ({ event, accumulatedArgs, copyToClipboard }: { event: ToolEvent; accumulatedArgs: any; copyToClipboard: (content: string) => void }) => {
	return (
		<div className="text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="flex items-center gap-2 mb-3">
				<svg className="mr-3 -ml-1 size-5 animate-spin text-blue-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
					<circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
					<path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
				<span className="font-medium">Updating {event.name.replace(/_/g, ' ')}...</span>
			</div>
			<div className="mt-2 text-xs bg-background/50 p-2 rounded border border-border relative">
				<button
					onClick={() => copyToClipboard(JSON.stringify(accumulatedArgs, null, 2))}
					className="absolute top-2 right-2 p-1.5 rounded bg-muted hover:bg-muted/80 transition-colors"
					title="Copy accumulated arguments"
				>
					<Copy className="w-3 h-3" />
				</button>
				<div className="font-semibold mb-1">Updating {event.name.replace(/_/g, ' ')}:</div>
				<pre className="overflow-x-auto text-xs">{JSON.stringify(accumulatedArgs, null, 2)}</pre>
			</div>
		</div>
	)
}
