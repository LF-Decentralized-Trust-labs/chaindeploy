import { ScrollArea } from '@/components/ui/scroll-area'
import { getLanguage } from '@/lib/language'
import { Check, Copy, Edit, FileText } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { LazyCodeBlock } from './lazy-code-block'
import { ToolEvent } from './ToolEventRenderer'
import { ToolSummaryCard } from './ToolSummaryCard'
import React from 'react'

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

// Add localStorage persistence for edit form and results
const STORAGE_KEY = 'edit-file-tool-state'

export const EditFileExecute = ({ event }: EditFileExecuteProps) => {
	const args = useMemo(() => (event.arguments ? JSON.parse(event.arguments) : {}), [event.arguments])
	const code = args.search_replace_blocks || ''
	const codeLines = code ? code.split('\n') : []
	const lastLines = codeLines.slice(-10).join('\n')
	return (
		<div className="flex flex-col gap-2 text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="flex items-center gap-2">
				<div className="animate-spin h-4 w-4 border-2 border-yellow-500 border-t-transparent rounded-full" />
				<span className="font-medium">Applying changes...</span>
			</div>
			{code && (
				<div className="mt-2 text-xs bg-background/50 p-2 rounded max-h-40 overflow-y-auto">
					<pre className="whitespace-pre-wrap break-words font-mono">{lastLines}</pre>
				</div>
			)}
		</div>
	)
}

// Memoize EditFileUpdate to avoid unnecessary re-renders
export const EditFileUpdate = React.memo(({ event, accumulatedArgs, copyToClipboard }: EditFileUpdateProps) => {
	const targetFile = useMemo(() => accumulatedArgs.target_file || '', [accumulatedArgs.target_file])
	const instructions = useMemo(() => accumulatedArgs.instructions || '', [accumulatedArgs.instructions])
	const codeEdit = useMemo(() => accumulatedArgs.search_replace_blocks || '', [accumulatedArgs.search_replace_blocks])
	const language = getLanguage(targetFile)
	const scrollContainerRef = useRef<HTMLDivElement>(null)

	// Only scroll when codeEdit actually changes
	const prevCodeEditRef = useRef<string>()
	useEffect(() => {
		if (scrollContainerRef.current && codeEdit && prevCodeEditRef.current !== codeEdit) {
			scrollContainerRef.current.scrollTop = scrollContainerRef.current.scrollHeight
			prevCodeEditRef.current = codeEdit
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
							<LazyCodeBlock code={codeEdit} language={language} />
						</div>
					) : (
						<div className="p-3 text-muted-foreground italic flex items-center justify-center h-[200px] w-full">Waiting for content...</div>
					)}
				</div>
			</div>
		</div>
	)
})

