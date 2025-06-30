import { postChaincodeProjectsByIdInvoke, postChaincodeProjectsByIdQuery } from '@/api/client'
import { FabricKeySelect } from '@/components/FabricKeySelect'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { MetadataForm } from '@/pages/smart-contracts/fabric/[id]/MetadataPlaygroundForm'
import { Check, Clipboard, Loader2, PlayCircle, RotateCcw, Search, X } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { toast } from 'sonner'

interface PlaygroundProps {
	projectId: number
	networkId: number
}

type Response = {
	fn: string
	args: string
	result: any
	timestamp: number
	type: 'invoke' | 'query'
	selectedKey?: { orgId: number; keyId: number }
	error?: string
}

function renderResponseContent(content: any) {
	if (!content) {
		return <span className="mt-1 italic">Empty response</span>
	}

	if (typeof content === 'string') {
		try {
			const parsed = JSON.parse(content)
			return <pre className="whitespace-pre-wrap break-all text-sm mt-1">{JSON.stringify(parsed, null, 2)}</pre>
		} catch {
			return <pre className="whitespace-pre-wrap break-all text-sm mt-1">{content}</pre>
		}
	} else if (typeof content === 'object' && content !== null) {
		return <pre className="whitespace-pre-wrap break-all text-sm mt-1">{JSON.stringify(content, null, 2)}</pre>
	} else {
		return <pre className="whitespace-pre-wrap break-all text-sm mt-1">{String(content)}</pre>
	}
}

interface PlaygroundCoreProps {
	fn: string
	setFn: (fn: string) => void
	args: string
	setArgs: (args: string) => void
	selectedKey: { orgId: number; keyId: number } | undefined
	setSelectedKey: (key: { orgId: number; keyId: number } | undefined) => void
	responses: Response[]
	loadingInvoke: boolean
	loadingQuery: boolean
	handleInvoke: (fn: string, args: string, selectedKey: { orgId: number; keyId: number } | undefined) => void
	handleQuery: (fn: string, args: string, selectedKey: { orgId: number; keyId: number } | undefined) => void
	handleDeleteResponse: (timestamp: number) => void
	restoreOnly: (resp: { fn: string; args: string; selectedKey?: { orgId: number; keyId: number } }) => void
	sortedResponses: Response[]
	networkId: number
}

