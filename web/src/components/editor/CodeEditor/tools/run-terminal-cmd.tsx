import React, { useMemo } from 'react'
import { Copy, Terminal, AlertCircle, CheckCircle } from 'lucide-react'
import { Prism as SyntaxHighlighter, SyntaxHighlighterProps } from 'react-syntax-highlighter'
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { ToolEvent } from './ToolEventRenderer'
import { ToolSummaryCard } from './ToolSummaryCard'
const SyntaxHighlighterComp = SyntaxHighlighter as unknown as React.ComponentType<SyntaxHighlighterProps>
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
	const isGoVet = useMemo(() => {
		if (typeof event.args === 'string') {
			try {
				const args = JSON.parse(event.args)
				return args.command?.includes('go vet')
			} catch (e) {
				return false
			}
		}
		return false
	}, [event.args])

	return (
		<div className="flex items-center gap-2 text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="animate-spin h-4 w-4 border-2 border-orange-500 border-t-transparent rounded-full" />
			<span className="font-medium">{isGoVet ? 'Running Go vet analysis...' : 'Executing terminal command...'}</span>
		</div>
	)
}

export const RunTerminalCmdUpdate = ({ event, accumulatedArgs, copyToClipboard }: RunTerminalCmdUpdateProps) => {
	const args = useMemo(() => {
		if (typeof accumulatedArgs === 'string') {
			try {
				const parsed = JSON.parse(accumulatedArgs)
				return parsed && typeof parsed === 'object' ? parsed : {}
			} catch (e) {
				return {}
			}
		}
		return accumulatedArgs && typeof accumulatedArgs === 'object' ? accumulatedArgs : {}
	}, [accumulatedArgs])

	const command = args.command || ''
	const isBackground = args.is_background || false
	const explanation = args.explanation || ''
	const output = args.output || ''
	const error = args.error || ''

	const handleCopyDelta = async () => {
		await copyToClipboard(JSON.stringify(args, null, 2))
	}

	const isGoVet = command.includes('go vet')
	const statusText = isGoVet ? 'Analyzing Go code for issues...' : `Updating ${event.name.replace(/_/g, ' ')}...`

	return (
		<div className="text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="flex items-center gap-2 mb-3">
				<svg className="mr-3 -ml-1 size-5 animate-spin text-blue-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
					<circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
					<path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
				<span className="font-medium">{statusText}</span>
			</div>

			<div className="space-y-3">
				{/* Command Section */}
				<div className="bg-background/50 p-3 rounded border border-border relative">
					<button onClick={handleCopyDelta} className="absolute top-2 right-2 p-1.5 rounded bg-muted hover:bg-muted/80 transition-colors" title="Copy arguments">
						<Copy className="w-3 h-3" />
					</button>
					<div className="font-semibold text-orange-600 flex items-center gap-2 mb-2">
						<Terminal className="w-3 h-3" />
						{isGoVet ? 'Go vet analysis' : isBackground ? 'Starting background command' : 'Preparing command'}:
					</div>
					<SyntaxHighlighterComp
						language="bash"
						style={vscDarkPlus}
						customStyle={{
							margin: 0,
							padding: '0.5rem',
							borderRadius: '0.25rem',
							fontSize: '0.75rem',
							lineHeight: '1.2',
						}}
					>
						{command}
					</SyntaxHighlighterComp>
					{explanation && <div className="text-xs text-muted-foreground italic mt-2">{explanation}</div>}
				</div>

				{/* Streaming Output Section */}
				{output && (
					<div className="bg-background/50 p-3 rounded border border-border">
						<div className="font-semibold text-sm mb-2 flex items-center gap-2">
							<div className="animate-pulse h-2 w-2 bg-green-500 rounded-full"></div>
							Streaming Output:
						</div>
						<SyntaxHighlighterComp
							language="bash"
							style={vscDarkPlus}
							customStyle={{
								margin: 0,
								padding: '0.5rem',
								borderRadius: '0.25rem',
								fontSize: '0.75rem',
								lineHeight: '1.2',
								maxHeight: '200px',
								overflow: 'auto',
							}}
						>
							{output}
						</SyntaxHighlighterComp>
					</div>
				)}

				{/* Streaming Error Section */}
				{error && (
					<div className="bg-red-50 border border-red-200 p-3 rounded">
						<div className="font-semibold text-sm mb-2 text-red-700 flex items-center gap-2">
							<AlertCircle className="w-3 h-3" />
							Error:
						</div>
						<SyntaxHighlighterComp
							language="bash"
							style={vscDarkPlus}
							customStyle={{
								margin: 0,
								padding: '0.5rem',
								borderRadius: '0.25rem',
								fontSize: '0.75rem',
								lineHeight: '1.2',
								maxHeight: '200px',
								overflow: 'auto',
							}}
						>
							{error}
						</SyntaxHighlighterComp>
					</div>
				)}
			</div>
		</div>
	)
}

