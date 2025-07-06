import { postChaincodeProjectsByIdInvoke, postChaincodeProjectsByIdQuery } from '@/api/client'
import { getChaincodeProjectsByIdMetadataOptions } from '@/api/client/@tanstack/react-query.gen'
import { FabricKeySelect } from '@/components/FabricKeySelect'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { MetadataForm } from '@/pages/smart-contracts/fabric/[id]/MetadataPlaygroundForm'
import { useQuery } from '@tanstack/react-query'
import { Check, Clipboard, Loader2, PlayCircle, RotateCcw, Search, X } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
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
	paramValues?: Record<string, string>
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
	handleInvoke: (fn: string, args: string, selectedKey: { orgId: number; keyId: number } | undefined, paramValues?: Record<string, string>) => void
	handleQuery: (fn: string, args: string, selectedKey: { orgId: number; keyId: number } | undefined, paramValues?: Record<string, string>) => void
	handleDeleteResponse: (timestamp: number) => void
	restoreOnly: (resp: { fn: string; args: string; selectedKey?: { orgId: number; keyId: number } }) => void
	sortedResponses: Response[]
	networkId: number
	paramValues?: Record<string, string>
	setParamValues?: (paramValues: Record<string, string>) => void
	selectedContract?: string
	setSelectedContract?: (c: string) => void
	selectedTx?: string
	setSelectedTx?: (t: string) => void
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
	paramValues,
	setParamValues,
	mode,
	setMode,
	metadata,
	onReloadMetadata,
	selectedContract,
	setSelectedContract,
	selectedTx,
	setSelectedTx,
}: PlaygroundCoreProps & {
	mode?: 'manual' | 'metadata'
	setMode?: (mode: 'manual' | 'metadata') => void
	metadata?: any
	onMetadataSubmit?: (txName: string, args: any) => void
	onReloadMetadata?: () => void
}) {
	const [copied, setCopied] = useState<{ [timestamp: number]: boolean }>({})
	const currentMode = useMemo(() => (mode ? mode : 'manual'), [mode])
	const handleSetMode = useCallback(
		(m: 'manual' | 'metadata') => {
			if (mode) setMode(m)
		},
		[mode, setMode]
	)

	// MetadataForm state management
	const contracts = useMemo(() => (metadata ? Object.keys(metadata.contracts || {}) : []), [metadata])
	const contract = useMemo(() => (selectedContract ? metadata?.contracts?.[selectedContract] : undefined), [selectedContract, metadata])
	const transactions = contract?.transactions || []

	// Keep contract/tx selection in sync with metadata changes
	useEffect(() => {
		if (contracts.length && !selectedContract && setSelectedContract) setSelectedContract(contracts[0])
	}, [contracts, selectedContract, setSelectedContract])
	useEffect(() => {
		if (transactions.length && !selectedTx && setSelectedTx) setSelectedTx(transactions[0]?.name)
	}, [transactions, selectedTx, setSelectedTx])

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
			<div className="grid grid-cols-1 md:grid-cols-2 gap-8 mb-4">
				{/* Playground form (left) */}
				<div className="border rounded bg-background shadow-sm p-6 flex flex-col overflow-auto h-[70vh] ">
					<h2 className="text-xl font-bold mb-4 flex items-center gap-2">
						<PlayCircle className="h-5 w-5" /> Playground
					</h2>
					{/* Shared Org/Key selector */}
					<div className="mb-4">
						<Label>Key & Organization</Label>
						<FabricKeySelect value={selectedKey} onChange={setSelectedKey} />
					</div>
					{showTabs && (
						<div className="flex items-center mb-6 gap-2">
							<Tabs value={currentMode} onValueChange={(v) => handleSetMode(v as 'manual' | 'metadata')}>
								<TabsList>
									<TabsTrigger value="metadata">From Metadata</TabsTrigger>
									<TabsTrigger value="manual">Manual</TabsTrigger>
								</TabsList>
							</Tabs>
							{onReloadMetadata && currentMode === 'metadata' && (
								<Button size="icon" variant="ghost" className="ml-1" onClick={onReloadMetadata} title="Reload metadata" aria-label="Reload metadata">
									<RotateCcw className="h-4 w-4" />
								</Button>
							)}
						</div>
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
								<Button className="flex-1" onClick={() => handleInvoke(fn, args, selectedKey, paramValues)} disabled={loadingInvoke || !fn || !selectedKey}>
									{loadingInvoke ? <Loader2 className="h-4 w-4 mr-2 animate-spin" /> : <PlayCircle className="h-4 w-4 mr-2" />}
									Invoke
								</Button>
								<Button className="flex-1" onClick={() => handleQuery(fn, args, selectedKey, paramValues)} disabled={loadingQuery || !fn || !selectedKey} variant="secondary">
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
									handleInvoke(txName, args as any, selectedKey, { ...paramValues })
								} else {
									handleQuery(txName, args as any, selectedKey, { ...paramValues })
								}
							}}
							loading={loadingInvoke || loadingQuery}
							selectedKey={selectedKey}
							paramValues={paramValues}
							setParamValues={setParamValues}
							selectedContract={selectedContract}
							setSelectedContract={setSelectedContract}
							selectedTx={selectedTx}
							setSelectedTx={setSelectedTx}
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
	const [mode, setMode] = useState<'manual' | 'metadata'>('manual')
	const STORAGE_KEY = useMemo(() => `playground-state-${projectId}`, [projectId])
	const metadataQuery = useQuery({
		...getChaincodeProjectsByIdMetadataOptions({ path: { id: projectId } }),
	})
	const metadata = useMemo(() => (metadataQuery.data?.result ? JSON.parse(metadataQuery.data.result as string) : undefined), [metadataQuery.data])
	// Move this up before the useEffect hooks
	const [paramValues, setParamValues] = useState<Record<string, string>>({})
	// Add state for selectedContract and selectedTx above useEffect hooks
	const [selectedContract, setSelectedContract] = useState<string | undefined>(undefined)
	const [selectedTx, setSelectedTx] = useState<string | undefined>(undefined)

	// Switch to metadata mode if metadata is available
	useEffect(() => {
		if (metadata && mode === 'manual') {
			setMode('metadata')
		}
	}, [metadata, mode, setMode])
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
				setParamValues(parsed.paramValues || {})
				setSelectedContract(parsed.selectedContract)
				setSelectedTx(parsed.selectedTx)
			} catch {}
		}
	}, [STORAGE_KEY])

	// Save state to localStorage on change
	useEffect(() => {
		if (fn.trim() || Object.keys(paramValues).length > 0 || selectedContract || selectedTx) {
			try {
				localStorage.setItem(
					STORAGE_KEY,
					JSON.stringify({
						fn,
						args,
						selectedKey,
						responses: responses.slice(0, 10),
						paramValues,
						selectedContract,
						selectedTx,
					})
				)
			} catch {}
		}
	}, [fn, args, selectedKey, responses, paramValues, selectedContract, selectedTx, STORAGE_KEY])

	const sortedResponses = useMemo(() => responses.slice().sort((a, b) => b.timestamp - a.timestamp), [responses])

	const handleInvoke = useCallback(
		async (fnParam: string, argsParam: string, selectedKeyParam: any, paramValuesObj?: Record<string, string>) => {
			if (!selectedKeyParam) return
			setLoadingInvoke(true)
			const toastId = toast.loading('Invoking...')
			const parsedArgs = typeof argsParam === 'string' ? (argsParam.trim() ? argsParam.split(',').map((s: string) => s.trim()) : []) : typeof argsParam === 'object' ? argsParam : []
			try {
				const res = await postChaincodeProjectsByIdInvoke({
					path: { id: projectId },
					body: {
						function: fnParam,
						args: parsedArgs,
						keyId: selectedKeyParam.keyId,
						orgId: selectedKeyParam.orgId,
					},
				})
				toast.dismiss(toastId)
				let nextResponses
				console.log('res', res)
				if (res.error) {
					nextResponses = [
						{ type: 'invoke', result: res.error.message, timestamp: Date.now(), fn: fnParam, args: argsParam, selectedKey: selectedKeyParam, paramValues: paramValuesObj },
						...responses,
					]
					setResponses(nextResponses)
				} else {
					nextResponses = [
						{ type: 'invoke', result: res.data, timestamp: Date.now(), fn: fnParam, args: argsParam, selectedKey: selectedKeyParam, paramValues: paramValuesObj },
						...responses,
					]
					setResponses(nextResponses)
				}
			} catch (err: any) {
				toast.dismiss(toastId)
				const msg = err?.response?.data?.message || err?.message || 'Unknown error'
				const nextResponses = [{ type: 'invoke', result: msg, timestamp: Date.now(), fn: fnParam, args: argsParam, paramValues: paramValuesObj } as Response, ...responses]
				setResponses(nextResponses)
			} finally {
				setLoadingInvoke(false)
			}
		},
		[projectId, responses]
	)

	const handleQuery = useCallback(
		async (fnParam: string, argsParam: string, selectedKeyParam: any, paramValuesObj?: Record<string, string>) => {
			if (!selectedKeyParam) return
			setLoadingQuery(true)
			const toastId = toast.loading('Querying...')
			try {
				const parsedArgs = typeof argsParam === 'string' ? (argsParam.trim() ? argsParam.split(',').map((s: string) => s.trim()) : []) : typeof argsParam === 'object' ? argsParam : []
				const res = await postChaincodeProjectsByIdQuery({
					path: { id: projectId },
					body: {
						function: fnParam,
						args: parsedArgs,
						keyId: selectedKeyParam.keyId,
						orgId: selectedKeyParam.orgId,
					},
				})
				toast.dismiss(toastId)
				let nextResponses
				if (res.error) {
					nextResponses = [
						{ type: 'query', result: res.error.message, timestamp: Date.now(), fn: fnParam, args: argsParam, selectedKey: selectedKeyParam, paramValues: paramValuesObj },
						...responses,
					]
					setResponses(nextResponses)
				} else {
					nextResponses = [{ type: 'query', result: res.data, timestamp: Date.now(), fn: fnParam, args: argsParam, selectedKey: selectedKeyParam, paramValues: paramValuesObj }, ...responses]
					setResponses(nextResponses)
				}
			} catch (err: any) {
				toast.dismiss(toastId)
				const msg = err?.response?.data?.message || err?.message || 'Unknown error'
				const nextResponses = [{ type: 'query', result: msg, timestamp: Date.now(), fn: fnParam, args: argsParam, paramValues: paramValuesObj } as Response, ...responses]
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
			paramValues={paramValues}
			setParamValues={setParamValues}
			mode={mode}
			setMode={setMode}
			metadata={metadata}
			onReloadMetadata={() => {
				toast.promise(metadataQuery.refetch(), {
					loading: 'Reloading metadata...',
					success: 'Metadata reloaded successfully',
					error: (err) => `Failed to reload metadata: ${err.message || 'Unknown error'}`,
				})
			}}
			selectedContract={selectedContract}
			setSelectedContract={setSelectedContract}
			selectedTx={selectedTx}
			setSelectedTx={setSelectedTx}
		/>
	)
}
