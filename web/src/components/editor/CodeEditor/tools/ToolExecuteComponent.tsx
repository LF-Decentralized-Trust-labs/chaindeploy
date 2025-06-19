import { ToolEvent } from './ToolEventRenderer'

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
					<pre className="overflow-x-auto">{event.arguments}</pre>
				</div>
			)}
		</div>
	)
}