export const RunTerminalCmdResult = ({ event }: RunTerminalCmdResultProps) => {
	const resultData = useMemo(() => {
		let rawResult = event.result
		if (typeof rawResult === 'string') {
			try {
				const parsed = JSON.parse(rawResult)
				if (parsed && typeof parsed === 'object') {
					return parsed
				}
			} catch (e) {
				return { output: rawResult, error: 'Failed to parse result JSON' }
			}
		}
		return rawResult && typeof rawResult === 'object' ? rawResult : {}
	}, [event.result])
	const args = useMemo(() => {
		if (typeof event.arguments === 'string') {
			try {
				const parsed = JSON.parse(event.arguments)
				return parsed
			} catch (e) {
				return {}
			}
		}
		return {}
	}, [event.arguments])
	const { command, output, error, success } = useMemo(
		() => ({
			command: args.command || '',
			output: resultData.output || resultData.result || '',
			error: resultData.error || '',
			success: resultData.success !== undefined ? resultData.success : !resultData.error,
		}),
		[resultData, args]
	)

	const summary = useMemo(() => {
		return success ? 'Command completed successfully.' : `Command failed${error ? '.' : ' with an error.'}`
	}, [success, error, output])

	return (
		<ToolSummaryCard event={event}>
			<div className="space-y-3">
				{/* Summary Section */}
				<div className="text-sm text-muted-foreground mb-3">
					{summary}
				</div>

				{/* Command Section */}
				<div className="bg-background/50 p-3 rounded border border-border">
					<div className="font-semibold text-orange-600 flex items-center gap-2 mb-2">
						<Terminal className="w-3 h-3" />
						Executed Command:
					</div>
					<SyntaxHighlighterComp 
						language="bash" 
						style={vscDarkPlus} 
						customStyle={{ 
							margin: 0, 
							padding: '0.5rem', 
							borderRadius: '0.25rem',
							fontSize: '0.75rem',
							lineHeight: '1.2',
						}}
					>
						{command}
					</SyntaxHighlighterComp>
				</div>

				{/* Output Section */}
				{output && (
					<div className="bg-background/50 p-3 rounded border border-border">
						<div className="font-semibold text-sm mb-2 flex items-center gap-2">
							{success ? <CheckCircle className="w-3 h-3 text-green-600" /> : <AlertCircle className="w-3 h-3 text-red-600" />}
							Output:
						</div>
						<div className="max-h-[200px] overflow-auto">
							<SyntaxHighlighterComp 
								language="bash" 
								style={vscDarkPlus} 
								customStyle={{ 
									margin: 0, 
									padding: '0.5rem', 
									borderRadius: '0.25rem',
									fontSize: '0.75rem',
									lineHeight: '1.2',
								}}
							>
								{output}
							</SyntaxHighlighterComp>
						</div>
					</div>
				)}

				{/* Error Section */}
				{error && (
					<div className="bg-red-50 border border-red-200 p-3 rounded">
						<div className="font-semibold text-sm mb-2 text-red-700 flex items-center gap-2">
							<AlertCircle className="w-3 h-3" />
							Error:
						</div>
						<div className="max-h-[200px] overflow-auto">
							<SyntaxHighlighterComp 
								language="bash" 
								style={vscDarkPlus} 
								customStyle={{ 
									margin: 0, 
									padding: '0.5rem', 
									borderRadius: '0.25rem',
									fontSize: '0.75rem',
									lineHeight: '1.2',
								}}
							>
								{error}
							</SyntaxHighlighterComp>
						</div>
					</div>
				)}
			</div>
		</ToolSummaryCard>
	)
}
