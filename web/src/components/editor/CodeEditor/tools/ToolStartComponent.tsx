import React from 'react'
import { ToolEvent } from './ToolEventRenderer'

interface ToolStartComponentProps {
	event: ToolEvent
}

export const ToolStartComponent = ({ event }: ToolStartComponentProps) => {
	return (
		<div className="flex items-center gap-2 text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="animate-spin h-4 w-4 border-2 border-primary border-t-transparent rounded-full" />
			<span className="font-medium">Starting {event.name.replace(/_/g, ' ')}...</span>
			{event.args && (
				<div className="mt-2 text-xs text-muted-foreground bg-background/50 p-2 rounded">
					<div className="font-semibold mb-1">Arguments:</div>
					<pre className="overflow-x-auto">{JSON.stringify(event.args, null, 2)}</pre>
				</div>
			)}
		</div>
	)
} 