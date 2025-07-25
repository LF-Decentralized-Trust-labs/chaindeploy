import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Copy, RefreshCw } from 'lucide-react'
import { ToolEvent } from './ToolEventRenderer'
import { ToolSummaryCard } from './ToolSummaryCard'

interface ReapplyUpdateProps {
	event: ToolEvent
	accumulatedArgs: any
	copyToClipboard: (content: string) => void
}

interface ReapplyResultProps {
	event: ToolEvent
	copyToClipboard: (content: string) => void
	copiedCode: string | null
}

interface ReapplyExecuteProps {
	event: ToolEvent
}

export const ReapplyExecute = ({ event }: ReapplyExecuteProps) => {
	return (
		<div className="flex items-center gap-2 text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="animate-spin h-4 w-4 border-2 border-blue-500 border-t-transparent rounded-full" />
			<span className="font-medium">Executing reapply...</span>
		</div>
	)
}

export const ReapplyUpdate = ({ event, accumulatedArgs, copyToClipboard }: ReapplyUpdateProps) => {
	const targetFile = accumulatedArgs.target_file || ''

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
					<RefreshCw className="w-3 h-3" />
					Reapplying last edit to: {targetFile}
				</div>
			</div>
		</div>
	)
}

export const ReapplyResult = ({ event }: ReapplyResultProps) => {
	const result = event.result && typeof event.result === 'object' ? (event.result as any) : {}
	const targetFile = result.target_file || ''
	const resultMessage = result.result || ''

	const summary = `Reapply operation completed for "${targetFile}".`

	return (
		<ToolSummaryCard event={event}>
			<div className="space-y-3">
				{/* Summary Section */}
				<div className="text-sm text-muted-foreground mb-3">
					{summary}
				</div>

				{/* Target File */}
				<div className="bg-background/50 p-3 rounded border border-border">
					<div className="font-semibold text-sm mb-2">Target File:</div>
					<div className="text-sm">{targetFile}</div>
				</div>

				{/* Status */}
				<div className="p-3 bg-blue-50 border border-blue-200 rounded">
					<div className="font-semibold text-sm mb-2 text-blue-700">Status:</div>
					<div className="text-sm text-blue-600">{resultMessage}</div>
				</div>
			</div>
		</ToolSummaryCard>
	)
}
