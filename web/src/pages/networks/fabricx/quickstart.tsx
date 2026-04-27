import {
	getNodeGroupsByIdChildren,
	getOrganizations,
	getServices,
	postNodeGroups,
	postNodeGroupsByIdInit,
	postOrganizations,
	postServicesPostgres,
	postServicesByIdStart,
	postServicesByIdPostgresDatabases,
	HandlerOrganizationResponse,
} from '@/api/client'
import { getNodesDefaultsBesuNodeOptions, postNetworksFabricxMutation, postNetworksFabricxByIdNodesByNodeIdJoinMutation } from '@/api/client/@tanstack/react-query.gen'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { useQuery } from '@tanstack/react-query'
import { AlertCircle, CheckCircle2, Loader2, Rocket } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'

// FabricX Quick Start
// ===================
// One-click provisioning of the canonical 4-party FabricX network. Mirrors the
// configuration exercised by TestFourPartyNetwork in
// pkg/nodes/fabricx/fabricx_integration_test.go.
//
// For each of the 4 parties we:
//   1. Ensure an organization exists (Party{N}MSP → creates if missing)
//   2. Create an orderer group node (stage 1 — certs generated, containers NOT started)
//   3. Create a committer node (stage 1 — certs generated, containers NOT started)
// Then we:
//   4. Create the FabricX network referencing those 4 orderer groups + 4 committers
//      (this produces the Arma-consensus genesis block)
//   5. Join each of the 8 nodes → writes genesis + starts containers
//
// Endorsement in FabricX is handled by token-sdk-x, not by chaindeploy-managed
// nodes, so this quick-start provisions orderer groups + committers only.

type StepStatus = 'pending' | 'running' | 'done' | 'error'

// Format anything thrown by hey-api / fetch / handlers into a human string.
// The generated client surfaces backend error envelopes as plain objects
// ({message, data}, {error, message}, validation arrays, etc.) — String(err)
// on those yields "[object Object]". This walks the common shapes.
function formatError(err: unknown): string {
	if (err == null) return 'Unknown error'
	if (typeof err === 'string') return err
	if (err instanceof Error && err.message) return err.message
	if (typeof err === 'object') {
		const e = err as Record<string, unknown>
		// Backend error envelope: {message, data: {detail, code}}
		if (typeof e.message === 'string') {
			const data = e.data as Record<string, unknown> | undefined
			const detail = data && typeof data.detail === 'string' ? data.detail : undefined
			return detail ? `${e.message} — ${detail}` : e.message
		}
		// Validation envelope: {error, errors: [{field, message}]}
		if (Array.isArray(e.errors) && e.errors.length > 0) {
			const parts = (e.errors as Array<Record<string, unknown>>)
				.map((v) => {
					const field = typeof v.field === 'string' ? v.field : ''
					const msg = typeof v.message === 'string' ? v.message : ''
					return field ? `${field}: ${msg}` : msg
				})
				.filter(Boolean)
			if (parts.length > 0) return parts.join('; ')
		}
		if (typeof e.error === 'string') return e.error
		try {
			return JSON.stringify(err)
		} catch {
			return 'Unknown error'
		}
	}
	return String(err)
}

interface Step {
	id: string
	label: string
	status: StepStatus
	detail?: string
}

interface PartyResult {
	partyId: number
	mspId: string
	organizationId: number
	ordererNodeGroupId: number
	// Ordered: router, batcher, consenter, assembler (same order as InitOrdererGroup)
	ordererChildNodeIds: number[]
	committerNodeGroupId: number
	// Ordered to match ChildRoles(GroupTypeFabricXCommitter):
	// verifier, validator, coordinator, sidecar, query-service.
	committerChildNodeIds: number[]
}

const NUM_PARTIES = 4