export function PlaygroundCore({
	fn,
	setFn,
	args,
	setArgs,
	selectedKey,
	setSelectedKey,
	responses,
	loadingInvoke,
	loadingQuery,
	handleInvoke,
	handleQuery,
	handleDeleteResponse,
	restoreOnly,
	sortedResponses,
	networkId,
	mode,
	setMode,
	metadata,
}: PlaygroundCoreProps & {
	mode?: 'manual' | 'metadata'
	setMode?: (mode: 'manual' | 'metadata') => void
	metadata?: any
	onMetadataSubmit?: (txName: string, args: any) => void
}) {
	const [copied, setCopied] = useState<{ [timestamp: number]: boolean }>({})
	const [internalMode, setInternalMode] = useState<'manual' | 'metadata'>(metadata ? 'metadata' : 'manual')
	const isControlled = useMemo(() => typeof mode !== 'undefined' && typeof setMode === 'function', [mode, setMode])
	const currentMode = useMemo(() => (isControlled ? mode : internalMode), [isControlled, mode, internalMode])
	const handleSetMode = useCallback(
		(m: 'manual' | 'metadata') => {
			if (isControlled) setMode(m)
			else setInternalMode(m)
		},
		[isControlled, setMode, setInternalMode]
	)

	// MetadataForm state management
	const contracts = useMemo(() => (metadata ? Object.keys(metadata.contracts || {}) : []), [metadata])
	const [selectedContract, setSelectedContract] = useState<string | undefined>(contracts[0])
	const contract = useMemo(() => (selectedContract ? metadata?.contracts?.[selectedContract] : undefined), [selectedContract, metadata])
	const transactions = contract?.transactions || []
	const [selectedTx, setSelectedTx] = useState<string | undefined>(transactions[0]?.name)

	// Keep contract/tx selection in sync with metadata changes
	useEffect(() => {
		if (contracts.length && !selectedContract) setSelectedContract(contracts[0])
	}, [contracts, selectedContract])
	useEffect(() => {
		if (transactions.length && !selectedTx) setSelectedTx(transactions[0]?.name)
	}, [transactions, selectedTx])

	// Restore logic
	const restoreOnlyInternal = useCallback(
		(response: any) => {
			restoreOnly(response)
		},
		[restoreOnly]
	)

	// If metadata is present, show tabs to switch modes
	const showTabs = useMemo(() => !!metadata, [metadata])

	return (
		<div className="w-full max-w-full mx-auto py-8">
			<div className="grid grid-cols-1 md:grid-cols-2 gap-8">
				{/* Playground form (left) */}
				<div className="border rounded bg-background shadow-sm p-6 flex flex-col h-fit">
					<h2 className="text-xl font-bold mb-4 flex items-center gap-2">
						<PlayCircle className="h-5 w-5" /> Playground
					</h2>
					{/* Shared Org/Key selector */}
					<div className="mb-4">
						<Label>Key & Organization</Label>
						<FabricKeySelect value={selectedKey} onChange={setSelectedKey} />
					</div>
					{showTabs && (
						<Tabs value={currentMode} onValueChange={(v) => handleSetMode(v as 'manual' | 'metadata')} className="mb-6">
							<TabsList>
								<TabsTrigger value="metadata">From Metadata</TabsTrigger>
								<TabsTrigger value="manual">Manual</TabsTrigger>
							</TabsList>
						</Tabs>
					)}
					{currentMode === 'manual' && (
						<>
							<div className="space-y-4 mb-4">
								<div>
									<Label htmlFor="fn">Function name</Label>
									<Input id="fn" value={fn} onChange={(e) => setFn(e.target.value)} placeholder="e.g. queryAsset" />
								</div>
								<div>
									<Label htmlFor="args">Arguments (comma separated)</Label>
									<Input id="args" value={args} onChange={(e) => setArgs(e.target.value)} placeholder="e.g. asset1, 100" />
								</div>
							</div>
							<div className="flex gap-2 mt-2">
								<Button className="flex-1" onClick={() => handleInvoke(fn, args, selectedKey)} disabled={loadingInvoke || !fn || !selectedKey}>
									{loadingInvoke ? <Loader2 className="h-4 w-4 mr-2 animate-spin" /> : <PlayCircle className="h-4 w-4 mr-2" />}
									Invoke
								</Button>
								<Button className="flex-1" onClick={() => handleQuery(fn, args, selectedKey)} disabled={loadingQuery || !fn || !selectedKey} variant="secondary">
									{loadingQuery ? <Loader2 className="h-4 w-4 mr-2 animate-spin" /> : <Search className="h-4 w-4 mr-2" />}
									Query
								</Button>
							</div>
						</>
					)}
					{currentMode === 'metadata' && metadata && (
						<MetadataForm
							metadata={metadata}
							onSubmit={(txName, args, type) => {
								if (type === 'invoke') {
									handleInvoke(txName, JSON.stringify(args), selectedKey)
								} else {
									handleQuery(txName, JSON.stringify(args), selectedKey)
								}
							}}
							loading={loadingInvoke || loadingQuery}
							selectedKey={selectedKey}
						/>
					)}
				</div>

				{/* Responses (right) */}
				<div className="flex flex-col h-full">
					<h3 className="text-lg font-semibold mb-4">Responses</h3>
					{responses.length === 0 ? (
						<div className="text-muted-foreground text-center py-12">No responses yet</div>
					) : (
						<div className="flex flex-col gap-4 overflow-y-auto max-h-[70vh] pr-2">
							{sortedResponses.map((response) => (
								<Card key={response.timestamp} className="p-4 bg-muted/60 border border-border rounded-lg w-full relative">
									<div className="flex items-center justify-between mb-1">
										<span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{response.type === 'invoke' ? 'Invoke' : 'Query'}</span>
										<span className="text-xs text-muted-foreground">{new Date(response.timestamp).toLocaleTimeString()}</span>
									</div>
									<div className="text-xs text-muted-foreground mb-1">
										fn: <span className="font-semibold">{response.fn}</span> &nbsp; args: <span className="font-mono">{response.args}</span>
									</div>
									<div className="absolute top-2 right-2 flex gap-1">
										<Button size="icon" variant="ghost" onClick={() => restoreOnlyInternal(response)} className="rounded-full" title="Restore">
											<RotateCcw className="h-4 w-4" />
										</Button>
										<Button size="icon" variant="secondary" onClick={() => handleInvoke(response.fn, response.args, selectedKey)} title="Restore & Invoke" disabled={!selectedKey}>
											<PlayCircle className="h-4 w-4" />
										</Button>
										<Button size="icon" variant="secondary" onClick={() => handleQuery(response.fn, response.args, selectedKey)} title="Restore & Query" disabled={!selectedKey}>
											<Search className="h-4 w-4" />
										</Button>
										<Button size="icon" variant="ghost" className="text-destructive" onClick={() => handleDeleteResponse(response.timestamp)} title="Delete">
											<X className="h-4 w-4" />
										</Button>
									</div>
									<div className="text-sm whitespace-pre-wrap break-all mt-6 relative">
										{response.error ? (
											<div className="relative bg-destructive/10 border border-destructive rounded p-4 text-destructive font-semibold flex items-start">
												<span className="flex-1">Error: {response.error}</span>
												<Button
													size="icon"
													variant="ghost"
													className="ml-2"
													onClick={() => {
														navigator.clipboard.writeText(response.error)
														setCopied((prev) => ({ ...prev, [response.timestamp]: true }))
														setTimeout(() => {
															setCopied((prev) => ({ ...prev, [response.timestamp]: false }))
														}, 1500)
													}}
													title="Copy error"
												>
													{copied[response.timestamp] ? <Check className="h-4 w-4 text-green-500" /> : <Clipboard className="h-4 w-4" />}
												</Button>
											</div>
										) : (
											<>
												<div className="max-h-64 overflow-auto overflow-x-auto pr-10">{renderResponseContent(response.result?.result ?? response.result)}</div>
												<Button
													size="icon"
													variant="ghost"
													className="absolute top-2 right-2 z-10"
													onClick={() => {
														const content = response.result?.result ?? response.result
														const text = typeof content === 'string' ? content : JSON.stringify(content, null, 2)
														navigator.clipboard.writeText(text)
														setCopied((prev) => ({ ...prev, [response.timestamp]: true }))
														setTimeout(() => {
															setCopied((prev) => ({ ...prev, [response.timestamp]: false }))
														}, 1500)
													}}
													title="Copy result"
												>
													{copied[response.timestamp] ? <Check className="h-4 w-4 text-green-500" /> : <Clipboard className="h-4 w-4" />}
												</Button>
												{networkId && response.result && !!response.result.blockNumber && !!response.result.transactionId && (
													<a
														href={`/networks/${networkId}/blocks/${response.result.blockNumber}`}
														className="inline-block mt-2 text-primary underline text-xs hover:text-primary/80"
														target="_blank"
														rel="noopener noreferrer"
													>
														View Block #{response.result.blockNumber}
													</a>
												)}
											</>
										)}
									</div>
								</Card>
							))}
						</div>
					)}
				</div>
			</div>
		</div>
	)
}

