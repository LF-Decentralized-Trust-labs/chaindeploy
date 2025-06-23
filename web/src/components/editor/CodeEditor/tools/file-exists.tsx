import { CheckCircle, XCircle } from 'lucide-react'
import React from 'react'
import { ToolSummaryCard } from './ToolSummaryCard'
import { ToolEvent } from './ToolEventRenderer'
import { ToolExecuteComponent } from './ToolExecuteComponent'

// --- Argument and Result Interfaces ---

interface FileExistsArgs {
	path: string
	explanation?: string
}

interface FileExistsResultData {
	exists: boolean
	file_path: string
	is_dir: boolean
	modified: string
	permissions: string
	size: number
}

// --- Component Props ---

interface FileExistsComponentProps {
	event: ToolEvent
}

interface FileExistsResultComponentProps {
	event: ToolEvent
	copyToClipboard: (content: string) => void
	copiedCode: string | null
}

// --- Execute Component ---

export const FileExistsExecute = ({ event }: FileExistsComponentProps) => {
	return <ToolExecuteComponent event={event} />
}

// --- Update Component ---

export const FileExistsUpdate = ({
	event,
	accumulatedArgs,
}: {
	event: ToolEvent
	accumulatedArgs: Partial<FileExistsArgs>
	copyToClipboard: (content: string) => void
}) => {
	const { path, explanation } = accumulatedArgs
	return (
		<div className="text-sm text-muted-foreground bg-muted/50 p-3 rounded-lg border border-border">
			<div className="flex items-center gap-2 mb-3">
				<svg className="mr-3 -ml-1 size-5 animate-spin text-blue-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
					<circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
					<path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
				<span className="font-medium">Checking if file exists...</span>
			</div>
			{path && <div className="mt-2 text-xs">Path: {path}</div>}
			{explanation && <div className="mt-1 text-xs italic">"{explanation}"</div>}
		</div>
	)
}

// --- Result Component ---

export const FileExistsResult = ({ event }: FileExistsResultComponentProps) => {
	if (!event.result) {
		return <div className="text-sm text-red-500">Error: No result found for file_exists.</div>
	}

	let resultData: FileExistsResultData
	try {
		// Result might be a string that needs parsing
		resultData = typeof event.result === 'string' ? JSON.parse(event.result) : (event.result as FileExistsResultData)
	} catch (error) {
		return <div className="text-sm text-red-500">Error parsing file_exists result.</div>
	}

	const summary = resultData.exists ? `File "${resultData.file_path}" exists.` : `File does not exist.`

	return (
		<ToolSummaryCard event={event}>
			<div className="space-y-3">
				{/* Summary Section */}
				<div className="text-sm text-muted-foreground mb-3">
					{summary}
				</div>

				{/* Status */}
				<div className="flex items-center gap-2 text-sm">
					{resultData.exists ? <CheckCircle className="h-5 w-5 text-green-500" /> : <XCircle className="h-5 w-5 text-red-500" />}
					<span>{resultData.exists ? 'Exists' : 'Not Found'}</span>
				</div>

				{/* File Details */}
				{resultData.exists && (
					<div className="bg-background/50 p-3 rounded border border-border">
						<div className="text-xs text-muted-foreground space-y-1">
							<div>
								<strong>Path:</strong> {resultData.file_path}
							</div>
							<div>
								<strong>Size:</strong> {resultData.size} bytes
							</div>
							<div>
								<strong>Modified:</strong> {new Date(resultData.modified).toLocaleString()}
							</div>
							<div>
								<strong>Permissions:</strong> {resultData.permissions}
							</div>
							<div>
								<strong>Is Directory:</strong> {resultData.is_dir ? 'Yes' : 'No'}
							</div>
						</div>
					</div>
				)}
			</div>
		</ToolSummaryCard>
	)
} 