// Port layout: each party owns a 20-port slot.
//   Orderer group: basePort + 0..3  (router, batcher, consenter, assembler)
//   Committer:     basePort + 10..14 (sidecar, coordinator, validator, verifier, query-service)
// One shared Postgres container (admin=postgres, one db+user per party) is
// bound to SHARED_POSTGRES_PORT on the host so all 4 committers can reach it.
const BASE_PORT = 17000
const SLOT_SIZE = 20
const SHARED_POSTGRES_PORT = 15432
const SHARED_POSTGRES_ADMIN_USER = 'postgres'
const SHARED_POSTGRES_ADMIN_PASSWORD = 'postgres'
// Service and docker network names derive from the user-chosen network name so
// multiple FabricX quick-start networks can coexist on one host without
// colliding on the UNIQUE services.name constraint. Keep the names short —
// docker object names cap at 63 chars but a 64-byte limit on the container
// name surfaces faster on long prefixes.
function sharedPostgresServiceName(networkName: string) {
	return `${networkName}-pg`
}
function sharedPostgresNetworkName(networkName: string) {
	return `${networkName}-pg-net`
}

function portsForParty(i: number) {
	const base = BASE_PORT + i * SLOT_SIZE
	return {
		router: base,
		batcher: base + 1,
		consenter: base + 2,
		assembler: base + 3,
		sidecar: base + 10,
		coordinator: base + 11,
		validator: base + 12,
		verifier: base + 13,
		queryService: base + 14,
	}
}

function partyDatabaseSpec(partyId: number) {
	return {
		db: `party${partyId}_fabricx`,
		user: `party${partyId}`,
		password: `party${partyId}pw`,
	}
}

