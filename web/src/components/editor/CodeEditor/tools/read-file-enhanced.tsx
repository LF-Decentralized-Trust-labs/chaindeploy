import { ScrollArea } from '@/components/ui/scroll-area'
import { getLanguage } from '@/lib/language'
import { Copy, FileText } from 'lucide-react'
import { LazyCodeBlock } from './lazy-code-block'
import { ToolEvent } from './ToolEventRenderer'
import { ToolSummaryCard } from './ToolSummaryCard'

interface ReadFileEnhancedUpdateProps {
	event: ToolEvent
	accumulatedArgs: any
	copyToClipboard: (content: string) => void
}

interface ReadFileEnhancedResultProps {
	event: ToolEvent
	copyToClipboard: (content: string) => void
	copiedCode: string | null
}

interface ReadFileEnhancedExecuteProps {
	event: ToolEvent
}

export const ReadFileEnhancedExecute = ({ event }: ReadFileEnhancedExecuteProps) => {
	return (
		<div className="flex items-center gap-2 text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="animate-spin h-4 w-4 border-2 border-blue-500 border-t-transparent rounded-full" />
			<span className="font-medium">Executing enhanced file read...</span>
		</div>
	)
}

export const ReadFileEnhancedUpdate = ({ event, accumulatedArgs, copyToClipboard }: ReadFileEnhancedUpdateProps) => {
	const targetFile = accumulatedArgs.target_file || ''
	const shouldReadEntireFile = accumulatedArgs.should_read_entire_file || false
	const startLine = accumulatedArgs.start_line_one_indexed || 1
	const endLine = accumulatedArgs.end_line_one_indexed_inclusive || 1
	const explanation = accumulatedArgs.explanation || ''
	
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
			<div className="mt-2 text-xs bg-background/50 p-2 rounded border border-border relative">
				<button onClick={handleCopyDelta} className="absolute top-2 right-2 p-1.5 rounded bg-muted hover:bg-muted/80 transition-colors" title="Copy arguments">
					<Copy className="w-3 h-3" />
				</button>
				<div className="font-semibold text-blue-600 flex items-center gap-2 mb-2">
					<FileText className="w-3 h-3" />
					Reading file: {targetFile}
				</div>
				{shouldReadEntireFile ? (
					<div className="text-xs text-muted-foreground">Reading entire file</div>
				) : (
					<div className="text-xs text-muted-foreground">Lines {startLine} to {endLine}</div>
				)}
				{explanation && (
					<div className="text-xs text-muted-foreground italic mt-1">
						{explanation}
					</div>
				)}
			</div>
		</div>
	)
}

export const ReadFileEnhancedResult = ({ event, copyToClipboard, copiedCode }: ReadFileEnhancedResultProps) => {
	const result = event.result && typeof event.result === 'object' ? event.result as any : {}
	const content = result.content || ''
	const filePath = result.file_path || ''
	const totalLines = result.total_lines || 0
	const startLine = result.start_line || 1
	const endLine = result.end_line || totalLines
	const language = getLanguage(filePath)

	const summary = totalLines > 0 
		? `Read ${endLine - startLine + 1} lines from "${filePath}" (${totalLines} total lines).`
		: `Read file "${filePath}".`

	return (
		<ToolSummaryCard event={event}>
			<div className="space-y-3">
				{/* Summary Section */}
				<div className="text-sm text-muted-foreground mb-3">
					{summary}
				</div>

				{/* File Info */}
				<div className="bg-background/50 p-3 rounded border border-border">
					<div className="font-semibold text-sm mb-2">File Info:</div>
					<div className="text-sm space-y-1">
						<div>Path: {filePath}</div>
						<div>Total Lines: {totalLines}</div>
						{startLine !== 1 || endLine !== totalLines ? (
							<div>Lines Read: {startLine} - {endLine}</div>
						) : null}
					</div>
				</div>

				{/* File Content */}
				{content && (
					<div className="bg-background/50 p-3 rounded border border-border">
						<div className="font-semibold text-sm mb-2 flex items-center justify-between">
							<span>File Content:</span>
							<button 
								onClick={() => copyToClipboard(content)} 
								className="p-1.5 rounded bg-muted hover:bg-muted/80 transition-colors" 
								title="Copy content"
							>
								{copiedCode === content ? <span className="text-green-500">✓</span> : <Copy className="w-4 h-4" />}
							</button>
						</div>
						<ScrollArea className="max-h-[400px] overflow-auto">
							<LazyCodeBlock
								code={content}
								language={language}
							/>
						</ScrollArea>
					</div>
				)}
			</div>
		</ToolSummaryCard>
	)
} 