export function Playground({ projectId, networkId }: PlaygroundProps) {
	const [fn, setFn] = useState('')
	const [args, setArgs] = useState('')
	const [selectedKey, setSelectedKey] = useState<{ orgId: number; keyId: number } | undefined>(undefined)
	const [responses, setResponses] = useState<Response[]>([])
	const [loadingInvoke, setLoadingInvoke] = useState(false)
	const [loadingQuery, setLoadingQuery] = useState(false)

	const STORAGE_KEY = `playground-state-${projectId}`

	// Load state from localStorage on mount
	useEffect(() => {
		const saved = localStorage.getItem(STORAGE_KEY)
		if (saved) {
			try {
				const parsed = JSON.parse(saved)
				setFn(parsed.fn || '')
				setArgs(parsed.args || '')
				setSelectedKey(parsed.selectedKey)
				setResponses(parsed.responses || [])
			} catch {}
		}
	}, [STORAGE_KEY])

	// Save state to localStorage on change
	useEffect(() => {
		try {
			localStorage.setItem(
				STORAGE_KEY,
				JSON.stringify({
					fn,
					args,
					selectedKey,
					responses: responses.slice(0, 10),
				})
			)
		} catch {}
	}, [fn, args, selectedKey, responses, STORAGE_KEY])

	const sortedResponses = useMemo(() => responses.slice().sort((a, b) => b.timestamp - a.timestamp), [responses])

	const handleInvoke = useCallback(
		async (fnParam: string, argsParam: string, selectedKeyParam: { orgId: number; keyId: number } | undefined) => {
			if (!selectedKeyParam) return
			setLoadingInvoke(true)
			const toastId = toast.loading('Invoking...')
			try {
				const res = await postChaincodeProjectsByIdInvoke({
					path: { id: projectId },
					body: {
						function: fnParam,
						args: argsParam.split(',').map((a) => a.trim()),
						keyId: selectedKeyParam.keyId,
						orgId: selectedKeyParam.orgId,
					},
				})
				toast.dismiss(toastId)
				let nextResponses
				console.log('res', res)
				if (res.error) {
					nextResponses = [{ type: 'invoke', result: res.error.message, timestamp: Date.now(), fn: fnParam, args: argsParam, selectedKey: selectedKeyParam }, ...responses]
					setResponses(nextResponses)
				} else {
					nextResponses = [{ type: 'invoke', result: res.data, timestamp: Date.now(), fn: fnParam, args: argsParam, selectedKey: selectedKeyParam }, ...responses]
					setResponses(nextResponses)
				}
			} catch (err: any) {
				toast.dismiss(toastId)
				const msg = err?.response?.data?.message || err?.message || 'Unknown error'
				const nextResponses = [{ type: 'invoke', result: msg, timestamp: Date.now(), fn: fnParam, args: argsParam } as Response, ...responses]
				setResponses(nextResponses)
			} finally {
				setLoadingInvoke(false)
			}
		},
		[projectId, responses]
	)

	const handleQuery = useCallback(
		async (fnParam: string, argsParam: string, selectedKeyParam: { orgId: number; keyId: number } | undefined) => {
			if (!selectedKeyParam) return
			setLoadingQuery(true)
			const toastId = toast.loading('Querying...')
			try {
				const res = await postChaincodeProjectsByIdQuery({
					path: { id: projectId },
					body: {
						function: fnParam,
						args: argsParam.split(',').map((a) => a.trim()),
						keyId: selectedKeyParam.keyId,
						orgId: selectedKeyParam.orgId,
					},
				})
				toast.dismiss(toastId)
				let nextResponses
				console.log('res', res)
				if (res.error) {
					nextResponses = [{ type: 'query', result: res.error.message, timestamp: Date.now(), fn: fnParam, args: argsParam, selectedKey: selectedKeyParam }, ...responses]
					setResponses(nextResponses)
				} else {
					nextResponses = [{ type: 'query', result: res.data, timestamp: Date.now(), fn: fnParam, args: argsParam, selectedKey: selectedKeyParam }, ...responses]
					setResponses(nextResponses)
				}
			} catch (err: any) {
				toast.dismiss(toastId)
				const msg = err?.response?.data?.message || err?.message || 'Unknown error'
				const nextResponses = [{ type: 'query', result: msg, timestamp: Date.now(), fn: fnParam, args: argsParam } as Response, ...responses]
				setResponses(nextResponses)
			} finally {
				setLoadingQuery(false)
			}
		},
		[projectId, responses]
	)

	const restoreOnly = useCallback((resp: { fn: string; args: string; selectedKey?: { orgId: number; keyId: number } }) => {
		setFn(resp.fn)
		setArgs(resp.args)
		if (resp.selectedKey) {
			setSelectedKey(resp.selectedKey)
		}
	}, [])

	const handleDeleteResponse = useCallback((timestamp: number) => {
		setResponses((prev) => prev.filter((op) => op.timestamp !== timestamp))
	}, [])

	return (
		<PlaygroundCore
			fn={fn}
			setFn={setFn}
			args={args}
			setArgs={setArgs}
			selectedKey={selectedKey}
			setSelectedKey={setSelectedKey}
			responses={responses}
			loadingInvoke={loadingInvoke}
			loadingQuery={loadingQuery}
			handleInvoke={handleInvoke}
			handleQuery={handleQuery}
			handleDeleteResponse={handleDeleteResponse}
			restoreOnly={restoreOnly}
			sortedResponses={sortedResponses}
			networkId={networkId}
		/>
	)
}
