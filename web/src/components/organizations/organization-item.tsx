import { GetOrganizationsByIdResponse } from '@/api/client'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { TimeAgo } from '@/components/ui/time-ago'
import { Building2, Key, MoreVertical, Trash } from 'lucide-react'
import { Link } from 'react-router-dom'

interface OrganizationItemProps {
	organization: GetOrganizationsByIdResponse
	onDelete: (org: GetOrganizationsByIdResponse) => void
}

export function OrganizationItem({ organization, onDelete }: OrganizationItemProps) {
	// Handle dropdown click without triggering the card click
	const handleDropdownClick = (e: React.MouseEvent) => {
		e.preventDefault()
		e.stopPropagation()
	}

	return (
		<Link
			to={`/organizations/${organization.id}`}
			className="block p-4 rounded-lg border bg-card text-card-foreground shadow-sm hover:shadow cursor-pointer transition-all duration-200 hover:border-primary/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
		>
			<div className="flex items-center justify-between">
				<div className="flex items-center gap-4">
					<div className="h-10 w-10 rounded-lg bg-primary/10 flex items-center justify-center">
						<Building2 className="h-5 w-5 text-primary" />
					</div>
					<div>
						<div className="font-medium text-foreground flex items-center gap-2 group">{organization.mspId}</div>
						{organization.description && <p className="text-sm text-muted-foreground">{organization.description}</p>}
						<div className="mt-1 flex items-center gap-2">
							{organization.providerName && (
								<div className="flex items-center gap-1">
									<Key className="h-3 w-3 text-muted-foreground" />
									<span className="text-xs text-muted-foreground">Provider: {organization.providerName}</span>
								</div>
							)}
							{organization.createdAt && (
								<div className="flex items-center gap-1">
									<span className="text-xs text-muted-foreground">
										Created <TimeAgo date={organization.createdAt} />
									</span>
								</div>
							)}
						</div>
					</div>
				</div>
				<div onClick={handleDropdownClick}>
					<DropdownMenu>
						<DropdownMenuTrigger asChild>
							<Button variant="ghost" size="icon" className="relative z-10">
								<MoreVertical className="h-4 w-4" />
								<span className="sr-only">Open menu</span>
							</Button>
						</DropdownMenuTrigger>
						<DropdownMenuContent align="end">
							<DropdownMenuItem
								className="text-destructive"
								onSelect={(e) => {
									e.preventDefault()
									onDelete(organization)
								}}
							>
								<Trash className="h-4 w-4 mr-2" />
								Delete
							</DropdownMenuItem>
						</DropdownMenuContent>
					</DropdownMenu>
				</div>
			</div>
			<div className="mt-4">
				<div className="flex gap-2">
					{organization.providerName && (
						<Badge variant="outline" className="text-xs">
							{organization.providerName}
						</Badge>
					)}
				</div>
			</div>
		</Link>
	)
}
