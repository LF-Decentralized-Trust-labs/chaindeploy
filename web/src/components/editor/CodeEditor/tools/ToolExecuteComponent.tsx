import { ToolEvent } from './ToolEventRenderer'
import type { SyntaxHighlighterProps } from 'react-syntax-highlighter'
import SyntaxHighlighter from 'react-syntax-highlighter'
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism'

const SyntaxHighlighterComp = SyntaxHighlighter as unknown as React.ComponentType<SyntaxHighlighterProps>

interface ToolExecuteComponentProps {
	event: ToolEvent
}

export const ToolExecuteComponent = ({ event }: ToolExecuteComponentProps) => {
	return (
		<div className="flex items-center gap-2 text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="animate-spin h-4 w-4 border-2 border-green-500 border-t-transparent rounded-full" />
			<span className="font-medium">Executing {event.name.replace(/_/g, ' ')}...</span>
			{event.arguments && (
				<div className="mt-2 text-xs bg-background/50 p-2 rounded">
					<div className="font-semibold mb-1">Final arguments:</div>
					<SyntaxHighlighterComp
						language="json"
						style={vscDarkPlus}
						PreTag="div"
						className="rounded text-xs"
						showLineNumbers={false}
						wrapLines={false}
						wrapLongLines={false}
						customStyle={{
							margin: 0,
							padding: '0.5rem',
							background: 'rgb(20, 20, 20)',
							fontSize: '10px',
						}}
					>
						{JSON.stringify(event.arguments, null, 2)}
					</SyntaxHighlighterComp>
				</div>
			)}
		</div>
	)
}
