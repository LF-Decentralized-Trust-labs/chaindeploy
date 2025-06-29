import { useState, useEffect, useCallback } from 'react'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { FabricKeySelect } from '@/components/FabricKeySelect'

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
						<SelectItem key={c} value={c}>{c}</SelectItem>
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
						<SelectItem key={t.name} value={t.name}>{t.name}</SelectItem>
					))}
				</SelectContent>
			</Select>
		</div>
	)
}

function ParamInput({ param, value, onChange }: { param: any; value: string; onChange: (v: string) => void }) {
	return (
		<div className="mb-2">
			<Label>{param.name} <span className="text-xs text-muted-foreground">({param.schema?.type || param.schema?.$ref || 'string'})</span></Label>
			<Input value={value} onChange={e => onChange(e.target.value)} placeholder={param.name} />
		</div>
	)
}

export function MetadataForm({ metadata, onSubmit, loading, selectedKey, setSelectedKey }: { metadata: any; onSubmit: (txName: string, args: string[], type: 'invoke' | 'query') => void; loading: boolean; selectedKey: any; setSelectedKey: (k: any) => void }) {
	const contracts = Object.keys(metadata.contracts || {})
	const [selectedContract, setSelectedContract] = useState<string | undefined>(contracts[0])
	const contract = selectedContract ? metadata.contracts[selectedContract] : undefined
	const transactions = contract?.transactions || []
	const [selectedTx, setSelectedTx] = useState<string | undefined>(transactions[0]?.name)
	const tx = transactions.find((t: any) => t.name === selectedTx)
	const [paramValues, setParamValues] = useState<Record<string, string>>({})

	useEffect(() => {
		if (transactions.length && !selectedTx) {
			setSelectedTx(transactions[0].name)
		}
	}, [transactions, selectedTx])

	useEffect(() => {
		if (contracts.length && !selectedContract) {
			setSelectedContract(contracts[0])
		}
	}, [contracts, selectedContract])

	const handleParamChange = useCallback((name: string, v: string) => {
		setParamValues((prev) => ({ ...prev, [name]: v }))
	}, [])

	const handleAction = (type: 'invoke' | 'query') => {
		if (!tx) return
		const args = (tx.parameters || []).map((p: any) => paramValues[p.name] ?? '')
		onSubmit(tx.name, args, type)
	}

	return (
		<form className="max-w-xl p-4 border rounded bg-background shadow-sm" onSubmit={e => e.preventDefault()}>
			<div className="mb-4">
				<Label>Key & Organization</Label>
				<FabricKeySelect value={selectedKey} onChange={setSelectedKey} />
			</div>
			<ContractSelect contracts={contracts} value={selectedContract} onChange={setSelectedContract} />
			{transactions.length > 0 && (
				<TransactionSelect transactions={transactions} value={selectedTx} onChange={setSelectedTx} />
			)}
			{tx && tx.parameters && tx.parameters.length > 0 && (
				<div className="mb-4">
					{tx.parameters.map((param: any) => (
						<ParamInput key={param.name} param={param} value={paramValues[param.name] || ''} onChange={v => handleParamChange(param.name, v)} />
					))}
				</div>
			)}
			<div className="flex gap-2 mt-2">
				<Button type="button" disabled={loading || !tx || !selectedKey} className="flex-1" onClick={() => handleAction('invoke')}>Invoke</Button>
				<Button type="button" disabled={loading || !tx || !selectedKey} className="flex-1" variant="secondary" onClick={() => handleAction('query')}>Query</Button>
			</div>
		</form>
	)
} 