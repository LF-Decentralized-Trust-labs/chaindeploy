import React, { useEffect, useRef, useState } from 'react'

interface SSEStreamViewerProps {
	projectId: number
}

type ToolStartEvent = {
	type: 'tool_start'
	toolCallID: string
	name: string
}
type ToolUpdateEvent = {
	type: 'tool_update'
	toolCallID: string
	name: string
	arguments: string
}
type ToolExecuteEvent = {
	type: 'tool_execute'
	toolCallID: string
	name: string
	args: Record<string, unknown>
}
type ToolResultEvent = {
	type: 'tool_result'
	toolCallID: string
	name: string
	result: Record<string, unknown>
}
type LLMEvent = {
	type: 'llm'
	content: string
}
type RawEvent = {
	type: 'raw'
	content: string
}
type UnknownEvent = {
	type: string
	[key: string]: unknown
}

type SSEEvent = ToolStartEvent | ToolUpdateEvent | ToolExecuteEvent | ToolResultEvent | LLMEvent | RawEvent | UnknownEvent

export const SSEStreamViewer: React.FC<SSEStreamViewerProps> = ({ projectId }) => {
	const [events, setEvents] = useState<SSEEvent[]>([])
	const [connected, setConnected] = useState(false)
	const logRef = useRef<HTMLDivElement>(null)

	useEffect(() => {
		const url = `/api/v1/ai/${projectId}/chat`
		const eventSource = new EventSource(url)
		setConnected(true)

		eventSource.onmessage = (event) => {
			// Some events may have multiple lines, handle each line
			const lines = event.data.split('\n').filter(Boolean)
			for (const line of lines) {
				try {
					const parsed = JSON.parse(line)
					setEvents((prev) => [...prev, parsed as SSEEvent])
				} catch {
					// Not JSON, treat as raw string
					setEvents((prev) => [...prev, { type: 'raw', content: line }])
				}
			}
		}

		eventSource.onerror = () => {
			setConnected(false)
			eventSource.close()
		}

		return () => {
			eventSource.close()
			setConnected(false)
		}
	}, [projectId])

	// Auto-scroll to bottom
	useEffect(() => {
		if (logRef.current) {
			logRef.current.scrollTop = logRef.current.scrollHeight
		}
	}, [events])

	// Render each event type differently
	const renderEvent = (event: SSEEvent, idx: number) => {
		switch (event.type) {
			case 'tool_start':
				return (
					<div key={idx} className="text-blue-600">
						�� Tool started: <b>{String(event.name)}</b> (id: {String((event as ToolStartEvent).toolCallID)})
					</div>
				)
			case 'tool_update':
				return (
					<div key={idx} className="text-blue-400">
						↪️ Tool update: <b>{String(event.name)}</b> <span className="text-xs">{String((event as ToolUpdateEvent).arguments)}</span>
					</div>
				)
			case 'tool_execute':
				return (
					<div key={idx} className="text-blue-700">
						🚀 Tool execute: <b>{String(event.name)}</b> <span className="text-xs">{JSON.stringify((event as ToolExecuteEvent).args)}</span>
					</div>
				)
			case 'tool_result':
				return (
					<div key={idx} className="text-green-600">
						✅ Tool result: <b>{String(event.name)}</b> <span className="text-xs">{JSON.stringify((event as ToolResultEvent).result)}</span>
					</div>
				)
			case 'llm':
				return (
					<span key={idx} className="text-black">
						{String((event as LLMEvent).content)}
					</span>
				)
			case 'raw':
				return (
					<div key={idx} className="text-gray-400">
						{String((event as RawEvent).content)}
					</div>
				)
			default:
				return (
					<div key={idx} className="text-gray-500">
						[{String(event.type)}] {JSON.stringify(event)}
					</div>
				)
		}
	}

	return (
		<div className="flex flex-col h-full border rounded bg-background">
			<div className="p-2 border-b font-semibold text-sm bg-muted">SSE Stream Viewer {connected ? <span className="text-green-500">●</span> : <span className="text-red-500">●</span>}</div>
			<div ref={logRef} className="flex-1 overflow-auto p-2 text-sm space-y-1 bg-background" style={{ fontFamily: 'monospace', minHeight: 200 }}>
				{events.map(renderEvent)}
			</div>
		</div>
	)
}
