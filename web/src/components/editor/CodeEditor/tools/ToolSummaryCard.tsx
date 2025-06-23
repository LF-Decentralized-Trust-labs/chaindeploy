import React from 'react'
import { ToolEvent } from './ToolEventRenderer'

interface ToolSummaryCardProps {
	event: ToolEvent
	children?: React.ReactNode
}

export const ToolSummaryCard = ({ event, children }: ToolSummaryCardProps) => {
	if (event.name === 'read_file' || event.name === 'write_file') {
		return (
			<div className="bg-muted/70 rounded-lg shadow border border-border flex flex-col">
				<div className="flex items-center gap-2 mb-4">
					<div className="rounded-full p-2 flex items-center justify-center min-w-[44px] min-h-[44px]">
						<span className="font-semibold text-sm leading-tight text-center block">{event.name.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())}</span>
					</div>
				</div>
				<div className="flex-1">{children}</div>
			</div>
		)
	}

	return (
		<div className="bg-muted/70 rounded-lg shadow border border-border flex flex-col">
			<div className="flex items-center gap-2 mb-4">
				<div className="rounded-full flex items-center justify-center min-w-[44px] min-h-[44px]">
					<span className="font-semibold text-sm leading-tight text-center block">{event.name.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())}</span>
				</div>
			</div>
			<div className="flex-1">{children}</div>
		</div>
	)
}
