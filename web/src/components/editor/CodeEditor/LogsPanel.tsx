import { useEffect, useRef, useState } from 'react'
import type { LogsPanelProps } from './types'

export function LogsPanel({ projectId, reloadKey }: LogsPanelProps & { reloadKey?: string | number }) {
	const [logs, setLogs] = useState<string[]>([])
	const logsEndRef = useRef<HTMLDivElement>(null)
	const eventSourceRef = useRef<EventSource | null>(null)

	useEffect(() => {
		console.log('Setting up SSE connection')

		// Close existing connection if any
		if (eventSourceRef.current) {
			console.log('Closing existing connection')
			eventSourceRef.current.close()
		}

		// Clear logs on reload
		setLogs([])

		// Create new connection
		const eventSource = new EventSource(`/api/v1/chaincode-projects/${projectId}/logs/stream`, {
			withCredentials: true,
		})
		eventSourceRef.current = eventSource

		// Connection opened
		eventSource.onopen = (event) => {
			console.log('SSE Connection opened', event)
		}

		// Listen for specific event type
		eventSource.addEventListener('message', (event) => {
			setLogs((prev) => [...prev, event.data])
		})

		// Listen for any custom events
		eventSource.addEventListener('log', (event) => {
			console.log('Received log event:', event)
			setLogs((prev) => [...prev, event.data])
		})

		// Error handling
		eventSource.onerror = (error) => {
			console.error('SSE Error:', error)
			// Try to reconnect after a delay
			setTimeout(() => {
				console.log('Attempting to reconnect...')
				eventSource.close()
				eventSourceRef.current = null
			}, 5000)
		}

		// Cleanup
		return () => {
			console.log('Cleaning up SSE connection')
			if (eventSourceRef.current) {
				eventSourceRef.current.close()
				eventSourceRef.current = null
			}
		}
	}, [projectId, reloadKey])

	// Auto-scroll to bottom when new logs arrive
	useEffect(() => {
		logsEndRef.current?.scrollIntoView({ behavior: 'smooth' })
	}, [logs])

	return (
		<div className="h-full flex flex-col bg-popover border-t border-border">
			<div className="flex items-center justify-between px-4 py-2 bg-muted border-b border-border">
				<h3 className="text-sm font-medium text-muted-foreground">Logs</h3>
			</div>
			<div className="flex flex-col md:flex-row gap-8 h-full max-h-[70vh] overflow-y-auto flex-1 overflow-auto p-2 space-y-2">
				{/* Playground form (left) */}
				<div className="flex-1  border rounded bg-background shadow-sm my-4">
					<div className="p-4 font-mono text-sm">
						{logs.length === 0 ? (
							<div className="text-muted-foreground">Waiting for logs...</div>
						) : (
							logs.map((log, index) => (
								<div key={index} className="text-popover-foreground whitespace-pre-wrap break-all overflow-hidden w-full">
									{log}
								</div>
							))
						)}
						<div ref={logsEndRef} />
					</div>
				</div>
			</div>
		</div>
	)
}
