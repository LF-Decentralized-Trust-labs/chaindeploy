import { Button } from '@/components/ui/button'
import { Code } from 'lucide-react'
import React from 'react'
import { ToolEvent } from './ToolEventRenderer'

interface ToolSummaryCardProps {
	event: ToolEvent
	summary: string
	children?: React.ReactNode
}

export const ToolSummaryCard = ({ event, summary, children }: ToolSummaryCardProps) => {
	if (event.name === 'read_file' || event.name === 'write_file') {
		return (
			<div className="bg-muted/70 rounded-lg p-4 my-2 shadow border border-border flex flex-col min-h-[140px]">
				<div className="flex items-center gap-2 mb-4">
					<div className="rounded-full p-2 flex items-center justify-center min-w-[44px] min-h-[44px]">
						<span className="font-semibold text-sm leading-tight text-center block">{event.name.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())}</span>
					</div>
				</div>
				<div className="text-sm mb-4">{summary}</div>
				<div className="flex-1">{children}</div>
			</div>
		)
	}

	return (
		<div className="bg-muted/70 rounded-lg p-4 my-2 shadow border border-border flex flex-col min-h-[140px]">
			<div className="flex items-center gap-2 mb-4">
				<div className="rounded-full p-2 flex items-center justify-center min-w-[44px] min-h-[44px]">
					<span className="font-semibold text-sm leading-tight text-center block">{event.name.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())}</span>
				</div>
			</div>
			<div className="text-sm mb-4">{summary}</div>
			<div className="flex-1">{children}</div>
			<div className="flex gap-2 mt-auto pt-2">
				<Button variant="secondary" size="sm" className="flex items-center gap-1">
					<Code className="h-3 w-3" />
					Code
				</Button>
			</div>
		</div>
	)
}
