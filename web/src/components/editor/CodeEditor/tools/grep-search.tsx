import React, { useMemo } from 'react'
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

interface GrepSearchExecuteProps {
	event: ToolEvent
}

export const GrepSearchExecute = ({ event }: GrepSearchExecuteProps) => {
	return (
		<div className="flex items-center gap-2 text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="animate-spin h-4 w-4 border-2 border-purple-500 border-t-transparent rounded-full" />
			<span className="font-medium">Executing grep search...</span>
		</div>
	)
}

export const GrepSearchUpdate = ({ event, accumulatedArgs, copyToClipboard }: GrepSearchUpdateProps) => {
	const args = useMemo(() => {
		if (event.arguments) {
			try {
				const parsed = JSON.parse(event.arguments)
				return parsed
			} catch (e) {
				return accumulatedArgs || {}
			}
		}
		return accumulatedArgs || {}
	}, [event.arguments, accumulatedArgs])
	
	const query = useMemo(() => args.query || '', [args.query])
	const includePattern = useMemo(() => args.include_pattern || '', [args.include_pattern])
	const excludePattern = useMemo(() => args.exclude_pattern || '', [args.exclude_pattern])
	const caseSensitive = useMemo(() => args.case_sensitive || false, [args.case_sensitive])
	const explanation = useMemo(() => args.explanation || '', [args.explanation])
	
	const handleCopyDelta = async () => {
		await copyToClipboard(JSON.stringify(args, null, 2))
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

export const GrepSearchResult = ({ event, copyToClipboard }: GrepSearchResultProps) => {
	const args = useMemo(() => {
		if (event.arguments) {
			try {
				const parsed = JSON.parse(event.arguments)
				return parsed
			} catch (e) {
				return {}
			}
		}
		return {}
	}, [event.arguments])
	
	const resultArgs = useMemo(() => {
		const result = event.result && typeof event.result === 'object' ? event.result as any : {}
		return result
	}, [event.result])
	
	const query = useMemo(() => args.query || resultArgs.query || '', [args.query, resultArgs.query])
	const explanation = useMemo(() => args.explanation || '', [args.explanation])
	const results = useMemo(() => resultArgs.results || [], [resultArgs.results])

	const summary = useMemo(() => `Found ${results.length} matches for "${query}".`, [results.length, query])

	const handleCopyArgs = async () => {
		await copyToClipboard(JSON.stringify(args, null, 2))
	}

	return (
		<ToolSummaryCard event={event}>
			<div className="space-y-3">
				{/* Summary Section */}
				<div className="text-sm text-muted-foreground mb-3">
					{summary}
				</div>

				{/* Search Query */}
				<div className="bg-background/50 p-3 rounded border border-border">
					<div className="font-semibold text-purple-600 flex items-center gap-2 mb-2">
						<Search className="w-3 h-3" />
						Search Query:
					</div>
					<div className="text-sm">{query}</div>
				</div>

				{/* Search Parameters */}
				{Object.keys(args).length > 0 && (
					<div className="bg-background/50 p-3 rounded border border-border">
						<div className="font-semibold text-sm mb-2">Search Parameters:</div>
						<div className="text-xs text-muted-foreground space-y-1">
							{args.include_pattern && <div>Include: {args.include_pattern}</div>}
							{args.exclude_pattern && <div>Exclude: {args.exclude_pattern}</div>}
							<div>Case sensitive: {args.case_sensitive ? 'Yes' : 'No'}</div>
						</div>
						<button 
							onClick={handleCopyArgs}
							className="mt-2 text-xs text-blue-600 hover:text-blue-800 flex items-center gap-1"
							title="Copy arguments"
						>
							<Copy className="w-3 h-3" />
							Copy Args
						</button>
					</div>
				)}

				{/* Results */}
				{results.length > 0 && (
					<div className="bg-background/50 p-3 rounded border border-border">
						<div className="font-semibold text-sm mb-2">Search Results:</div>
						<ScrollArea className="max-h-[300px]">
							<div className="space-y-2">
								{results.map((result: any, index: number) => (
									<div key={index} className="p-2 border rounded">
										<div className="font-semibold text-sm mb-1">{result.path}</div>
										{result.line_number && (
											<div className="text-xs text-muted-foreground mb-1">Line: {result.line_number}</div>
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
					</div>
				)}

				{/* Explanation */}
				{explanation && (
					<div className="text-xs text-muted-foreground italic">
						{explanation}
					</div>
				)}
			</div>
		</ToolSummaryCard>
	)
} 