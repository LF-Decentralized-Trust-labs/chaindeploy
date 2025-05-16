import { ModelsKeyResponse } from '@/api/client'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Separator } from '@/components/ui/separator'
import { formatDistanceToNow, format } from 'date-fns'
import { Copy, Key, LockIcon, MoreVertical, Trash } from 'lucide-react'
import { Link } from 'react-router-dom'

interface KeyItemProps {
	keyResponse: ModelsKeyResponse
	onDelete: (key: ModelsKeyResponse) => void
	createdAt?: string
}

export function KeyItem({ keyResponse, onDelete, createdAt }: KeyItemProps) {
	const copyToClipboard = (text: string) => {
		navigator.clipboard.writeText(text)
	}

	return (
		<Link to={`/settings/keys/${keyResponse.id}`}>
			<Card className="p-4 hover:bg-muted/50 transition-colors">
				<div className="flex items-center justify-between">
					<div className="flex items-center gap-4">
						<div className="h-10 w-10 rounded-lg bg-primary/10 flex items-center justify-center">
							<Key className="h-5 w-5 text-primary" />
						</div>
						<div>
							<div className="flex items-center gap-2">
								<h3 className="font-medium">{keyResponse.name}</h3>
								{keyResponse.certificate && (
									<Badge variant="outline" className="gap-1">
										<LockIcon className="h-3 w-3" />
										Has Certificate
									</Badge>
								)}
							</div>
							<p className="text-sm text-muted-foreground">
								{keyResponse.algorithm}
								{keyResponse.algorithm === 'EC' && keyResponse.curve && ` • ${keyResponse.curve}`}
								{keyResponse.algorithm === 'RSA' && keyResponse.keySize && ` • ${keyResponse.keySize} bits`}
								{createdAt && (
									<span>
										{' '}
										<span className="text-xs text-muted-foreground" title={format(new Date(createdAt), 'PPP p')}>
											Created {formatDistanceToNow(new Date(createdAt), { addSuffix: true })}
										</span>
									</span>
								)}
							</p>
						</div>
					</div>
					<div className="flex items-center gap-2">
						<div className="text-xs px-2 py-1 rounded-md bg-primary/10 text-primary pointer-events-none">{keyResponse.provider?.name}</div>
						<DropdownMenu>
							<DropdownMenuTrigger asChild>
								<Button variant="ghost" size="icon" className="pointer-events-auto" onClick={(e) => e.preventDefault()}>
									<MoreVertical className="h-4 w-4" />
								</Button>
							</DropdownMenuTrigger>
							<DropdownMenuContent align="end">
								<DropdownMenuItem
									className="text-destructive"
									onSelect={(e) => {
										e.preventDefault()
										onDelete(keyResponse)
									}}
								>
									<Trash className="h-4 w-4 mr-2" />
									Delete
								</DropdownMenuItem>
							</DropdownMenuContent>
						</DropdownMenu>
					</div>
				</div>

				<Separator className="my-4" />

				<div className="space-y-4">
					{keyResponse.publicKey && (
						<div>
							<div className="flex items-center justify-between mb-2">
								<h4 className="text-sm font-medium">Public Key</h4>
								<Button
									variant="ghost"
									size="icon"
									className="h-6 w-6 pointer-events-auto"
									onClick={(e) => {
										e.preventDefault()
										copyToClipboard(keyResponse.publicKey!)
									}}
								>
									<Copy className="h-3 w-3" />
								</Button>
							</div>
							<pre className="text-xs bg-muted px-2 py-1 rounded block overflow-x-auto">{keyResponse.publicKey}</pre>
						</div>
					)}

					<div className="flex flex-col gap-2 text-xs">
						{keyResponse.sha1Fingerprint && (
							<div className="flex items-center gap-2">
								<span className="font-medium">SHA1:</span>
								<code className="px-2 py-1 rounded-md bg-muted font-mono">{keyResponse.sha1Fingerprint}</code>
							</div>
						)}
						{keyResponse.sha256Fingerprint && (
							<div className="flex items-center gap-2">
								<span className="font-medium">SHA256:</span>
								<code className="px-2 py-1 rounded-md bg-muted font-mono">{keyResponse.sha256Fingerprint}</code>
							</div>
						)}
						{keyResponse.ethereumAddress && (
							<div className="flex items-center gap-2">
								<span className="font-medium">Ethereum Address:</span>
								<code className="px-2 py-1 rounded-md bg-muted font-mono">{keyResponse.ethereumAddress}</code>
							</div>
						)}
					</div>
				</div>
			</Card>
		</Link>
	)
}
