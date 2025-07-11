import { useState } from 'react'
import * as timeago from 'timeago.js'
import { Button } from '@/components/ui/button'
import { Check, X as XIcon } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { getScFabricDefinitionsByDefinitionIdTimelineOptions } from '@/api/client/@tanstack/react-query.gen'

const LIFECYCLE_ACTIONS = ['install', 'approve', 'deploy', 'commit'] as const
type LifecycleAction = (typeof LIFECYCLE_ACTIONS)[number]
const actionLabels: Record<LifecycleAction, string> = {
	install: 'Install',
	approve: 'Approve',
	deploy: 'Deploy',
	commit: 'Commit',
}
const actionColors: Record<LifecycleAction, string> = {
	install: 'bg-blue-100 text-blue-800',
	approve: 'bg-yellow-100 text-yellow-800',
	deploy: 'bg-purple-100 text-purple-800',
	commit: 'bg-green-100 text-green-800',
}

function DefinitionTimeline({ definitionId }: { definitionId: number }) {
	const [expanded, setExpanded] = useState(false)
	const { data, isLoading, error } = useQuery(getScFabricDefinitionsByDefinitionIdTimelineOptions({ path: { definitionId } }))

	const sortedData = data?.slice()?.sort((a, b) => new Date(b.created_at!).getTime() - new Date(a.created_at!).getTime()) || []
	const visibleData = expanded ? sortedData : sortedData.slice(0, 5)

	return (
		<div className="space-y-2">
			{visibleData.map((event) => {
				let result: string | undefined = undefined
				if (event.event_data) {
					try {
						const parsed = typeof event.event_data === 'string' ? JSON.parse(event.event_data) : event.event_data
						result = parsed?.result
					} catch {
						// ignore
					}
				}
				return (
					<div key={event.id} className="flex items-start gap-2 text-sm py-1">
						{/* Result icon */}
						<div className="flex items-center justify-center min-w-[24px]">
							{result === 'success' ? <Check className="text-green-600 w-4 h-4" /> : result === 'failure' ? <XIcon className="text-red-600 w-4 h-4" /> : null}
						</div>
						<div className={`px-2 py-1 rounded ${actionColors[event.event_type as LifecycleAction] || 'bg-gray-100 text-gray-800'} min-w-[70px] text-center`} style={{ flexShrink: 0 }}>
							{actionLabels[event.event_type as LifecycleAction] || event.event_type}
						</div>
						<div className="min-w-[90px] text-muted-foreground text-right pr-2" style={{ flexShrink: 0 }}>
							{timeago.format(event.created_at!)}
						</div>
						<div className="flex-1 break-all text-muted-foreground">
							{event.event_data && (typeof event.event_data === 'object' ? JSON.stringify(event.event_data) : String(event.event_data))}
						</div>
					</div>
				)
			})}
			{isLoading && <div className="text-sm text-muted-foreground">Loading timeline...</div>}
			{error && <div className="text-sm text-red-500">Failed to load timeline</div>}
			{sortedData.length === 0 && !isLoading && <div className="text-sm text-muted-foreground">No events yet</div>}
			{sortedData.length > 5 && (
				<Button variant="ghost" size="sm" className="w-full mt-2" onClick={() => setExpanded((v) => !v)}>
					{expanded ? 'Show Less' : 'Show More'}
				</Button>
			)}
		</div>
	)
}

export { DefinitionTimeline }