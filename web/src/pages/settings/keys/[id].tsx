import { getKeysByIdOptions } from '@/api/client/@tanstack/react-query.gen'
import { CertificateDecoder } from '@/components/keys/certificate-decoder'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useQuery } from '@tanstack/react-query'
import { format } from 'date-fns'
import { Activity, ArrowLeft, Copy, Download, Key, LockIcon } from 'lucide-react'
import { Link, useParams } from 'react-router-dom'
import { toast } from 'sonner'

export default function KeyDetailPage() {
	const { id } = useParams()
	const { data: key, isLoading } = useQuery({
		...getKeysByIdOptions({
			path: { id: Number(id) },
		}),
	})

	const copyToClipboard = (text: string) => {
		navigator.clipboard.writeText(text)
		toast.success('Copied to clipboard')
	}

	const downloadKey = (content: string, filename: string) => {
		const blob = new Blob([content], { type: 'text/plain' })
		const url = window.URL.createObjectURL(blob)
		const a = document.createElement('a')
		a.href = url
		a.download = filename
		document.body.appendChild(a)
		a.click()
		window.URL.revokeObjectURL(url)
		document.body.removeChild(a)
	}

	if (isLoading) {
		return (
			<div className="flex-1 p-8">
				<div className="max-w-4xl mx-auto">
					<div className="mb-8">
						<Skeleton className="h-8 w-32 mb-2" />
						<Skeleton className="h-5 w-64" />
					</div>
					<div className="space-y-8">
						<Card className="p-6">
							<div className="space-y-4">
								<div className="flex items-center gap-4">
									<Skeleton className="h-12 w-12 rounded-lg" />
									<div>
										<Skeleton className="h-6 w-48 mb-2" />
										<Skeleton className="h-4 w-32" />
									</div>
								</div>
								<Skeleton className="h-24 w-full" />
							</div>
						</Card>
					</div>
				</div>
			</div>
		)
	}

	if (!key) {
		return (
			<div className="flex-1 p-8">
				<div className="max-w-4xl mx-auto text-center">
					<Key className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
					<h1 className="text-2xl font-semibold mb-2">Key not found</h1>
					<p className="text-muted-foreground mb-8">The key you're looking for doesn't exist or you don't have access to it.</p>
					<Button asChild>
						<Link to="/settings/keys">
							<ArrowLeft className="mr-2 h-4 w-4" />
							Back to Keys
						</Link>
					</Button>
				</div>
			</div>
		)
	}

	return (
		<div className="flex-1 p-8">
			<div className="max-w-4xl mx-auto">
				<div className="flex items-center gap-2 text-muted-foreground mb-8">
					<Button variant="ghost" size="sm" asChild>
						<Link to="/settings/keys">
							<ArrowLeft className="mr-2 h-4 w-4" />
							Keys
						</Link>
					</Button>
				</div>

				<div className="flex items-center justify-between mb-8">
					<div>
						<div className="flex items-center gap-3 mb-1">
							<h1 className="text-2xl font-semibold">{key.name}</h1>
							<Badge variant="outline">{key.algorithm}</Badge>
							{key.certificate && (
								<Badge variant="outline" className="gap-1">
									<LockIcon className="h-3 w-3" />
									Has Certificate
								</Badge>
							)}
						</div>
						<div className="flex items-center gap-2 text-muted-foreground">
							<span>Provider: {key.provider?.name}</span>
							<span>•</span>
							<span>Created {format(new Date(key.createdAt!), 'PP')}</span>
						</div>
					</div>
					<Badge className="gap-1">
						<Activity className="h-3 w-3" />
						{key.status}
					</Badge>
				</div>

				<Tabs defaultValue="details" className="space-y-4">
					<TabsList>
						<TabsTrigger value="details">Details</TabsTrigger>
						<TabsTrigger value="certificate" disabled={!key.certificate}>
							Certificate{' '}
							{key.certificate && (
								<Badge variant="outline" className="ml-2 bg-background">
									Valid
								</Badge>
							)}
						</TabsTrigger>
					</TabsList>

					<TabsContent value="details" className="space-y-4">
						<Card className="p-6">
							<h2 className="text-lg font-semibold mb-4">Key Information</h2>
							<div className="space-y-6">
								<div className="grid grid-cols-2 gap-4">
									<div>
										<h3 className="text-sm font-medium mb-1">Algorithm</h3>
										<p className="text-sm text-muted-foreground">{key.algorithm}</p>
									</div>
									{key.curve && (
										<div>
											<h3 className="text-sm font-medium mb-1">Curve</h3>
											<p className="text-sm text-muted-foreground">{key.curve}</p>
										</div>
									)}
									{!!key.keySize && (
										<div>
											<h3 className="text-sm font-medium mb-1">Key Size</h3>
											<p className="text-sm text-muted-foreground">{key.keySize} bits</p>
										</div>
									)}
								</div>

								{key.publicKey && (
									<div>
										<div className="flex items-center justify-between mb-2">
											<h3 className="text-sm font-medium">Public Key</h3>
											<div className="flex gap-2">
												<Button variant="ghost" size="sm" className="h-8" onClick={() => copyToClipboard(key.publicKey!)}>
													<Copy className="h-4 w-4 mr-2" />
													Copy
												</Button>
												<Button variant="ghost" size="sm" className="h-8" onClick={() => downloadKey(key.publicKey!, `${key.name}-public.key`)}>
													<Download className="h-4 w-4 mr-2" />
													Download
												</Button>
											</div>
										</div>
										<pre className="text-xs bg-muted p-4 rounded-lg overflow-x-auto">{key.publicKey}</pre>
									</div>
								)}

								<div>
									<h3 className="text-sm font-medium mb-2">Fingerprints</h3>
									<div className="space-y-3">
										{key.sha1Fingerprint && (
											<div>
												<p className="text-sm text-muted-foreground mb-1">SHA1</p>
												<code className="text-xs bg-muted px-3 py-2 rounded-md block font-mono">{key.sha1Fingerprint}</code>
											</div>
										)}
										{key.sha256Fingerprint && (
											<div>
												<p className="text-sm text-muted-foreground mb-1">SHA256</p>
												<code className="text-xs bg-muted px-3 py-2 rounded-md block font-mono">{key.sha256Fingerprint}</code>
											</div>
										)}
									</div>
								</div>
								<div>
									{key.ethereumAddress && (
										<div>
											<div className="flex items-center justify-between mb-2">
												<h3 className="text-sm font-medium">Ethereum Address</h3>
												<Button variant="ghost" size="sm" className="h-8" onClick={() => copyToClipboard(key.ethereumAddress!)}>
													<Copy className="h-4 w-4 mr-2" />
													Copy
												</Button>
											</div>
											<code className="text-xs bg-muted px-3 py-2 rounded-md block font-mono">{key.ethereumAddress}</code>
										</div>
									)}
								</div>
							</div>
						</Card>
					</TabsContent>

					<TabsContent value="certificate" className="space-y-4">
						{key.certificate && (
							<>
								<CertificateDecoder pem={key.certificate} />
							</>
						)}
					</TabsContent>
				</Tabs>
			</div>
		</div>
	)
}
