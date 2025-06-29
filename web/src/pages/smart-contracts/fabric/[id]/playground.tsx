import { getScFabricChaincodesByIdOptions } from '@/api/client/@tanstack/react-query.gen'
import { postScFabricChaincodesByChaincodeIdInvoke, postScFabricChaincodesByChaincodeIdQuery } from '@/api/client/sdk.gen'
import { PlaygroundCore } from '@/components/editor/CodeEditor/Playground'
import { Button } from '@/components/ui/button'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'

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

	const networkId = data?.chaincode?.network_id || 0

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
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [chaincodeId])

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

	const sortedResponses = useMemo(() => {
		return responses.slice().sort((a, b) => b.timestamp - a.timestamp)
	}, [responses])

	const handleInvoke = async (fn: string, args: string, selectedKeyParam?: { orgId: number; keyId: number }) => {
		if (!chaincodeId) return
		setLoadingInvoke(true)
		try {
			const parsedArgs = args.trim() ? JSON.parse(args) : []
			const res = await postScFabricChaincodesByChaincodeIdInvoke({
				path: { chaincodeId },
				body: { function: fn, args: parsedArgs, key_id: selectedKeyParam?.keyId.toString() },
			})
			setResponses((prev) => [{ type: 'invoke', fn, args, selectedKey: selectedKeyParam, result: res.data, timestamp: Date.now() }, ...prev])
		} catch (e: any) {
			toast.error(e?.message || 'Invoke failed')
			setResponses((prev) => [{ type: 'invoke', fn, args, selectedKey: selectedKeyParam, error: e?.message || e, timestamp: Date.now() }, ...prev])
		} finally {
			setLoadingInvoke(false)
		}
	}

	const handleQuery = async (fn: string, args: string, selectedKeyParam?: { orgId: number; keyId: number }) => {
		if (!chaincodeId) return
		setLoadingQuery(true)
		try {
			const parsedArgs = args.trim() ? JSON.parse(args) : []
			const res = await postScFabricChaincodesByChaincodeIdQuery({
				path: { chaincodeId },
				body: { function: fn, args: parsedArgs, key_id: selectedKeyParam?.keyId.toString() },
			})
			if (res.error) {
				setResponses((prev) => [{ type: 'query', fn, args, selectedKey: selectedKeyParam, error: res.error.message, timestamp: Date.now() }, ...prev])
				return
			}
			setResponses((prev) => [{ type: 'query', fn, args, selectedKey: selectedKeyParam, result: res.data, timestamp: Date.now() }, ...prev])
		} catch (e: any) {
			toast.error(e?.message || 'Query failed')
			setResponses((prev) => [{ type: 'query', fn, args, selectedKey: selectedKeyParam, error: e?.message || e, timestamp: Date.now() }, ...prev])
		} finally {
			setLoadingQuery(false)
		}
	}

	const handleDeleteResponse = (timestamp: number) => {
		setResponses((prev) => prev.filter((op) => op.timestamp !== timestamp))
	}

	const restoreOnly = (resp: { fn: string; args: string; selectedKey?: { orgId: number; keyId: number } }) => {
		setFn(resp.fn)
		setArgs(resp.args)
		if (resp.selectedKey) setSelectedKey(resp.selectedKey)
	}

	return (
		<div className="p-8 max-w-full mx-auto">
			<Button variant="link" onClick={() => navigate(-1)} className="mb-4">
				Back
			</Button>
			<h1 className="text-2xl font-bold mb-2">Chaincode Playground</h1>
			<div className="mb-4 text-muted-foreground">{data?.chaincode?.name ? `Chaincode: ${data.chaincode.name}` : 'Loading...'}</div>
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
		</div>
	)
}
