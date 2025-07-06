import { ScrollArea } from '@/components/ui/scroll-area'
import { jsonrepair } from 'jsonrepair'
import { Copy } from 'lucide-react'
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { toast } from 'sonner'
import { ToolExecuteComponent } from './ToolExecuteComponent'
import { ToolStartComponent } from './ToolStartComponent'
import { ToolSummaryCard } from './ToolSummaryCard'

// Import all tool components
import { CodebaseSearchExecute, CodebaseSearchResult, CodebaseSearchUpdate } from './codebase-search'
import { DeleteFileExecute, DeleteFileResult, DeleteFileUpdate } from './delete-file'
import { EditFileExecute, EditFileResult, EditFileUpdate } from './edit-file'
import { FileExistsExecute, FileExistsResult, FileExistsUpdate } from './file-exists'
import { FileSearchExecute, FileSearchResult, FileSearchUpdate } from './file-search'
import { GrepSearchExecute, GrepSearchResult, GrepSearchUpdate } from './grep-search'
import { ListDirExecute, ListDirResult, ListDirUpdate } from './list-dir'
import { ReadFileExecute, ReadFileResult, ReadFileUpdate } from './read-file'
import { ReadFileEnhancedExecute, ReadFileEnhancedResult, ReadFileEnhancedUpdate } from './read-file-enhanced'
import { ReapplyExecute, ReapplyResult, ReapplyUpdate } from './reapply'
import { RewriteFileExecute, RewriteFileResult, RewriteFileUpdate } from './rewrite-file'
import { RunTerminalCmdExecute, RunTerminalCmdResult, RunTerminalCmdUpdate } from './run-terminal-cmd'
import { SearchReplaceExecute, SearchReplaceResult, SearchReplaceUpdate } from './search-replace'
import { WriteFileExecute, WriteFileResult, WriteFileUpdate } from './write-file'

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
	file_exists: { update: FileExistsUpdate, result: FileExistsResult, execute: FileExistsExecute },
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
	rewrite_file: { update: RewriteFileUpdate, result: RewriteFileResult, execute: RewriteFileExecute },
}

// Memoized parseDelta for expensive JSON parsing
const useParsedDelta = (deltaString?: string) => {
	return useMemo(() => {
		if (!deltaString) return null
		try {
			return JSON.parse(jsonrepair(deltaString))
		} catch (e) {
			try {
				const repaired = jsonrepair(deltaString)
				return JSON.parse(repaired)
			} catch (e) {
				return null
			}
		}
	}, [deltaString])
}

export const ToolEventRenderer = React.memo(
	({ event }: ToolEventProps) => {
		const [copiedCode, setCopiedCode] = useState<string | null>(null)
		const previousDeltaRef = useRef<any>(null)
		// Reset previous delta when tool event changes (new tool call starts)
		useEffect(() => {
			if (event.type === 'start') {
				previousDeltaRef.current = null
			}
		}, [event.toolCallID, event.type])

		// Memoized parseDelta usage
		const parsedArguments = useParsedDelta(event.arguments)

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
					if (parsedArguments !== null) {
						previousDeltaRef.current = parsedArguments
						accumulatedArgs = parsedArguments
					}
					return <UpdateComponent event={event} accumulatedArgs={accumulatedArgs} copyToClipboard={copyToClipboard} />
				}
				// Fallback for unknown tools
				let accumulatedArgs = previousDeltaRef.current || {}
				if (parsedArguments !== null) {
					previousDeltaRef.current = parsedArguments
					accumulatedArgs = parsedArguments
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
		}, [event, parsedArguments, copyToClipboard, copiedCode])

		return content
	},
	(prevProps, nextProps) => {
		// Only re-render if relevant event fields change
		const prev = prevProps.event
		const next = nextProps.event
		return (
			prev.toolCallID === next.toolCallID && prev.type === next.type && prev.name === next.name && prev.arguments === next.arguments && prev.result === next.result && prev.error === next.error
		)
	}
)

// Memoize fallback components for further optimization
const DefaultResultComponent = React.memo(({ event, copyToClipboard, copiedCode }: { event: ToolEvent; copyToClipboard: (content: string) => void; copiedCode: string | null }) => {
	const summary = `${event.name.replace(/_/g, ' ')} completed successfully.`
	return (
		<div className="max-w-xl w-full mx-2">
			<ToolSummaryCard event={event}>
				<div className="space-y-3">
					{/* Summary Section */}
					<div className="text-sm text-muted-foreground mb-3">{summary}</div>
					{/* Result Content */}
					{event.result && (
						<div className="bg-background/50 p-3 rounded border border-border">
							<div className="font-semibold text-sm mb-2">Result:</div>
							<pre className="text-xs bg-muted p-2 rounded overflow-x-auto whitespace-pre-wrap break-words">{JSON.stringify(event.result, null, 2)}</pre>
						</div>
					)}
				</div>
			</ToolSummaryCard>
		</div>
	)
})

const DefaultUpdateComponent = React.memo(({ event, accumulatedArgs, copyToClipboard }: { event: ToolEvent; accumulatedArgs: any; copyToClipboard: (content: string) => void }) => {
	return (
		<div className="max-w-xl w-full mx-2">
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
					<pre className="overflow-x-auto text-xs whitespace-pre-wrap break-words">{JSON.stringify(accumulatedArgs, null, 2)}</pre>
				</div>
			</div>
		</div>
	)
})

// NOTE: If you ever render a large list of ToolEventRenderer, consider virtualizing the list for best performance.
