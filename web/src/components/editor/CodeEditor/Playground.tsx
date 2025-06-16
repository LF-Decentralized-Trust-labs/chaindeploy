import { postChaincodeProjectsByIdInvoke, postChaincodeProjectsByIdQuery } from '@/api/client'
import { FabricKeySelect } from '@/components/FabricKeySelect'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { PlayCircle, RotateCcw, Search, X } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { toast } from 'sonner'

interface PlaygroundProps {
	projectId: number
	networkId: number
}

interface Operation {
	fn: string
	args: string
	selectedKey: { orgId: number; keyId: number } | undefined
	type: 'invoke' | 'query'
	timestamp: number
}
type Response = {
	fn: string
	args: string
	result: any
	timestamp: number
	type: 'invoke' | 'query'
	selectedKey?: { orgId: number; keyId: number }
}
function isValidResponse(r: any): r is { type: 'invoke' | 'query'; response: any; error?: string; timestamp: number } {
	return r && (r.type === 'invoke' || r.type === 'query') && typeof r.timestamp === 'number'
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

export function Playground({ projectId, networkId }: PlaygroundProps) {
	const [fn, setFn] = useState('')
	const [args, setArgs] = useState('')
	const [selectedKey, setSelectedKey] = useState<{ orgId: number; keyId: number } | undefined>(undefined)
	const [responses, setResponses] = useState<Response[]>([])
	const [loadingInvoke, setLoadingInvoke] = useState(false)
	const [loadingQuery, setLoadingQuery] = useState(false)
	const didMount = useRef(false)

	// Load state from localStorage on mount
	useEffect(() => {
		const saved = localStorage.getItem(`playground-state-${projectId}`)
		if (saved) {
			try {
				const parsed = JSON.parse(saved)
				setFn(parsed.fn || '')
				setArgs(parsed.args || '')
				setSelectedKey(parsed.selectedKey)
				setResponses(parsed.responses || [])
			} catch {}
		}
	}, [projectId])

	const saveToLocalStorage = useCallback(
		(
			nextState: Partial<{
				fn: string
				args: string
				selectedKey: { orgId: number; keyId: number } | undefined
				responses: Response[]
			}> = {}
		) => {
			try {
				localStorage.setItem(
					`playground-state-${projectId}`,
					JSON.stringify({
						fn,
						args,
						selectedKey,
						...nextState,
						responses: (nextState.responses || responses).slice(-10),
					})
				)
			} catch (e: any) {
				if (e && (e.name === 'QuotaExceededError' || e.code === 22)) {
					// Try trimming responses and saving again
					const trimmedResponses = (nextState.responses || responses).slice(-10)
					try {
						localStorage.setItem(
							`playground-state-${projectId}`,
							JSON.stringify({
								fn,
								args,
								selectedKey,
								...nextState,
								responses: trimmedResponses,
							})
						)
						toast.error('Local storage was full. Oldest responses were removed to save new ones.')
					} catch {
						toast.error('Local storage is full. Unable to save playground state.')
					}
				} else {
					throw e
				}
			}
		},
		[projectId, fn, args, selectedKey, responses]
	)

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
						args: argsParam
							.split(',')
							.map((a) => a.trim())
							.filter(Boolean),
						keyId: selectedKeyParam.keyId,
						orgId: selectedKeyParam.orgId,
					},
				})
				toast.dismiss(toastId)
				let nextResponses
				if (res.error) {
					nextResponses = [...responses, { type: 'invoke', result: res.error.message, timestamp: Date.now(), fn: fnParam, args: argsParam, selectedKey: selectedKeyParam }]
					setResponses(nextResponses)
				} else {
					nextResponses = [...responses, { type: 'invoke', result: res.data, timestamp: Date.now(), fn: fnParam, args: argsParam, selectedKey: selectedKeyParam }]
					setResponses(nextResponses)
				}
				saveToLocalStorage({ responses: nextResponses })
			} catch (err: any) {
				toast.dismiss(toastId)
				const msg = err?.response?.data?.message || err?.message || 'Unknown error'
				const nextResponses = [...responses, { type: 'invoke', result: msg, timestamp: Date.now(), fn: fnParam, args: argsParam } as Response]
				setResponses(nextResponses)
				saveToLocalStorage({ responses: nextResponses })
			} finally {
				setLoadingInvoke(false)
			}
		},
		[projectId, responses, saveToLocalStorage]
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
						args: argsParam
							.split(',')
							.map((a) => a.trim())
							.filter(Boolean),
						keyId: selectedKeyParam.keyId,
						orgId: selectedKeyParam.orgId,
					},
				})
				toast.dismiss(toastId)
				let nextResponses
				if (res.error) {
					nextResponses = [...responses, { type: 'query', result: res.error.message, timestamp: Date.now(), fn: fnParam, args: argsParam, selectedKey: selectedKeyParam }]
					setResponses(nextResponses)
				} else {
					nextResponses = [...responses, { type: 'query', result: res.data, timestamp: Date.now(), fn: fnParam, args: argsParam, selectedKey: selectedKeyParam }]
					setResponses(nextResponses)
				}
				saveToLocalStorage({ responses: nextResponses })
			} catch (err: any) {
				toast.dismiss(toastId)
				const msg = err?.response?.data?.message || err?.message || 'Unknown error'
				const nextResponses = [...responses, { type: 'query', result: msg, timestamp: Date.now(), fn: fnParam, args: argsParam } as Response]
				setResponses(nextResponses)
				saveToLocalStorage({ responses: nextResponses })
			} finally {
				setLoadingQuery(false)
			}
		},
		[projectId, responses, saveToLocalStorage]
	)

	const restoreOnly = useCallback(
		(resp: { fn: string; args: string; selectedKey?: { orgId: number; keyId: number } }) => {
			setFn(resp.fn)
			setArgs(resp.args)
			if (resp.selectedKey) {
				setSelectedKey(resp.selectedKey)
			}
			saveToLocalStorage({ fn: resp.fn, args: resp.args, selectedKey: resp.selectedKey || selectedKey })
		},
		[setFn, setArgs, setSelectedKey, saveToLocalStorage, selectedKey]
	)

	const sortedResponses = useMemo(() => responses.sort((a, b) => b.timestamp - a.timestamp), [responses])

	// Persist fn and args only after initial mount and if both have value
	useEffect(() => {
		if (didMount.current) {
			if (fn && args) {
				saveToLocalStorage({ fn, args })
			}
		} else {
			didMount.current = true
		}
	}, [fn, args])

	// Add a function to delete an operation
	const handleDeleteResponse = useCallback(
		(timestamp: number) => {
			setResponses((prev) => {
				const next = prev.filter((op) => op.timestamp !== timestamp)
				saveToLocalStorage({ responses: next })
				return next
			})
		},
		[saveToLocalStorage]
	)

	return (
		<div className="grid grid-cols-1 md:grid-cols-6 gap-8 h-[90%] my-8 mx-4">
			{/* Playground form (left) */}
			<div className="md:col-span-2 min-w-[320px] max-w-[480px] border rounded bg-background shadow-sm h-full overflow-y-auto">
				<div className="px-4 py-4 grid gap-4 h-full">
					<h2 className="text-xl font-bold mb-2 grid grid-flow-col auto-cols-max items-center gap-2">
						<PlayCircle className="h-5 w-5" /> Playground
					</h2>
					<div className="grid gap-1">
						<Label>Key & Organization</Label>
						<FabricKeySelect value={selectedKey} onChange={setSelectedKey} />
					</div>
					<div className="grid gap-1">
						<Label htmlFor="fn">Function name</Label>
						<Input id="fn" value={fn} onChange={(e) => setFn(e.target.value)} placeholder="e.g. queryAsset" />
					</div>
					<div className="grid gap-1">
						<Label htmlFor="args">Arguments (comma separated)</Label>
						<Input id="args" value={args} onChange={(e) => setArgs(e.target.value)} placeholder="e.g. asset1, 100" />
					</div>
					<div className="grid grid-flow-col auto-cols-max gap-2 mt-2">
						<Button onClick={() => handleInvoke(fn, args, selectedKey)} disabled={loadingInvoke || !fn || !selectedKey}>
							<PlayCircle className="h-4 w-4 mr-2" />
							Invoke
						</Button>
						<Button onClick={() => handleQuery(fn, args, selectedKey)} disabled={loadingQuery || !fn || !selectedKey} variant="secondary">
							<Search className="h-4 w-4 mr-2" />
							Query
						</Button>
					</div>
				</div>
			</div>
			{/* Responses (right) */}
			<div className="md:col-span-4 border rounded bg-muted/30 p-2 h-full overflow-y-auto">
				{responses.length === 0 ? (
					<div className="text-muted-foreground text-center py-8">No responses yet</div>
				) : (
					<div className="grid gap-2">
						{sortedResponses.map((response) => (
							<div key={response.timestamp} className="p-3 border-b border-muted bg-background rounded relative">
								<div className="grid grid-flow-col auto-cols-max items-center gap-2 mb-1">
									<span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{response.type === 'invoke' ? 'Invoke' : 'Query'}</span>
									<span className="text-xs text-muted-foreground">{new Date(response.timestamp).toLocaleTimeString()}</span>
								</div>
								<div className="text-xs text-muted-foreground mb-1">
									fn: <span className="font-semibold">{response.fn}</span> &nbsp; args: <span className="font-mono">{response.args}</span>
								</div>
								<div className="absolute top-2 right-2 flex gap-1">
									<Button size="icon" variant="ghost" onClick={() => restoreOnly(response)} className="rounded-full" title="Restore">
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
								<div className="text-sm whitespace-pre-wrap break-all mt-6">
									<div className="max-h-64 overflow-auto overflow-x-auto">{renderResponseContent(response.result?.result ?? response.result)}</div>
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
								</div>
							</div>
						))}
					</div>
				)}
			</div>
		</div>
	)
}
