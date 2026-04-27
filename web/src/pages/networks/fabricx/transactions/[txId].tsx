import { getNetworksFabricxByIdTransactionsByTxIdOptions } from '@/api/client/@tanstack/react-query.gen'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, Copy } from 'lucide-react'
import { Link, useParams } from 'react-router-dom'
import { toast } from 'sonner'

export default function FabricXTransactionDetailPage() {
	const { id: idParam, txId } = useParams<{ id: string; txId: string }>()
	const id = Number(idParam)

	const { data: tx, isLoading } = useQuery({
		...getNetworksFabricxByIdTransactionsByTxIdOptions({ path: { id, txId: txId || '' } } as any),
		enabled: Number.isFinite(id) && !!txId,
	})

	const copy = (text: string) => {
		if (!text) return
		navigator.clipboard.writeText(text)
		toast.success('Copied to clipboard')
	}

	const t = tx as any
	const namespaces: any[] = Array.isArray(t?.namespaces) ? t.namespaces : []
	const endorsers: any[] = Array.isArray(t?.endorsers) ? t.endorsers : []

	return (
		<div className="flex-1 p-8">
			<div className="max-w-5xl mx-auto">
				<Button variant="ghost" size="sm" asChild className="mb-4">
					<Link to={`/networks/${id}/fabricx`}>
						<ArrowLeft className="mr-2 h-4 w-4" />
						Back to Network
					</Link>
				</Button>

				<div className="mb-6 flex items-start justify-between">
					<div>
						<h1 className="text-2xl font-semibold">Transaction</h1>
						<p className="text-muted-foreground font-mono text-xs break-all">{txId}</p>
					</div>
					<Badge variant="outline">Network #{id}</Badge>
				</div>

				{isLoading ? (
					<Card>
						<CardContent className="p-6 space-y-3">
							<Skeleton className="h-6 w-48" />
							<Skeleton className="h-4 w-full" />
							<Skeleton className="h-4 w-5/6" />
						</CardContent>
					</Card>
				) : !tx ? (
					<Card>
						<CardContent className="p-6">
							<p className="text-sm text-muted-foreground">Transaction not found.</p>
						</CardContent>
					</Card>
				) : (
					<div className="space-y-6">
						<Card>
							<CardHeader>
								<CardTitle className="text-base">Envelope</CardTitle>
								<CardDescription>Decoded channel header</CardDescription>
							</CardHeader>
							<CardContent>
								<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
									<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
										<div>
											<p className="text-sm font-medium">Tx ID</p>
											<p className="text-xs text-muted-foreground font-mono break-all">{t.txId || txId}</p>
										</div>
										<Button variant="ghost" size="sm" className="h-8 w-8 p-0" onClick={() => copy(t.txId || txId || '')}>
											<Copy className="h-4 w-4" />
										</Button>
									</div>
									<div className="p-3 rounded-lg border bg-muted/30">
										<p className="text-sm font-medium">Type</p>
										<Badge variant="outline" className="mt-1">
											{t.type || '—'}
										</Badge>
									</div>
									<div className="p-3 rounded-lg border bg-muted/30">
										<p className="text-sm font-medium">Channel</p>
										<p className="text-sm text-muted-foreground font-mono">{t.channelId || '—'}</p>
									</div>
									<div className="p-3 rounded-lg border bg-muted/30">
										<p className="text-sm font-medium">Timestamp</p>
										<p className="text-sm text-muted-foreground">{t.timestamp ? new Date(t.timestamp).toLocaleString() : '—'}</p>
									</div>
								</div>
							</CardContent>
						</Card>

						<Card>
							<CardHeader>
								<CardTitle className="text-base">Namespaces</CardTitle>
								<CardDescription>{namespaces.length} namespace{namespaces.length === 1 ? '' : 's'} touched by this tx</CardDescription>
							</CardHeader>
							<CardContent>
								{namespaces.length === 0 ? (
									<p className="text-sm text-muted-foreground">No namespaces in this transaction.</p>
								) : (
									<div className="space-y-6">
										{namespaces.map((ns: any, i: number) => (
											<div key={`${ns.nsId}-${i}`} className="space-y-3">
												<div className="flex items-center gap-2">
													<Badge variant="secondary" className="font-mono">
														{ns.nsId}
													</Badge>
													<span className="text-xs text-muted-foreground">v{ns.nsVersion}</span>
												</div>

												{ns.reads?.length > 0 && (
													<div>
														<p className="text-xs font-medium text-muted-foreground mb-1">Reads</p>
														<Table>
															<TableHeader>
																<TableRow>
																	<TableHead>Key</TableHead>
																	<TableHead className="w-24">Version</TableHead>
																	<TableHead>Label</TableHead>
																</TableRow>
															</TableHeader>
															<TableBody>
																{ns.reads.map((r: any, j: number) => (
																	<TableRow key={j}>
																		<TableCell className="font-mono text-xs break-all">{r.key}</TableCell>
																		<TableCell>{r.version ?? '—'}</TableCell>
																		<TableCell className="text-xs text-muted-foreground">{r.keyLabel || '—'}</TableCell>
																	</TableRow>
																))}
															</TableBody>
														</Table>
													</div>
												)}

												{ns.readWrites?.length > 0 && (
													<div>
														<p className="text-xs font-medium text-muted-foreground mb-1">Read/Writes</p>
														<Table>
															<TableHeader>
																<TableRow>
																	<TableHead>Key</TableHead>
																	<TableHead className="w-24">Version</TableHead>
																	<TableHead>Value</TableHead>
																	<TableHead>Info</TableHead>
																</TableRow>
															</TableHeader>
															<TableBody>
																{ns.readWrites.map((rw: any, j: number) => (
																	<TableRow key={j}>
																		<TableCell className="font-mono text-xs break-all">{rw.key}</TableCell>
																		<TableCell>{rw.version ?? '—'}</TableCell>
																		<TableCell className="font-mono text-xs text-muted-foreground truncate max-w-[240px]">
																			{rw.value}
																		</TableCell>
																		<TableCell className="text-xs text-muted-foreground">{rw.valueInfo || '—'}</TableCell>
																	</TableRow>
																))}
															</TableBody>
														</Table>
													</div>
												)}

												{ns.blindWrites?.length > 0 && (
													<div>
														<p className="text-xs font-medium text-muted-foreground mb-1">Blind writes</p>
														<Table>
															<TableHeader>
																<TableRow>
																	<TableHead>Key</TableHead>
																	<TableHead>Value</TableHead>
																	<TableHead>Info</TableHead>
																</TableRow>
															</TableHeader>
															<TableBody>
																{ns.blindWrites.map((w: any, j: number) => (
																	<TableRow key={j}>
																		<TableCell className="font-mono text-xs break-all">{w.key}</TableCell>
																		<TableCell className="font-mono text-xs text-muted-foreground truncate max-w-[240px]">
																			{w.value}
																		</TableCell>
																		<TableCell className="text-xs text-muted-foreground">{w.valueInfo || '—'}</TableCell>
																	</TableRow>
																))}
															</TableBody>
														</Table>
													</div>
												)}
											</div>
										))}
									</div>
								)}
							</CardContent>
						</Card>

						<Card>
							<CardHeader>
								<CardTitle className="text-base">Endorsers</CardTitle>
								<CardDescription>Identities that endorsed this transaction</CardDescription>
							</CardHeader>
							<CardContent>
								{endorsers.length === 0 ? (
									<p className="text-sm text-muted-foreground">No endorsers.</p>
								) : (
									<Table>
										<TableHeader>
											<TableRow>
												<TableHead>MSP ID</TableHead>
												<TableHead>Subject</TableHead>
											</TableRow>
										</TableHeader>
										<TableBody>
											{endorsers.map((e: any, i: number) => (
												<TableRow key={i}>
													<TableCell className="font-mono text-sm">{e.mspId}</TableCell>
													<TableCell className="text-sm text-muted-foreground">{e.subject || '—'}</TableCell>
												</TableRow>
											))}
										</TableBody>
									</Table>
								)}
							</CardContent>
						</Card>

						<Card>
							<CardHeader>
								<CardTitle className="text-base">Raw transaction JSON</CardTitle>
								<CardDescription>Decoded envelope</CardDescription>
							</CardHeader>
							<CardContent>
								<div className="bg-muted/50 p-4 rounded-md overflow-auto max-h-96">
									<pre className="text-xs font-mono">
										<code>{JSON.stringify(tx, null, 2)}</code>
									</pre>
								</div>
							</CardContent>
						</Card>
					</div>
				)}
			</div>
		</div>
	)
}
