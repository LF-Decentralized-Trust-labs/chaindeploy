import { useQuery } from '@tanstack/react-query'
import { Card } from '../ui/card'
import { Badge } from '../ui/badge'
import { Shield, AlertCircle, Copy, Check } from 'lucide-react'
import { Skeleton } from '../ui/skeleton'
import { Alert, AlertDescription } from '../ui/alert'
import { Button } from '../ui/button'
import { getKeysByIdOptions } from '@/api/client/@tanstack/react-query.gen'
import { useState } from 'react'
import { toast } from 'sonner'

interface ValidatorItemProps {
	keyId: number
	index: number
}

export function ValidatorItem({ keyId, index }: ValidatorItemProps) {
	const [copiedField, setCopiedField] = useState<'address' | 'publicKey' | null>(null)
	
	const {
		data: validatorKey,
		isLoading,
		error,
	} = useQuery({
		...getKeysByIdOptions({
			path: { id: keyId },
		}),
	})

	const copyToClipboard = async (text: string, field: 'address' | 'publicKey') => {
		try {
			await navigator.clipboard.writeText(text)
			setCopiedField(field)
			setTimeout(() => setCopiedField(null), 2000)
		} catch (err) {
			toast.error('Failed to copy to clipboard')
		}
	}

	if (isLoading) {
		return (
			<Card className="p-3">
				<div className="flex items-center gap-2">
					<Skeleton className="h-6 w-6 rounded-full" />
					<div className="flex flex-col gap-1">
						<Skeleton className="h-4 w-24" />
						<Skeleton className="h-4 w-48" />
					</div>
				</div>
			</Card>
		)
	}

	if (error) {
		return (
			<Alert variant="destructive">
				<AlertCircle className="h-4 w-4" />
				<AlertDescription>Failed to load validator {index + 1} data</AlertDescription>
			</Alert>
		)
	}

	return (
		<Card className="p-3">
			<div className="flex items-center gap-2">
				<Badge variant="secondary" className="h-6 w-6 rounded-full p-1">
					<Shield className="h-4 w-4" />
				</Badge>
				<div className="flex flex-col gap-1 flex-1">
					<div className="text-xs text-muted-foreground">Validator {index + 1}</div>
					<div className="flex items-center gap-2">
						<code className="text-xs">{validatorKey?.ethereumAddress}</code>
						<Button
							variant="ghost"
							size="sm"
							className="h-4 w-4 p-0"
							onClick={() => validatorKey?.ethereumAddress && copyToClipboard(validatorKey.ethereumAddress, 'address')}
						>
							{copiedField === 'address' ? (
								<Check className="h-3 w-3 text-green-500" />
							) : (
								<Copy className="h-3 w-3" />
							)}
						</Button>
					</div>
					{validatorKey?.publicKey && (
						<div className="flex items-center gap-2">
							<code className="text-xs text-muted-foreground">Key: {validatorKey.publicKey.slice(0, 20)}...</code>
							<Button
								variant="ghost"
								size="sm"
								className="h-4 w-4 p-0"
								onClick={() => copyToClipboard(validatorKey.publicKey, 'publicKey')}
							>
								{copiedField === 'publicKey' ? (
									<Check className="h-3 w-3 text-green-500" />
								) : (
									<Copy className="h-3 w-3" />
								)}
							</Button>
						</div>
					)}
				</div>
			</div>
		</Card>
	)
}