export default function FabricXQuickStartPage() {
	const navigate = useNavigate()

	const [networkName, setNetworkName] = useState('fabricx-quickstart')
	// Docker Desktop (macOS/Windows) cannot reach containers via the host's external IP,
	// so rewrite addresses to host.docker.internal / 127.0.0.1. Default to true on
	// platforms where Docker Desktop is typical.
	const [localDev, setLocalDev] = useState(() => {
		const ua = typeof navigator !== 'undefined' ? navigator.userAgent || '' : ''
		return /Mac|Windows/i.test(ua)
	})
	// FabricX requires the channel ID to be "arma" — it is hard-coded here and validated server-side.
	const channelName = 'arma'
	// External IP is read from the system-wide node defaults (same source the
	// Fabric/Besu node creation flows use). It is not user-editable here so the
	// quick-start cannot drift from the rest of the platform.
	const besuDefaultsQuery = useQuery({ ...getNodesDefaultsBesuNodeOptions() })
	const externalIp = besuDefaultsQuery.data?.defaults?.[0]?.externalIp ?? ''
	const externalIpLoading = besuDefaultsQuery.isLoading
	const externalIpError = besuDefaultsQuery.error as Error | undefined
	const [running, setRunning] = useState(false)
	const [done, setDone] = useState(false)
	const [networkId, setNetworkId] = useState<number | null>(null)
	const [steps, setSteps] = useState<Step[]>([])
	const [error, setError] = useState<string | null>(null)

	const orgsQuery = (window as unknown as { __orgCache?: HandlerOrganizationResponse[] }).__orgCache

	const setStep = (id: string, patch: Partial<Step>) => {
		setSteps((prev) => prev.map((s) => (s.id === id ? { ...s, ...patch } : s)))
	}

	const makeInitialSteps = (): Step[] => {
		const arr: Step[] = []
		for (let i = 0; i < NUM_PARTIES; i++) {
			arr.push({ id: `org-${i}`, label: `Ensure Party${i + 1}MSP organization`, status: 'pending' })
		}
		arr.push({ id: 'pg-create', label: 'Create shared Postgres service', status: 'pending' })
		arr.push({ id: 'pg-start', label: 'Start shared Postgres container', status: 'pending' })
		arr.push({ id: 'pg-dbs', label: 'Provision per-party databases + roles', status: 'pending' })
		for (let i = 0; i < NUM_PARTIES; i++) {
			arr.push({ id: `orderer-${i}`, label: `Create + init Party${i + 1} orderer node group`, status: 'pending' })
		}
		for (let i = 0; i < NUM_PARTIES; i++) {
			arr.push({ id: `committer-${i}`, label: `Create Party${i + 1} committer node`, status: 'pending' })
		}
		arr.push({ id: 'network', label: 'Create FabricX network + genesis block', status: 'pending' })
		for (let i = 0; i < NUM_PARTIES; i++) {
			arr.push({ id: `join-orderer-${i}`, label: `Join + start Party${i + 1} orderer (4 children)`, status: 'pending' })
		}
		for (let i = 0; i < NUM_PARTIES; i++) {
			arr.push({ id: `join-committer-${i}`, label: `Join + start Party${i + 1} committer`, status: 'pending' })
		}
		return arr
	}

	const progressPercent = useMemo(() => {
		if (steps.length === 0) return 0
		const doneCount = steps.filter((s) => s.status === 'done').length
		return Math.round((doneCount / steps.length) * 100)
	}, [steps])

	async function findOrCreateOrg(mspId: string): Promise<number> {
		// Prefer cache from this page's first load; otherwise fetch fresh.
		let existing: HandlerOrganizationResponse[] = orgsQuery ?? []
		if (existing.length === 0) {
			const resp = await getOrganizations()
			existing = ((resp.data as unknown as { items?: HandlerOrganizationResponse[] })?.items ?? [])
		}
		const match = existing.find((o) => o.mspId === mspId)
		if (match?.id) return match.id

		const created = await postOrganizations({
			body: { mspId, name: mspId, description: `Auto-created by FabricX quick-start` },
		})
		const createdId = (created.data as unknown as { id?: number } | undefined)?.id
		if (!createdId) throw new Error(`organization creation returned no id for ${mspId}`)
		return createdId
	}

	// Creates a node_group, initializes it (generates crypto + per-role child
	// nodes), and returns the group ID plus the 4 child node IDs in the order
	// the backend's InitOrdererGroup creates them: router, batcher, consenter,
	// assembler.
	async function createOrdererNodeGroup(party: { partyId: number; mspId: string; orgId: number }): Promise<{ groupId: number; childIds: number[] }> {
		const p = portsForParty(party.partyId - 1)
		const groupName = `${party.mspId.toLowerCase()}-orderer`

		// Stage 1a: create empty node_group row
		const createResp = await postNodeGroups({
			body: {
				name: groupName,
				platform: 'FABRICX',
				groupType: 'FABRICX_ORDERER_GROUP',
				organizationId: party.orgId,
				mspId: party.mspId,
				partyId: party.partyId,
				externalIp: externalIp,
				domainNames: [externalIp, 'localhost'],
			},
		})
		const groupId = (createResp.data as unknown as { id?: number } | undefined)?.id
		if (!groupId) {
			const serverErr = (createResp as unknown as { error?: { error?: string } | string })?.error
			const detail = typeof serverErr === 'string' ? serverErr : serverErr?.error || 'unknown error'
			throw new Error(`node_group creation failed for ${groupName}: ${detail}`)
		}

		// Stage 1b: init — generates certs, populates deployment_config, creates 4 children
		await postNodeGroupsByIdInit({
			path: { id: groupId },
			body: {
				routerPort: p.router,
				batcherPort: p.batcher,
				consenterPort: p.consenter,
				assemblerPort: p.assembler,
			},
		})

		// Fetch children so we can join each to the network in stage 2
		const childrenResp = await getNodeGroupsByIdChildren({ path: { id: groupId } })
		const childRows = (childrenResp.data as unknown as Array<{ id?: number; nodeType?: string }> | undefined) ?? []
		const order = ['FABRICX_ORDERER_ROUTER', 'FABRICX_ORDERER_BATCHER', 'FABRICX_ORDERER_CONSENTER', 'FABRICX_ORDERER_ASSEMBLER']
		const childIds: number[] = []
		for (const role of order) {
			const row = childRows.find((r) => r.nodeType === role)
			if (!row?.id) throw new Error(`node_group ${groupId} missing child for role ${role}`)
			childIds.push(row.id)
		}
		return { groupId, childIds }
	}

	// createCommitterNodeGroup creates a FABRICX_COMMITTER node_group plus
	// its 5 per-role child rows (sidecar/coordinator/validator/verifier/
	// query-service) via the same /node-groups + /init flow the orderer
	// path uses. Children share one identity by design; per-container
	// logs come from each child's own /logs endpoint.
	async function createCommitterNodeGroup(party: { partyId: number; mspId: string; orgId: number; ordererAssemblerPort: number }): Promise<{ groupId: number; childIds: number[] }> {
		const p = portsForParty(party.partyId - 1)
		const groupName = `${party.mspId.toLowerCase()}-committer`
		const dbSpec = partyDatabaseSpec(party.partyId)

		// Stage 1a: create empty node_group row
		const createResp = await postNodeGroups({
			body: {
				name: groupName,
				platform: 'FABRICX',
				groupType: 'FABRICX_COMMITTER',
				organizationId: party.orgId,
				mspId: party.mspId,
				externalIp: externalIp,
				domainNames: [externalIp, 'localhost'],
			},
		})
		const groupId = (createResp.data as unknown as { id?: number } | undefined)?.id
		if (!groupId) {
			const serverErr = (createResp as unknown as { error?: { error?: string } | string })?.error
			const detail = typeof serverErr === 'string' ? serverErr : serverErr?.error || 'unknown error'
			throw new Error(`node_group creation failed for ${groupName}: ${detail}`)
		}

		// Stage 1b: init — generates certs, populates deployment_config, creates 5 children
		await postNodeGroupsByIdInit({
			path: { id: groupId },
			body: {
				sidecarPort: p.sidecar,
				coordinatorPort: p.coordinator,
				validatorPort: p.validator,
				verifierPort: p.verifier,
				queryServicePort: p.queryService,
				// All committers share the same postgres container (bound to
				// the host on SHARED_POSTGRES_PORT). Each party gets its own
				// database + role inside it.
				postgresHost: externalIp,
				postgresPort: SHARED_POSTGRES_PORT,
				postgresDb: dbSpec.db,
				postgresUser: dbSpec.user,
				postgresPassword: dbSpec.password,
				channelId: channelName,
				ordererEndpoints: [`${externalIp}:${party.ordererAssemblerPort}`],
			} as never,
		})

		// Fetch children. Order matches ChildRoles(GroupTypeFabricXCommitter):
		// verifier, validator, coordinator, sidecar, query-service.
		const childrenResp = await getNodeGroupsByIdChildren({ path: { id: groupId } })
		const childRows = (childrenResp.data as unknown as Array<{ id?: number; nodeType?: string }> | undefined) ?? []
		const order = [
			'FABRICX_COMMITTER_VERIFIER',
			'FABRICX_COMMITTER_VALIDATOR',
			'FABRICX_COMMITTER_COORDINATOR',
			'FABRICX_COMMITTER_SIDECAR',
			'FABRICX_COMMITTER_QUERY_SERVICE',
		]
		const childIds: number[] = []
		for (const role of order) {
			const row = childRows.find((r) => r.nodeType === role)
			if (!row?.id) throw new Error(`node_group ${groupId} missing child for role ${role}`)
			childIds.push(row.id)
		}
		return { groupId, childIds }
	}

	async function setupSharedPostgres(serviceName: string): Promise<number> {
		// Idempotent: if a previous quickstart (or a parallel one on a different
		// network name that happened to collide) already persisted a service row
		// with the same name, reuse it instead of failing on UNIQUE(services.name).
		const listResp = await getServices()
		const existing = (listResp.data as unknown as Array<{ id?: number; name?: string }> | undefined) ?? []
		const match = existing.find((s) => s.name === serviceName)
		if (match?.id) return match.id

		const createResp = await postServicesPostgres({
			body: {
				name: serviceName,
				db: 'postgres',
				user: SHARED_POSTGRES_ADMIN_USER,
				password: SHARED_POSTGRES_ADMIN_PASSWORD,
				hostPort: SHARED_POSTGRES_PORT,
			},
		})
		const id = (createResp.data as unknown as { id?: number } | undefined)?.id
		if (!id) throw new Error('shared Postgres service creation returned no id')
		return id
	}

	async function startSharedPostgres(serviceId: number, networkName: string): Promise<void> {
		await postServicesByIdStart({
			path: { id: serviceId },
			body: { networkName },
		})
		// Small wait for postgres to finish startup before CREATE ROLE runs.
		await new Promise((resolve) => setTimeout(resolve, 2000))
	}

	async function provisionPartyDatabases(serviceId: number): Promise<void> {
		const databases = Array.from({ length: NUM_PARTIES }, (_, i) => partyDatabaseSpec(i + 1))
		await postServicesByIdPostgresDatabases({
			path: { id: serviceId },
			body: { databases },
		})
	}

	const joinMutation = postNetworksFabricxByIdNodesByNodeIdJoinMutation()
	const createNetworkMutation = postNetworksFabricxMutation()

	async function runQuickStart() {
		setRunning(true)
		setError(null)
		setDone(false)
		setNetworkId(null)
		setSteps(makeInitialSteps())

		const parties: PartyResult[] = []

		try {
			// Phase 1: orgs
			for (let i = 0; i < NUM_PARTIES; i++) {
				const mspId = `Party${i + 1}MSP`
				setStep(`org-${i}`, { status: 'running' })
				const orgId = await findOrCreateOrg(mspId)
				setStep(`org-${i}`, { status: 'done', detail: `org #${orgId}` })
				parties.push({
					partyId: i + 1,
					mspId,
					organizationId: orgId,
					ordererNodeGroupId: 0,
					ordererChildNodeIds: [],
					committerNodeGroupId: 0,
					committerChildNodeIds: [],
				})
			}

			// Phase 1.5: shared Postgres service — one container with per-party dbs.
			const pgServiceName = sharedPostgresServiceName(networkName)
			const pgNetworkName = sharedPostgresNetworkName(networkName)
			setStep('pg-create', { status: 'running' })
			const pgServiceId = await setupSharedPostgres(pgServiceName)
			setStep('pg-create', { status: 'done', detail: `service #${pgServiceId}` })

			setStep('pg-start', { status: 'running' })
			await startSharedPostgres(pgServiceId, pgNetworkName)
			setStep('pg-start', { status: 'done', detail: `host :${SHARED_POSTGRES_PORT}` })

			setStep('pg-dbs', { status: 'running' })
			await provisionPartyDatabases(pgServiceId)
			setStep('pg-dbs', { status: 'done', detail: `${NUM_PARTIES} databases` })

			// Phase 2: orderer node groups (ADR-0001 path)
			for (let i = 0; i < NUM_PARTIES; i++) {
				setStep(`orderer-${i}`, { status: 'running' })
				const { groupId, childIds } = await createOrdererNodeGroup({ partyId: parties[i].partyId, mspId: parties[i].mspId, orgId: parties[i].organizationId })
				parties[i].ordererNodeGroupId = groupId
				parties[i].ordererChildNodeIds = childIds
				setStep(`orderer-${i}`, { status: 'done', detail: `group #${groupId}, children ${childIds.join(',')}` })
			}

			// Phase 3: committer node groups (ADR-0001 path)
			for (let i = 0; i < NUM_PARTIES; i++) {
				setStep(`committer-${i}`, { status: 'running' })
				const ports = portsForParty(parties[i].partyId - 1)
				const { groupId, childIds } = await createCommitterNodeGroup({
					partyId: parties[i].partyId,
					mspId: parties[i].mspId,
					orgId: parties[i].organizationId,
					ordererAssemblerPort: ports.assembler,
				})
				parties[i].committerNodeGroupId = groupId
				parties[i].committerChildNodeIds = childIds
				setStep(`committer-${i}`, { status: 'done', detail: `group #${groupId}, children ${childIds.join(',')}` })
			}

			// Phase 4: create network (produces Arma genesis)
			setStep('network', { status: 'running' })
			const netResp = await createNetworkMutation.mutationFn!({
				body: {
					name: networkName,
					description: 'FabricX 4-party quick-start network',
					config: {
						channelName: channelName,
						localDev: localDev,
						organizations: parties.map((p) => ({
							id: p.organizationId,
							ordererNodeGroupId: p.ordererNodeGroupId,
							committerNodeGroupId: p.committerNodeGroupId,
						})),
					},
				},
			} as unknown as Parameters<typeof createNetworkMutation.mutationFn>[0])

			const createdNetworkId = (netResp as unknown as { id?: number })?.id
			if (!createdNetworkId) throw new Error('network creation returned no id')
			setNetworkId(createdNetworkId)
			setStep('network', { status: 'done', detail: `network #${createdNetworkId}` })

			// Phase 5: join all nodes (stage 2).
			// Orderer side: walk the 4 children in router→batcher→consenter→assembler
			// order. The first call writes the genesis to the group's config dir and
			// starts all 4 containers; subsequent child joins just flip the
			// network_nodes row status to "joined" (the parent group is already up).
			//
			// Continue on transient errors: on macOS Docker Desktop the apiproxy
			// sometimes returns "bind source path does not exist" during a burst
			// of ContainerCreate calls. The backend now writes genesis + flips
			// status even if start fails, and the node can be retried later. So
			// we record the error against the current step but keep joining the
			// rest — otherwise one transient failure leaves 19 parties unjoined.
			const joinErrors: string[] = []
			for (let i = 0; i < NUM_PARTIES; i++) {
				setStep(`join-orderer-${i}`, { status: 'running' })
				let hadErr = false
				for (const childId of parties[i].ordererChildNodeIds) {
					try {
						await joinMutation.mutationFn!({
							path: { id: createdNetworkId, nodeId: childId },
						} as unknown as Parameters<typeof joinMutation.mutationFn>[0])
					} catch (e) {
						hadErr = true
						joinErrors.push(`orderer party${i + 1} child #${childId}: ${formatError(e)}`)
					}
				}
				setStep(`join-orderer-${i}`, { status: hadErr ? 'error' : 'done', detail: hadErr ? 'transient docker error — retry via node page' : undefined })
			}
			for (let i = 0; i < NUM_PARTIES; i++) {
				setStep(`join-committer-${i}`, { status: 'running' })
				let hadErr = false
				for (const childId of parties[i].committerChildNodeIds) {
					try {
						await joinMutation.mutationFn!({
							path: { id: createdNetworkId, nodeId: childId },
						} as unknown as Parameters<typeof joinMutation.mutationFn>[0])
					} catch (e) {
						hadErr = true
						joinErrors.push(`committer party${i + 1} child #${childId}: ${formatError(e)}`)
					}
				}
				setStep(`join-committer-${i}`, { status: hadErr ? 'error' : 'done', detail: hadErr ? 'transient docker error — retry via node page' : undefined })
			}
			if (joinErrors.length > 0) {
				toast.error(`${joinErrors.length} join(s) hit transient Docker errors; retry via Nodes page`)
			}

			setDone(true)
			toast.success('FabricX 4-party network is up.')
		} catch (err) {
			const msg = formatError(err)
			setError(msg)
			// Mark the currently-running step as errored
			setSteps((prev) => prev.map((s) => (s.status === 'running' ? { ...s, status: 'error', detail: msg } : s)))
			toast.error(`Quick-start failed: ${msg}`)
		} finally {
			setRunning(false)
		}
	}

	return (
		<div className="container mx-auto p-6 max-w-4xl space-y-6">
			<div className="flex items-center gap-3">
				<Rocket className="h-8 w-8 text-primary" />
				<div>
					<h1 className="text-2xl font-semibold">FabricX Quick Start</h1>
					<p className="text-sm text-muted-foreground">
						Provision a 4-party FabricX network with Arma consensus in one click. Creates 4 orgs, 4 orderer groups, and 4 committers, then generates genesis and starts everything.
					</p>
				</div>
			</div>

			{!running && !done && (
				<Card>
					<CardHeader>
						<CardTitle>Configuration</CardTitle>
						<CardDescription>Sensible defaults — override only if you know what you&rsquo;re doing.</CardDescription>
					</CardHeader>
					<CardContent className="space-y-4">
						<div className="space-y-2">
							<Label htmlFor="networkName">Network name</Label>
							<Input id="networkName" value={networkName} onChange={(e) => setNetworkName(e.target.value)} />
						</div>
						<div className="flex flex-row items-start gap-3 rounded-md border p-3">
							<Checkbox
								id="localDev"
								checked={localDev}
								onCheckedChange={(v) => setLocalDev(v === true)}
							/>
							<div className="space-y-1 leading-none">
								<Label htmlFor="localDev" className="cursor-pointer">Local development mode</Label>
								<p className="text-xs text-muted-foreground">
									On macOS/Windows (Docker Desktop) containers can&rsquo;t reach the host via the external IP.
									Enabling this rewrites addresses to <code>host.docker.internal</code> /<code>127.0.0.1</code> so
									the network works locally. Leave off on Linux with native Docker.
								</p>
							</div>
						</div>
						<div className="space-y-2">
							<Label>Channel ID</Label>
							<div className="rounded-md border bg-muted/40 px-3 py-2 text-sm font-mono">arma</div>
							<p className="text-xs text-muted-foreground">
								FabricX requires the channel ID to be <code>arma</code>. This cannot be changed.
							</p>
						</div>
						<div className="space-y-2">
							<Label>External host</Label>
							{externalIpLoading ? (
								<div className="rounded-md border bg-muted/40 px-3 py-2 text-sm font-mono text-muted-foreground">Loading…</div>
							) : externalIpError ? (
								<div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm">
									Failed to load platform default external IP: {externalIpError.message}
								</div>
							) : externalIp ? (
								<div className="rounded-md border bg-muted/40 px-3 py-2 text-sm font-mono">{externalIp}</div>
							) : (
								<div className="rounded-md border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-sm">
									No external IP configured. Set it under{' '}
									<Link to="/settings/network" className="underline">Settings → Network</Link> before continuing.
								</div>
							)}
							<p className="text-xs text-muted-foreground">
								Reuses the platform-wide external host configured for Fabric and Besu nodes.
							</p>
						</div>
						<div className="rounded-md bg-muted p-3 text-xs font-mono">
							<div>Ports per party (base {BASE_PORT}, slot {SLOT_SIZE}):</div>
							{Array.from({ length: NUM_PARTIES }, (_, i) => {
								const p = portsForParty(i)
								return (
									<div key={i}>
										Party{i + 1}: router={p.router} batcher={p.batcher} consenter={p.consenter} assembler={p.assembler} | committer={p.sidecar}-{p.queryService}
									</div>
								)
							})}
							<div className="mt-1">Shared Postgres: :{SHARED_POSTGRES_PORT} (one container, {NUM_PARTIES} databases)</div>
						</div>
					</CardContent>
				</Card>
			)}

			{!running && !done && (
				<div className="flex justify-end gap-3">
					<Button variant="outline" onClick={() => navigate('/networks')}>
						Cancel
					</Button>
					<Button onClick={runQuickStart} disabled={!networkName || !externalIp || externalIpLoading}>
						<Rocket className="h-4 w-4 mr-2" />
						Provision 4-party network
					</Button>
				</div>
			)}

			{(running || done || steps.length > 0) && (
				<Card>
					<CardHeader>
						<div className="flex items-center justify-between">
							<div>
								<CardTitle>Provisioning</CardTitle>
								<CardDescription>{progressPercent}% complete</CardDescription>
							</div>
							{done && <CheckCircle2 className="h-6 w-6 text-green-600" />}
						</div>
					</CardHeader>
					<CardContent className="space-y-4">
						<Progress value={progressPercent} />
						<div className="space-y-1 text-sm">
							{steps.map((s) => (
								<div key={s.id} className="flex items-center gap-2">
									{s.status === 'pending' && <span className="h-3 w-3 rounded-full border border-muted-foreground/40" />}
									{s.status === 'running' && <Loader2 className="h-3 w-3 animate-spin text-primary" />}
									{s.status === 'done' && <CheckCircle2 className="h-3 w-3 text-green-600" />}
									{s.status === 'error' && <AlertCircle className="h-3 w-3 text-destructive" />}
									<span className={s.status === 'error' ? 'text-destructive' : ''}>{s.label}</span>
									{s.detail && <span className="text-xs text-muted-foreground">— {s.detail}</span>}
								</div>
							))}
						</div>

						{error && (
							<Alert variant="destructive">
								<AlertCircle className="h-4 w-4" />
								<AlertTitle>Quick-start failed</AlertTitle>
								<AlertDescription>{error}</AlertDescription>
							</Alert>
						)}

						{done && networkId !== null && (
							<Alert>
								<CheckCircle2 className="h-4 w-4" />
								<AlertTitle>Ready</AlertTitle>
								<AlertDescription>
									FabricX network #{networkId} is running with 4 parties.{' '}
									<Link to="/networks" className="underline">
										View networks
									</Link>
								</AlertDescription>
							</Alert>
						)}
					</CardContent>
				</Card>
			)}
		</div>
	)
}
