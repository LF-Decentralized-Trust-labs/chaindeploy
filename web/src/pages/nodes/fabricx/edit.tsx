import { getNodesByIdOptions, putNodesByIdMutation } from '@/api/client/@tanstack/react-query.gen'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Save } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'

// Known image tags that ChainLaunch's templates target. The Custom… entry
// swaps in a free-text input so users can pin to a tag we don't ship by
// default.
const ORDERER_IMAGE_VERSIONS = ['v1.0.0-alpha', 'v0.0.24'] as const
const COMMITTER_IMAGE_VERSIONS = ['v1.0.0-alpha', 'v0.1.9'] as const

type Kind = 'ordererGroup' | 'committer' | 'unsupported'

// classifyNode maps a node row to the editor mode. Per-role child rows
// (router/sidecar/...) bounce to the parent group's editor since they
// share identity and the API rejects per-child version updates.
function classifyNode(nodeType?: string): { kind: Kind; childOfGroupID?: number } {
	switch (nodeType) {
		case 'FABRICX_ORDERER_GROUP':
			return { kind: 'ordererGroup' }
		case 'FABRICX_COMMITTER':
			return { kind: 'committer' }
		default:
			return { kind: 'unsupported' }
	}
}

export default function EditFabricXNodePage() {
	const navigate = useNavigate()
	const queryClient = useQueryClient()
	const { id } = useParams<{ id: string }>()
	const nodeID = id ? parseInt(id, 10) : NaN

	const { data: node, isLoading } = useQuery({
		...getNodesByIdOptions({ path: { id: nodeID } }),
		enabled: !Number.isNaN(nodeID),
	})

	const { kind } = classifyNode(node?.nodeType)
	const currentVersion = (node?.fabricXOrdererGroup?.version ?? node?.fabricXCommitter?.version ?? '') as string
	const knownVersions = kind === 'ordererGroup' ? ORDERER_IMAGE_VERSIONS : COMMITTER_IMAGE_VERSIONS

	// Local form state. Initialised once the node loads.
	const [selected, setSelected] = useState<string>('')
	const [custom, setCustom] = useState<string>('')

	useEffect(() => {
		if (!currentVersion) return
		const isKnown = (knownVersions as readonly string[]).includes(currentVersion)
		setSelected(isKnown ? currentVersion : 'custom')
		setCustom(isKnown ? '' : currentVersion)
	}, [currentVersion, knownVersions])

	const updateNode = useMutation({
		...putNodesByIdMutation(),
		onSuccess: () => {
			toast.success('Image tag updated. Click Restart on the node to apply.')
			queryClient.invalidateQueries({ queryKey: ['getNodesById'] })
			navigate(`/nodes/${id}`)
		},
		onError: (error: any) => {
			toast.error(`Failed to update node: ${error?.error?.message || error?.message || 'Unknown error'}`)
		},
	})

	if (isLoading) return <div className="p-8">Loading…</div>
	if (!node) return <div className="p-8">Node not found</div>

	if (kind === 'unsupported') {
		// Leaf child rows share identity with the parent group; bounce
		// the user to the parent's editor with a one-line explainer.
		return (
			<div className="flex-1 p-8">
				<div className="max-w-3xl mx-auto">
					<Button variant="ghost" size="sm" asChild className="mb-4">
						<Link to={`/nodes/${id}`}>
							<ArrowLeft className="mr-2 h-4 w-4" />
							Back to node
						</Link>
					</Button>
					<Card className="p-6 space-y-4">
						<h1 className="text-2xl font-semibold">Edit not available on this row</h1>
						<p className="text-sm text-muted-foreground">
							This is a per-role Fabric-X child node. The image tag is shared across all
							siblings of the parent node group, so the editor lives there. Open the
							parent group's detail page to edit its version.
						</p>
					</Card>
				</div>
			</div>
		)
	}

	const targetVersion = selected === 'custom' ? custom.trim() : selected
	const dirty = targetVersion !== '' && targetVersion !== currentVersion
	const canSave = dirty && !updateNode.isPending

	const onSave = () => {
		if (!canSave) return
		const body =
			kind === 'ordererGroup'
				? { fabricXOrdererGroup: { version: targetVersion } }
				: { fabricXCommitter: { version: targetVersion } }
		updateNode.mutate({ path: { id: nodeID }, body })
	}

	const heading = kind === 'ordererGroup' ? 'Edit Fabric-X Orderer Group' : 'Edit Fabric-X Committer'
	const description =
		kind === 'ordererGroup'
			? 'Change the docker image tag of fabric-x-orderer used by this orderer group.'
			: 'Change the docker image tag of fabric-x-committer used by this committer.'

	return (
		<div className="flex-1 p-8">
			<div className="max-w-3xl mx-auto">
				<Button variant="ghost" size="sm" asChild className="mb-4">
					<Link to={`/nodes/${id}`}>
						<ArrowLeft className="mr-2 h-4 w-4" />
						Back to node
					</Link>
				</Button>

				<Card className="p-6 space-y-6">
					<div className="space-y-1">
						<h1 className="text-2xl font-semibold">{heading}</h1>
						<p className="text-sm text-muted-foreground">{description}</p>
					</div>

					<div className="space-y-2">
						<p className="text-xs font-medium text-muted-foreground">Current version</p>
						<p className="font-mono text-sm">{currentVersion || 'N/A'}</p>
					</div>

					<div className="space-y-2">
						<p className="text-xs font-medium text-muted-foreground">New version</p>
						<Select value={selected} onValueChange={setSelected}>
							<SelectTrigger>
								<SelectValue placeholder="Select a version" />
							</SelectTrigger>
							<SelectContent>
								{knownVersions.map((v: string) => (
									<SelectItem key={v} value={v}>
										{v}
									</SelectItem>
								))}
								<SelectItem value="custom">Custom…</SelectItem>
							</SelectContent>
						</Select>
					</div>

					{selected === 'custom' && (
						<div className="space-y-2">
							<p className="text-xs font-medium text-muted-foreground">Custom tag</p>
							<Input
								value={custom}
								onChange={(e) => setCustom(e.target.value)}
								placeholder="e.g. v1.0.0-rc1"
								className="font-mono"
							/>
						</div>
					)}

					<div className="rounded-md border bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
						Saving updates the image tag in the database. The change takes effect the
						next time you Restart the node — running containers are not stopped here so
						you control the moment of downtime.
					</div>

					<div className="flex items-center justify-end gap-2">
						<Button variant="outline" asChild>
							<Link to={`/nodes/${id}`}>Cancel</Link>
						</Button>
						<Button onClick={onSave} disabled={!canSave}>
							<Save className="mr-1.5 h-4 w-4" />
							{updateNode.isPending ? 'Saving…' : 'Save'}
						</Button>
					</div>
				</Card>
			</div>
		</div>
	)
}
