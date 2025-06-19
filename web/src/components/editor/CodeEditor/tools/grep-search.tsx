import React from 'react'
import { Copy, Search } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { ToolEvent } from './ToolEventRenderer'
import { ToolSummaryCard } from './ToolSummaryCard'

interface GrepSearchUpdateProps {
	event: ToolEvent
	accumulatedArgs: any
	copyToClipboard: (content: string) => void
}

interface GrepSearchResultProps {
	event: ToolEvent
	copyToClipboard: (content: string) => void
	copiedCode: string | null
}

export const GrepSearchUpdate = ({ event, accumulatedArgs, copyToClipboard }: GrepSearchUpdateProps) => {
	const query = accumulatedArgs.query || ''
	const includePattern = accumulatedArgs.include_pattern || ''
	const excludePattern = accumulatedArgs.exclude_pattern || ''
	const caseSensitive = accumulatedArgs.case_sensitive || false
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
				<div className="font-semibold text-purple-600 flex items-center gap-2 mb-2">
					<Search className="w-3 h-3" />
					Searching: {query}
				</div>
				<div className="text-xs text-muted-foreground space-y-1">
					{includePattern && <div>Include: {includePattern}</div>}
					{excludePattern && <div>Exclude: {excludePattern}</div>}
					<div>Case sensitive: {caseSensitive ? 'Yes' : 'No'}</div>
				</div>
				{explanation && (
					<div className="text-xs text-muted-foreground italic mt-1">
						{explanation}
					</div>
				)}
			</div>
		</div>
	)
}

export const GrepSearchResult = ({ event }: GrepSearchResultProps) => {
	const result = event.result && typeof event.result === 'object' ? event.result as any : {}
	const query = result.query || ''
	const results = result.results || []

	const summary = `Found ${results.length} matches for "${query}".`

	const details = (
		<Dialog>
			<DialogTrigger asChild>
				<Button variant="ghost" size="sm" className="h-6 text-xs">
					View Results
				</Button>
			</DialogTrigger>
			<DialogContent className="max-w-4xl">
				<DialogHeader>
					<DialogTitle>Grep Search Results</DialogTitle>
				</DialogHeader>
				<ScrollArea className="max-h-[60vh]">
					<div className="space-y-4">
						{results.map((result: any, index: number) => (
							<div key={index} className="p-3 border rounded-lg">
								<div className="font-semibold text-sm mb-2">{result.path}</div>
								{result.line_number && (
									<div className="text-xs text-muted-foreground mb-2">Line: {result.line_number}</div>
								)}
								{result.lines && (
									<pre className="text-xs bg-muted p-2 rounded overflow-x-auto">
										{result.lines}
									</pre>
								)}
							</div>
						))}
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