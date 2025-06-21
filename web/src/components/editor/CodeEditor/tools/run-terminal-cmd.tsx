import React from 'react'
import { Copy, Terminal } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { ToolEvent } from './ToolEventRenderer'
import { ToolSummaryCard } from './ToolSummaryCard'

interface RunTerminalCmdUpdateProps {
	event: ToolEvent
	accumulatedArgs: any
	copyToClipboard: (content: string) => void
}

interface RunTerminalCmdResultProps {
	event: ToolEvent
	copyToClipboard: (content: string) => void
	copiedCode: string | null
}

interface RunTerminalCmdExecuteProps {
	event: ToolEvent
}

export const RunTerminalCmdExecute = ({ event }: RunTerminalCmdExecuteProps) => {
	return (
		<div className="flex items-center gap-2 text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="animate-spin h-4 w-4 border-2 border-orange-500 border-t-transparent rounded-full" />
			<span className="font-medium">Executing terminal command...</span>
		</div>
	)
}

export const RunTerminalCmdUpdate = ({ event, accumulatedArgs, copyToClipboard }: RunTerminalCmdUpdateProps) => {
	const command = accumulatedArgs.command || ''
	const isBackground = accumulatedArgs.is_background || false
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
				<div className="font-semibold text-orange-600 flex items-center gap-2 mb-2">
					<Terminal className="w-3 h-3" />
					{isBackground ? 'Starting background command' : 'Preparing command'}: {command}
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

export const RunTerminalCmdResult = ({ event }: RunTerminalCmdResultProps) => {
	const result = event.result && typeof event.result === 'object' ? event.result as any : {}
	const command = result.command || ''
	const output = result.result || ''
	const error = result.error || ''
	const background = result.background || false
	const pid = result.pid

	const summary = background 
		? `Command started in background${pid ? ` (PID: ${pid})` : ''}.`
		: error 
			? `Command completed with error.`
			: `Command completed successfully.`

	const details = (
		<Dialog>
			<DialogTrigger asChild>
				<Button variant="ghost" size="sm" className="h-6 text-xs">
					View Output
				</Button>
			</DialogTrigger>
			<DialogContent className="max-w-4xl">
				<DialogHeader>
					<DialogTitle>Terminal Command Output</DialogTitle>
				</DialogHeader>
				<ScrollArea className="max-h-[60vh]">
					<div className="space-y-4">
						<div className="p-3 bg-muted rounded-lg">
							<div className="font-semibold text-sm mb-2">Command:</div>
							<pre className="text-sm bg-background p-2 rounded">{command}</pre>
						</div>
						{output && (
							<div className="p-3 bg-muted rounded-lg">
								<div className="font-semibold text-sm mb-2">Output:</div>
								<pre className="text-sm bg-background p-2 rounded overflow-x-auto">{output}</pre>
							</div>
						)}
						{error && (
							<div className="p-3 bg-red-50 border border-red-200 rounded-lg">
								<div className="font-semibold text-sm mb-2 text-red-700">Error:</div>
								<pre className="text-sm bg-background p-2 rounded overflow-x-auto text-red-600">{error}</pre>
							</div>
						)}
						{background && (
							<div className="p-3 bg-blue-50 border border-blue-200 rounded-lg">
								<div className="font-semibold text-sm mb-2 text-blue-700">Background Process:</div>
								<div className="text-sm">
									{pid && <div>PID: {pid}</div>}
									<div>Status: Running in background</div>
								</div>
							</div>
						)}
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