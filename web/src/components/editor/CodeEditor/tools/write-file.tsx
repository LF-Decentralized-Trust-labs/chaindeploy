import { getLanguage } from '@/lib/language'
import { Check, Copy, FileText } from 'lucide-react'
import { useEffect, useMemo, useRef, useCallback, useState } from 'react'
import { ToolEvent } from './ToolEventRenderer'
import { ToolSummaryCard } from './ToolSummaryCard'
import { LazyCodeBlock } from './lazy-code-block'

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
		<div className="flex items-center gap-2 text-sm text-muted-foreground bg-muted/50 rounded-lg border border-border">
			<div className="animate-spin h-4 w-4 border-2 border-green-500 border-t-transparent rounded-full" />
			<span className="font-medium">Executing file write...</span>
		</div>
	)
}

export const WriteFileUpdate = ({ event, accumulatedArgs, copyToClipboard }: WriteFileUpdateProps) => {
	const targetFile = useMemo(() => accumulatedArgs.target_file || '', [accumulatedArgs.target_file])
	const content = useMemo(() => accumulatedArgs.content || '', [accumulatedArgs.content])
	const language = useMemo(() => getLanguage(targetFile), [targetFile])
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
		<div className="text-sm text-muted-foreground bg-muted/50  rounded-lg border border-border">
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
							<LazyCodeBlock code={content} language={language} />
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
	const path = useMemo(() => resultArgs.path || '', [resultArgs.path])
	const content = useMemo(() => resultArgs.content || '', [resultArgs.content])
	const language = useMemo(() => getLanguage(path), [path])
	const [copied, setCopied] = useState<string | null>(null)
	const handleCopy = useCallback(() => {
		copyToClipboard(content)
		setCopied(content)
		setTimeout(() => setCopied(null), 2000)
	}, [content, copyToClipboard])
	return (
		<div className="max-w-xl w-full ">
			<ToolSummaryCard event={event}>
				<div className="flex items-center bg-muted px-3 py-2 rounded-t text-muted-foreground text-xs font-mono gap-2">
					<FileText className="w-4 h-4" />
					<span className="truncate">{path}</span>
					<button onClick={handleCopy} className="ml-auto p-1.5 rounded hover:bg-background transition-colors" title="Copy code">
						{copied === content ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
					</button>
				</div>
				<div className="w-full whitespace-pre-wrap break-words bg-background rounded-b p-4 text-xs">
					<LazyCodeBlock code={content} language={language} previewLines={5} />
				</div>
			</ToolSummaryCard>
		</div>
	)
}
