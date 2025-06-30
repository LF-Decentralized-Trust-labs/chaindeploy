import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useCallback, useEffect, useMemo, useState } from 'react'

function ContractSelect({ contracts, value, onChange }: { contracts: string[]; value: string | undefined; onChange: (v: string) => void }) {
	return (
		<div className="mb-4">
			<Label>Contract</Label>
			<Select value={value} onValueChange={onChange}>
				<SelectTrigger className="w-full">
					<SelectValue placeholder="Select contract" />
				</SelectTrigger>
				<SelectContent>
					{contracts.map((c) => (
						<SelectItem key={c} value={c}>
							{c}
						</SelectItem>
					))}
				</SelectContent>
			</Select>
		</div>
	)
}

function TransactionSelect({ transactions, value, onChange }: { transactions: any[]; value: string | undefined; onChange: (v: string) => void }) {
	return (
		<div className="mb-4">
			<Label>Transaction</Label>
			<Select value={value} onValueChange={onChange}>
				<SelectTrigger className="w-full">
					<SelectValue placeholder="Select transaction" />
				</SelectTrigger>
				<SelectContent>
					{transactions.map((t) => (
						<SelectItem key={t.name} value={t.name}>
							{t.name}
						</SelectItem>
					))}
				</SelectContent>
			</Select>
		</div>
	)
}

function ParamInput({ param, value, onChange }: { param: any; value: string; onChange: (v: string) => void }) {
	const type = param.schema?.type || param.schema?.$ref || 'string'
	return (
		<div className="mb-2">
			<Label>
				{param.name} <span className="text-xs text-muted-foreground">({type})</span>
			</Label>
			{type === 'boolean' ? (
				<input type="checkbox" checked={value === 'true'} onChange={(e) => onChange(e.target.checked ? 'true' : 'false')} className="ml-2" />
			) : type === 'number' || type === 'integer' ? (
				<Input type="number" value={value} onChange={(e) => onChange(e.target.value)} placeholder={param.name} />
			) : (
				<Input value={value} onChange={(e) => onChange(e.target.value)} placeholder={param.name} />
			)}
		</div>
	)
}

export function MetadataForm({
	metadata,
	onSubmit,
	loading,
	selectedKey,
	restoredOperation,
	paramValues: controlledParamValues,
	setParamValues: controlledSetParamValues,
}: {
	metadata: any
	onSubmit: (txName: string, args: string[], type: 'invoke' | 'query') => void
	loading: boolean
	selectedKey: any
	restoredOperation?: any
	paramValues?: Record<string, string>
	setParamValues?: (v: Record<string, string>) => void
}) {
	const contracts = useMemo(() => Object.keys(metadata.contracts || {}), [metadata])
	const [selectedContract, setSelectedContract] = useState<string | undefined>(contracts[0])
	const contract = useMemo(() => (selectedContract ? metadata.contracts[selectedContract] : undefined), [selectedContract, metadata])
	const transactions = useMemo(() => contract?.transactions || [], [contract])
	const [selectedTx, setSelectedTx] = useState<string | undefined>(transactions[0]?.name)
	const tx = useMemo(() => transactions.find((t: any) => t.name === selectedTx), [transactions, selectedTx])
	const [internalParamValues, setInternalParamValues] = useState<Record<string, string>>({})
	const paramValues = controlledParamValues ?? internalParamValues
	const setParamValues = controlledSetParamValues ?? setInternalParamValues

	// Restore state from restoredOperation
	useEffect(() => {
		if (restoredOperation && metadata) {
			// Find contract/tx for the function name
			let found = false
			for (const contractName of Object.keys(metadata.contracts || {})) {
				const contract = metadata.contracts[contractName]
				const tx = (contract.transactions || []).find((t: any) => t.name === restoredOperation.fn)
				if (tx) {
					setSelectedContract(contractName)
					setSelectedTx(tx.name)
					let paramObj: Record<string, string> = {}
					if (restoredOperation.paramValues) {
						paramObj = { ...restoredOperation.paramValues }
					} else {
						const argsArr = Array.isArray(restoredOperation.args)
							? restoredOperation.args
							: typeof restoredOperation.args === 'string'
								? restoredOperation.args.split(',').map((s: string) => s.trim())
								: []
						for (const p of tx.parameters || []) {
							paramObj[p.name] = argsArr[p.name] ?? ''
						}
					}
					setParamValues(paramObj)
					found = true
					break
				}
			}
			if (!found) {
				setParamValues({})
			}
		}
	}, [restoredOperation, metadata, setParamValues])

	const handleParamChange = useCallback(
		(name: string, v: string) => {
			setParamValues({ ...paramValues, [name]: v })
		},
		[paramValues, setParamValues]
	)

	const handleAction = useCallback(
		(type: 'invoke' | 'query') => {
			if (!tx) return
			const args = (tx.parameters || []).map((p: any) => paramValues[p.name] ?? '')
			onSubmit(tx.name, args, type)
		},
		[tx, paramValues, onSubmit]
	)

	return (
		<form className="max-w-xl p-4 border rounded bg-background shadow-sm" onSubmit={(e) => e.preventDefault()}>
			<ContractSelect contracts={contracts} value={selectedContract} onChange={setSelectedContract} />
			{transactions.length > 0 && <TransactionSelect transactions={transactions} value={selectedTx} onChange={setSelectedTx} />}
			{tx && tx.parameters && tx.parameters.length > 0 && (
				<div className="mb-4">
					{tx.parameters.map((param: any) => (
						<ParamInput key={param.name} param={param} value={paramValues[param.name] || ''} onChange={(v) => handleParamChange(param.name, v)} />
					))}
				</div>
			)}
			<div className="flex gap-2 mt-2">
				<Button type="button" disabled={loading || !tx || !selectedKey} className="flex-1" onClick={() => handleAction('invoke')}>
					Invoke
				</Button>
				<Button type="button" disabled={loading || !tx || !selectedKey} className="flex-1" variant="secondary" onClick={() => handleAction('query')}>
					Query
				</Button>
			</div>
		</form>
	)
}
