import { getScFabricChaincodesByChaincodeIdMetadataOptions, getScFabricChaincodesByIdOptions } from '@/api/client/@tanstack/react-query.gen'
import { postScFabricChaincodesByChaincodeIdInvoke, postScFabricChaincodesByChaincodeIdQuery } from '@/api/client/sdk.gen'
import { PlaygroundCore } from '@/components/editor/CodeEditor/Playground'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useQuery } from '@tanstack/react-query'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { Skeleton } from '@/components/ui/skeleton'

export default function ChaincodePlaygroundPage() {
	const { id } = useParams<{ id: string }>()
	const navigate = useNavigate()
	const chaincodeId = Number(id)

	const STORAGE_KEY = `fabric-playground-state-${chaincodeId}`

	const { data, isLoading } = useQuery({
		...getScFabricChaincodesByIdOptions({ path: { id: chaincodeId } }),
		enabled: !!chaincodeId,
	})

	const [fn, setFn] = useState('')
	const [args, setArgs] = useState('')
	const [selectedKey, setSelectedKey] = useState<{ orgId: number; keyId: number } | undefined>(undefined)
	const [responses, setResponses] = useState<any[]>([])
	const [loadingInvoke, setLoadingInvoke] = useState(false)
	const [loadingQuery, setLoadingQuery] = useState(false)

	const [mode, setMode] = useState<'metadata' | 'manual'>('manual')
	const metadataQuery = useQuery({
		...getScFabricChaincodesByChaincodeIdMetadataOptions({ path: { chaincodeId } }),
		enabled: !!chaincodeId,
	})
	const metadata = useMemo(() => (metadataQuery.data?.result ? JSON.parse(metadataQuery.data.result as string) : undefined), [metadataQuery.data])
	const networkId = useMemo(() => data?.chaincode?.network_id || 0, [data])
	useEffect(() => {
		if (metadataQuery.data?.result) {
			setMode('metadata')
		}
	}, [metadataQuery.data])
	const [paramValues, setParamValues] = useState<Record<string, string>>({})
	const [selectedContract, setSelectedContract] = useState<string | undefined>(undefined)
	const [selectedTx, setSelectedTx] = useState<string | undefined>(undefined)

	// Load state from localStorage on mount
	useEffect(() => {
		const saved = localStorage.getItem(STORAGE_KEY)
		if (saved) {
			try {
				const parsed = JSON.parse(saved)
				setFn(parsed.fn || '')
				setArgs(parsed.args || '')
				setSelectedKey(parsed.selectedKey)
				setParamValues(parsed.paramValues || {})
				setResponses(parsed.responses || [])
				setSelectedContract(parsed.selectedContract)
				setSelectedTx(parsed.selectedTx)
			} catch {}
		}
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [chaincodeId])

	// Save state to localStorage on change
	useEffect(() => {
		const saveToStorage = () => {
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

		// Only save if we have actual data (not on initial mount)
		if (fn || args || selectedKey || responses.length > 0 || selectedContract || selectedTx) {
			saveToStorage()
		}
	}, [fn, args, selectedKey, responses, paramValues, selectedContract, selectedTx, STORAGE_KEY])

	const sortedResponses = useMemo(() => {
		return responses.slice().sort((a, b) => b.timestamp - a.timestamp)
	}, [responses])

	const handleInvoke = useCallback(
		async (fn: string, args: string, selectedKeyParam?: { orgId: number; keyId: number }, paramValues?: Record<string, string>) => {
			if (!chaincodeId) return
			setLoadingInvoke(true)
			const toastId = toast.loading('Invoking...')
			const parsedArgs = typeof args === 'string' ? (args.trim() ? JSON.parse(args) : []) : typeof args === 'object' ? args : []
			try {
				const res = await postScFabricChaincodesByChaincodeIdInvoke({
					path: { chaincodeId },
					body: { function: fn, args: parsedArgs, key_id: selectedKeyParam?.keyId.toString() },
				})
				setResponses((prev) => [{ type: 'invoke', fn, args: parsedArgs, selectedKey: selectedKeyParam, result: res.data.result, timestamp: Date.now(), paramValues }, ...prev])
			} catch (e: any) {
				toast.error(e?.message || 'Invoke failed')
				setResponses((prev) => [{ type: 'invoke', fn, args: parsedArgs, selectedKey: selectedKeyParam, error: e?.message || e, timestamp: Date.now(), paramValues }, ...prev])
			} finally {
				setLoadingInvoke(false)
				toast.dismiss(toastId)
			}
		},
		[chaincodeId]
	)

	const handleQuery = useCallback(
		async (fn: string, args: string, selectedKeyParam?: { orgId: number; keyId: number }, paramValues?: Record<string, string>) => {
			if (!chaincodeId) return
			setLoadingQuery(true)
			const toastId = toast.loading('Querying...')
			const parsedArgs = typeof args === 'string' ? (args.trim() ? args.split(',').map((s: string) => s.trim()) : []) : typeof args === 'object' ? args : []
			try {
				const res = await postScFabricChaincodesByChaincodeIdQuery({
					path: { chaincodeId },
					body: { function: fn, args: parsedArgs, key_id: selectedKeyParam?.keyId.toString() },
				})
				if (res.error) {
					setResponses((prev) => [{ type: 'query', fn, args: parsedArgs, selectedKey: selectedKeyParam, error: res.error.message, timestamp: Date.now(), paramValues }, ...prev])
					return
				}
				setResponses((prev) => [{ type: 'query', fn, args: parsedArgs, selectedKey: selectedKeyParam, result: res.data, timestamp: Date.now(), paramValues }, ...prev])
			} catch (e: any) {
				toast.error(e?.message || 'Query failed')
				setResponses((prev) => [{ type: 'query', fn, args: parsedArgs, selectedKey: selectedKeyParam, error: e?.message || e, timestamp: Date.now(), paramValues }, ...prev])
			} finally {
				setLoadingQuery(false)
				toast.dismiss(toastId)
			}
		},
		[chaincodeId]
	)

	const handleDeleteResponse = useCallback((timestamp: number) => {
		setResponses((prev) => prev.filter((op) => op.timestamp !== timestamp))
	}, [])

	const restoreOnly = useCallback((resp: { fn: string; args: string; selectedKey?: { orgId: number; keyId: number }; paramValues?: Record<string, string> }) => {
		setFn(resp.fn)
		setArgs(resp.args)
		if (resp.paramValues) setParamValues(resp.paramValues)
		if (resp.selectedKey) setSelectedKey(resp.selectedKey)
	}, [])

	return (
		<div className="p-8 max-w-full mx-auto">
			<Button variant="link" onClick={() => navigate(-1)} className="mb-4">
				Back
			</Button>
			<h1 className="text-2xl font-bold mb-2">Chaincode Playground</h1>
			<div className="mb-4 text-muted-foreground">{isLoading ? <Skeleton className="h-6 w-48 mb-2" /> : data?.chaincode?.name ? `Chaincode: ${data.chaincode.name}` : 'Loading...'}</div>

			{(isLoading || metadataQuery.isLoading) && (
				<div className="space-y-4 mb-4">
					<Skeleton className="h-8 w-1/3" />
					<Skeleton className="h-6 w-1/2" />
					<Skeleton className="h-96 w-full" />
				</div>
			)}

			{metadataQuery.error && mode === 'metadata' && !metadataQuery.isLoading && (
				<Alert variant="destructive" className="mb-4">
					Metadata unavailable: {metadataQuery.error.message}
				</Alert>
			)}

			{!(isLoading || metadataQuery.isLoading) && (
				<PlaygroundCore
					mode={mode}
					metadata={metadata}
					onMetadataSubmit={(txName, args) => {
						handleInvoke(txName, JSON.stringify(args), selectedKey)
					}}
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
					fn={fn}
					setFn={setFn}
					args={args}
					setArgs={setArgs}
					setMode={setMode}
					setParamValues={setParamValues}
					paramValues={paramValues}
					selectedContract={selectedContract}
					setSelectedContract={setSelectedContract}
					selectedTx={selectedTx}
					setSelectedTx={setSelectedTx}
				/>
			)}
		</div>
	)
}
