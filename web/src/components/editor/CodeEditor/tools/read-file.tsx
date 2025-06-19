import { Code, Copy, Check } from 'lucide-react'
import { ToolEvent } from './ToolEventRenderer'
import { ToolSummaryCard } from './ToolSummaryCard'
import { useMemo } from 'react'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import type { SyntaxHighlighterProps } from 'react-syntax-highlighter'
import SyntaxHighlighter from 'react-syntax-highlighter'
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism'
import { getLanguage } from '@/lib/language'

const SyntaxHighlighterComp = SyntaxHighlighter as unknown as React.ComponentType<SyntaxHighlighterProps>

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

export const ReadFileUpdate = ({ event, accumulatedArgs, copyToClipboard }: ReadFileUpdateProps) => {
	const path = accumulatedArgs.path || ''

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
				<div className="font-semibold text-blue-600 flex items-center gap-2">
					<Code className="w-3 h-3" />
					Reading file: {path || 'Unknown file'}
				</div>
			</div>
		</div>
	)
}

export const ReadFileResult = ({ event, copyToClipboard, copiedCode }: ReadFileResultProps) => {
	const resultArgs = useMemo(() => {
		const args = event.arguments && typeof event.arguments === 'string' ? JSON.parse(event.arguments) : {}
		return args
	}, [event.arguments])
	
	const path = resultArgs.path || ''
	const result = event.result && typeof event.result === 'object' ? (event.result as any) : {}
	const content = result.content || ''

	const summary = `The file "${path}" has been read successfully.`

	const details = (
		<Dialog>
			<DialogTrigger asChild>
				<Button variant="ghost" size="sm" className="h-6 text-xs">
					View Content
				</Button>
			</DialogTrigger>
			<DialogContent className="max-w-2xl">
				<DialogHeader>
					<DialogTitle>File Content: {path}</DialogTitle>
				</DialogHeader>
				<ScrollArea className="max-h-[60vh]">
					<div className="space-y-4">
						<div className="p-3 bg-muted rounded-lg">
							<div className="font-semibold text-sm mb-2">File:</div>
							<div className="text-sm">{path}</div>
						</div>
						<div className="relative group">
							<div className="absolute right-2 top-2 opacity-0 group-hover:opacity-100 transition-opacity">
								<button onClick={() => copyToClipboard(content)} className="p-1.5 rounded bg-muted hover:bg-muted/80 transition-colors" title="Copy content">
									{copiedCode === content ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
								</button>
							</div>
							<SyntaxHighlighterComp
								language={getLanguage(path)}
								style={vscDarkPlus}
								PreTag="div"
								className="rounded-lg"
								showLineNumbers={true}
								wrapLines={true}
								wrapLongLines={true}
								customStyle={{ margin: 0, padding: '1rem' }}
							>
								{content}
							</SyntaxHighlighterComp>
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
