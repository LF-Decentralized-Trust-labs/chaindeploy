import React from 'react'
import { Copy, Search } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { ToolEvent } from './ToolEventRenderer'
import { ToolSummaryCard } from './ToolSummaryCard'

interface CodebaseSearchUpdateProps {
	event: ToolEvent
	accumulatedArgs: any
	copyToClipboard: (content: string) => void
}

interface CodebaseSearchResultProps {
	event: ToolEvent
	copyToClipboard: (content: string) => void
	copiedCode: string | null
}

interface CodebaseSearchExecuteProps {
	event: ToolEvent
}

export const CodebaseSearchExecute = ({ event }: CodebaseSearchExecuteProps) => {
	return (
		<div className="flex items-center gap-2 text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="animate-spin h-4 w-4 border-2 border-purple-500 border-t-transparent rounded-full" />
			<span className="font-medium">Executing codebase search...</span>
		</div>
	)
}

export const CodebaseSearchUpdate = ({ event, accumulatedArgs, copyToClipboard }: CodebaseSearchUpdateProps) => {
	const query = accumulatedArgs.query || ''
	const targetDirectories = accumulatedArgs.target_directories || []
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
					Searching codebase: {query}
				</div>
				{targetDirectories.length > 0 && (
					<div className="text-xs text-muted-foreground mb-1">
						Directories: {targetDirectories.join(', ')}
					</div>
				)}
				{explanation && (
					<div className="text-xs text-muted-foreground italic">
						{explanation}
					</div>
				)}
			</div>
		</div>
	)
}

export const CodebaseSearchResult = ({ event }: CodebaseSearchResultProps) => {
	const query = event.result && typeof event.result === 'object' && 'query' in event.result 
		? (event.result as any).query 
		: ''
	
	const results = event.result && typeof event.result === 'object' && 'results' in event.result 
		? (event.result as any).results 
		: []

	const summary = `Found ${results.length} results for "${query}".`

	const details = (
		<Dialog>
			<DialogTrigger asChild>
				<Button variant="ghost" size="sm" className="h-6 text-xs">
					View Results
				</Button>
			</DialogTrigger>
			<DialogContent className="max-w-4xl">
				<DialogHeader>
					<DialogTitle>Codebase Search Results</DialogTitle>
				</DialogHeader>
				<ScrollArea className="max-h-[60vh]">
					<div className="space-y-4">
						{results.map((result: any, index: number) => (
							<div key={index} className="p-3 border rounded-lg">
								<div className="font-semibold text-sm mb-2">{result.file}</div>
								{result.score && (
									<div className="text-xs text-muted-foreground mb-2">Score: {result.score}</div>
								)}
								{result.content && (
									<pre className="text-xs bg-muted p-2 rounded overflow-x-auto">
										{result.content}
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