function DiffBlock({ code }: { code: string }) {
	const [expanded, setExpanded] = React.useState(false)
	const MAX_LINES = 30
	const CONTEXT_BEFORE = 5
	// Parse the code for conflict markers
	const originalMatch = code.match(/<<<<<<< ORIGINAL([\s\S]*?)=======/)
	const updatedMatch = code.match(/=======[\s\S]*?>>>>>>> UPDATED/)
	const original = originalMatch ? originalMatch[1].trim().split('\n') : []
	const updated = updatedMatch ? updatedMatch[0].replace('=======', '').replace('>>>>>>> UPDATED', '').trim().split('\n') : []

	// Simple line diff algorithm for unified diff
	function getUnifiedDiffLines(orig: string[], upd: string[]) {
		const lines: { type: 'remove' | 'add' | 'equal'; value: string }[] = []
		let i = 0,
			j = 0
		while (i < orig.length || j < upd.length) {
			if (i < orig.length && j < upd.length && orig[i] === upd[j]) {
				lines.push({ type: 'equal', value: orig[i] })
				i++
				j++
			} else if (j < upd.length && !orig.includes(upd[j])) {
				lines.push({ type: 'add', value: upd[j] })
				j++
			} else if (i < orig.length && !upd.includes(orig[i])) {
				lines.push({ type: 'remove', value: orig[i] })
				i++
			} else {
				// fallback: treat as changed
				if (i < orig.length) lines.push({ type: 'remove', value: orig[i] })
				if (j < upd.length) lines.push({ type: 'add', value: upd[j] })
				i++
				j++
			}
		}
		return lines
	}

	let lines: { type: 'remove' | 'add' | 'equal'; value: string }[] = []
	if (original.length && updated.length) {
		lines = getUnifiedDiffLines(original, updated)
	} else if (original.length) {
		lines = original.map((l) => ({ type: 'remove', value: l }))
	} else if (updated.length) {
		lines = updated.map((l) => ({ type: 'add', value: l }))
	}
	// Find the first changed line
	const firstChangeIdx = lines.findIndex((l) => l.type === 'add' || l.type === 'remove')
	let previewLines = lines
	if (!expanded && lines.length > MAX_LINES && firstChangeIdx !== -1) {
		const start = Math.max(0, firstChangeIdx - CONTEXT_BEFORE)
		previewLines = lines.slice(start, start + MAX_LINES)
	} else if (!expanded && lines.length > MAX_LINES) {
		previewLines = lines.slice(0, MAX_LINES)
	}
	return (
		<div>
			<pre className="bg-background p-2 rounded border border-border text-xs overflow-x-auto whitespace-pre font-mono">
				{previewLines.map((line, idx) => (
					<div
						key={idx}
						className={
							line.type === 'add'
								? 'bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300'
								: line.type === 'remove'
									? 'bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300'
									: ''
						}
					>
						{line.type === 'add' ? '+' : line.type === 'remove' ? '-' : ' '}
						{line.value}
					</div>
				))}
			</pre>
			{lines.length > MAX_LINES && (
				<button className="text-xs underline mt-1" onClick={() => setExpanded((v) => !v)}>
					{expanded ? 'Hide diff' : `Show full diff (${lines.length} lines)`}
				</button>
			)}
		</div>
	)
}

// Memoize EditFileResult to avoid unnecessary re-renders
export const EditFileResult = React.memo(({ event }: EditFileResultProps) => {
	const resultArgs = useMemo(() => {
		const args = event.arguments && typeof event.arguments === 'string' ? JSON.parse(event.arguments) : {}
		return args
	}, [event.arguments])
	const [copiedCode, setCopiedCode] = useState<string | null>(null)
	const filePath = useMemo(() => resultArgs.target_file || '', [resultArgs.target_file])
	const code = resultArgs.search_replace_blocks || ''
	const language = getLanguage(filePath)
	const copyToClipboard = useCallback((content: string) => {
		navigator.clipboard.writeText(content)
		setCopiedCode(content)
		setTimeout(() => setCopiedCode(null), 2000)
	}, [])

	// Add localStorage load on mount
	useEffect(() => {
		const saved = localStorage.getItem(STORAGE_KEY)
		if (saved) {
			try {
				const parsed = JSON.parse(saved)
				// Restore form state and results if needed
				// (You may need to adapt this to your actual form state structure)
				// setFn(parsed.fn || '')
				// setArgs(parsed.args || '')
				// setSelectedKey(parsed.selectedKey)
				// setResponses(parsed.responses || [])
			} catch {}
		}
	}, [])
	// Save state to localStorage on change
	useEffect(() => {
		try {
			localStorage.setItem(
				STORAGE_KEY,
				JSON.stringify({
					// fn,
					// args,
					// selectedKey,
					// responses,
					// ...add any other state you want to persist
				})
			)
		} catch {}
	}, [/* dependencies: fn, args, selectedKey, responses, etc. */])

	return (
		<div className="max-w-xl w-full mx-2">
			<ToolSummaryCard event={event}>
				<div className="flex items-center bg-muted px-3 py-2 rounded-t text-muted-foreground text-xs font-mono gap-2">
					<FileText className="w-5 h-5" />
					<span className="truncate">File {filePath} updated</span>
					<button onClick={() => copyToClipboard(code)} className="ml-auto p-1.5 rounded hover:bg-background transition-colors" title="Copy code">
						{copiedCode === code ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
					</button>
				</div>
				<div className="w-full whitespace-pre-wrap break-words bg-background rounded-b p-4 text-xs">
					{code.includes('<<<<<<< ORIGINAL') && code.includes('=======') && code.includes('>>>>>>> UPDATED') ? (
						<DiffBlock code={code} />
					) : (
						<LazyCodeBlock code={code} language={language} previewLines={5} />
					)}
				</div>
			</ToolSummaryCard>
		</div>
	)
})
