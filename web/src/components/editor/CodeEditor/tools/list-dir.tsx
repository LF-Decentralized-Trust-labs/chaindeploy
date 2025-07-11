import React from 'react'
import { Copy, FolderOpen } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { ToolEvent } from './ToolEventRenderer'
import { ToolSummaryCard } from './ToolSummaryCard'

interface ListDirUpdateProps {
	event: ToolEvent
	accumulatedArgs: any
	copyToClipboard: (content: string) => void
}

interface ListDirResultProps {
	event: ToolEvent
	copyToClipboard: (content: string) => void
	copiedCode: string | null
}

interface ListDirExecuteProps {
	event: ToolEvent
}

export const ListDirExecute = ({ event }: ListDirExecuteProps) => {
	return (
		<div className="flex items-center gap-2 text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="animate-spin h-4 w-4 border-2 border-green-500 border-t-transparent rounded-full" />
			<span className="font-medium">Executing directory listing...</span>
		</div>
	)
}

export const ListDirUpdate = ({ event, accumulatedArgs, copyToClipboard }: ListDirUpdateProps) => {
	const relativePath = accumulatedArgs.relative_workspace_path || ''
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
				<div className="font-semibold text-green-600 flex items-center gap-2 mb-2">
					<FolderOpen className="w-3 h-3" />
					Listing directory: {relativePath || 'root'}
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

export const ListDirResult = ({ event }: ListDirResultProps) => {
	const result = event.result && typeof event.result === 'object' ? event.result as any : {}
	const path = result.path || ''
	const items = result.items || []

	const summary = `Found ${items.length} items in "${path}".`

	return (
		<ToolSummaryCard event={event}>
			<div className="space-y-3">
				{/* Summary Section */}
				<div className="text-sm text-muted-foreground mb-3">
					{summary}
				</div>

				{/* Directory Path */}
				<div className="bg-background/50 p-3 rounded border border-border">
					<div className="font-semibold text-green-600 flex items-center gap-2 mb-2">
						<FolderOpen className="w-3 h-3" />
						Directory:
					</div>
					<div className="text-sm">{path}</div>
				</div>

				{/* Directory Contents */}
				{items.length > 0 && (
					<div className="bg-background/50 p-3 rounded border border-border">
						<div className="font-semibold text-sm mb-2">Contents:</div>
						<ScrollArea className="max-h-[300px]">
							<div className="space-y-2">
								{items.map((item: any, index: number) => (
									<div key={index} className="flex items-center justify-between p-2 border rounded">
										<div className="flex items-center gap-2">
											<span className={item.is_dir ? 'text-blue-500' : 'text-gray-500'}>
												{item.is_dir ? '📁' : '📄'}
											</span>
											<span className="font-medium">{item.name}</span>
										</div>
										{!item.is_dir && item.size !== undefined && (
											<span className="text-xs text-muted-foreground">
												{(item.size / 1024).toFixed(1)} KB
											</span>
										)}
									</div>
								))}
							</div>
						</ScrollArea>
					</div>
				)}
			</div>
		</ToolSummaryCard>
	)
} 