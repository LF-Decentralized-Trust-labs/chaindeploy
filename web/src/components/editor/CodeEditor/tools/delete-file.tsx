import React from 'react'
import { Copy, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { ToolEvent } from './ToolEventRenderer'
import { ToolSummaryCard } from './ToolSummaryCard'

interface DeleteFileUpdateProps {
	event: ToolEvent
	accumulatedArgs: any
	copyToClipboard: (content: string) => void
}

interface DeleteFileResultProps {
	event: ToolEvent
	copyToClipboard: (content: string) => void
	copiedCode: string | null
}

export const DeleteFileUpdate = ({ event, accumulatedArgs, copyToClipboard }: DeleteFileUpdateProps) => {
	const targetFile = accumulatedArgs.target_file || ''
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
				<div className="font-semibold text-red-600 flex items-center gap-2 mb-2">
					<Trash2 className="w-3 h-3" />
					Deleting file: {targetFile}
				</div>
				{explanation && (
					<div className="text-xs text-muted-foreground italic">
						{explanation}
					</div>
				)}
			</div>
		</div>
	)
}

export const DeleteFileResult = ({ event }: DeleteFileResultProps) => {
	const result = event.result && typeof event.result === 'object' ? event.result as any : {}
	const path = result.path || ''
	const resultMessage = result.result || ''

	const summary = resultMessage === 'File does not exist' 
		? `File "${path}" does not exist.`
		: `File "${path}" deleted successfully.`

	const details = (
		<Dialog>
			<DialogTrigger asChild>
				<Button variant="ghost" size="sm" className="h-6 text-xs">
					View Details
				</Button>
			</DialogTrigger>
			<DialogContent className="max-w-2xl">
				<DialogHeader>
					<DialogTitle>Delete File Details</DialogTitle>
				</DialogHeader>
				<ScrollArea className="max-h-[60vh]">
					<div className="space-y-4">
						<div className="p-3 bg-muted rounded-lg">
							<div className="font-semibold text-sm mb-2">File:</div>
							<div className="text-sm">{path}</div>
						</div>
						<div className={`p-3 rounded-lg border ${
							resultMessage === 'File does not exist' 
								? 'bg-yellow-50 border-yellow-200' 
								: 'bg-green-50 border-green-200'
						}`}>
							<div className={`font-semibold text-sm mb-2 ${
								resultMessage === 'File does not exist' ? 'text-yellow-700' : 'text-green-700'
							}`}>Status:</div>
							<div className={`text-sm ${
								resultMessage === 'File does not exist' ? 'text-yellow-600' : 'text-green-600'
							}`}>{resultMessage}</div>
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