import React from 'react'
import { ToolEvent } from './ToolEventRenderer'

interface ToolSummaryCardProps {
	event: ToolEvent
	children?: React.ReactNode
}

export const ToolSummaryCard = ({ event, children }: ToolSummaryCardProps) => {
	return (
		<div className="bg-muted/70 rounded-lg shadow border border-border flex flex-col text-xs">
			<div className="flex-1">{children}</div>
		</div>
	)
}
