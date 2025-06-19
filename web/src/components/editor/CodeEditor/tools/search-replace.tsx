import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Copy, Replace } from 'lucide-react'
import { useMemo } from 'react'
import { ToolEvent } from './ToolEventRenderer'
import { ToolSummaryCard } from './ToolSummaryCard'

interface SearchReplaceUpdateProps {
	event: ToolEvent
	accumulatedArgs: any
	copyToClipboard: (content: string) => void
}

interface SearchReplaceResultProps {
	event: ToolEvent
	copyToClipboard: (content: string) => void
	copiedCode: string | null
}

export const SearchReplaceUpdate = ({ event, accumulatedArgs, copyToClipboard }: SearchReplaceUpdateProps) => {
	const filePath = accumulatedArgs.file_path || ''
	const oldString = accumulatedArgs.old_string || ''
	const newString = accumulatedArgs.new_string || ''

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
				<div className="font-semibold text-orange-600 flex items-center gap-2 mb-2">
					<Replace className="w-3 h-3" />
					Search and replace in: {filePath}
				</div>
				<div className="text-xs text-muted-foreground space-y-1">
					<div>
						Find: <span className="font-mono bg-red-100 px-1 rounded">{oldString}</span>
					</div>
					<div>
						Replace with: <span className="font-mono bg-green-100 px-1 rounded">{newString}</span>
					</div>
				</div>
			</div>
		</div>
	)
}

export const SearchReplaceResult = ({ event }: SearchReplaceResultProps) => {
	const result = event.result && typeof event.result === 'object' ? (event.result as any) : {}
	const resultArgs = useMemo(() => {
		const args = event.arguments && typeof event.arguments === 'string' ? JSON.parse(event.arguments) : {}
		return args
	}, [event.arguments])
	
	const filePath = resultArgs.file_path || ''
	const oldString = resultArgs.old_string || ''
	const newString = resultArgs.new_string || ''

	const summary = `Search and replace completed successfully in "${filePath}".`

	const details = (
		<Dialog>
			<DialogTrigger asChild>
				<Button variant="ghost" size="sm" className="h-6 text-xs">
					View Details
				</Button>
			</DialogTrigger>
			<DialogContent className="max-w-2xl">
				<DialogHeader>
					<DialogTitle>Search and Replace Details</DialogTitle>
				</DialogHeader>
				<ScrollArea className="max-h-[60vh]">
					<div className="space-y-4">
						<div className="p-3 bg-muted rounded-lg">
							<div className="font-semibold text-sm mb-2">File:</div>
							<div className="text-sm">{filePath}</div>
						</div>
						<div className="p-3 bg-muted rounded-lg">
							<div className="font-semibold text-sm mb-2">Search and Replace:</div>
							<div className="text-sm space-y-2">
								<div>
									<span className="font-semibold">Find:</span> <span className="font-mono bg-[#392426] text-[#ffd1d1] px-2 py-0.5 rounded">{oldString}</span>
								</div>
								<div>
									<span className="font-semibold">Replace with:</span> <span className="font-mono bg-[#1a2721] text-[#d1ffda] px-2 py-0.5 rounded">{newString}</span>
								</div>
							</div>
						</div>
						<div className="p-3 bg-green-50 border border-green-200 rounded-lg">
							<div className="font-semibold text-sm mb-2 text-green-700">Status:</div>
							<div className="text-sm text-green-600">Search and replace completed successfully</div>
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
