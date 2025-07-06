import { Code, Copy, FileText, ChevronDown, ChevronUp } from 'lucide-react'
import { useMemo, useState } from 'react'
import { ToolEvent } from './ToolEventRenderer'
import { ToolSummaryCard } from './ToolSummaryCard'
import { LazyCodeBlock } from './lazy-code-block'

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
	return (
		<div className="flex items-center gap-2 text-xs text-muted-foreground bg-muted/50 p-2 rounded border border-border">
			<FileText className="w-4 h-4" />
			<span>Reading file <span className="font-mono">{path}</span></span>
		</div>
	)
}

export const ReadFileResult = ({ event, copyToClipboard, copiedCode }: ReadFileResultProps) => {
	const resultArgs = useMemo(() => {
		const args = event.arguments && typeof event.arguments === 'string' ? JSON.parse(event.arguments) : {}
		return args
	}, [event.arguments])
	const path = useMemo(() => resultArgs.target_file || '', [resultArgs.target_file])
	const [showDetails, setShowDetails] = useState(false)
	const result = event.result && typeof event.result === 'object' ? event.result as any : {}
	return (
		<div className="bg-muted/50 p-2 rounded border border-border text-xs text-muted-foreground">
			<div className="flex items-center gap-2">
				<FileText className="w-4 h-4" />
				<span>File <span className="font-mono">{path}</span> read</span>
				<button onClick={() => setShowDetails((v) => !v)} className="ml-2 p-1 rounded hover:bg-muted">
					{showDetails ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
				</button>
			</div>
			{showDetails && (
				<div className="mt-2 w-full space-y-2">
					<div className="flex flex-col gap-1">
						<div><span className="font-semibold">File:</span> <span className="font-mono">{result.file_path}</span></div>
						<div><span className="font-semibold">Lines:</span> {result.start_line} - {result.end_line} (<span className="font-mono">{result.total_lines}</span> total)</div>
					</div>
					{result.content && (
						<LazyCodeBlock code={result.content} language="plaintext" previewLines={10} className="mt-1" />
					)}
				</div>
			)}
		</div>
	)
}
