import { BesuIcon } from '@/components/icons/besu-icon'
import { FabricIcon } from '@/components/icons/fabric-icon'
import { ThemeToggle } from '@/components/theme/ThemeToggle'
import { useBreadcrumbs } from '@/contexts/BreadcrumbContext'
import { ExternalLink, MessageSquare, Plus } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { Link } from 'react-router-dom'
import { Breadcrumb, BreadcrumbItem, BreadcrumbLink, BreadcrumbList, BreadcrumbPage, BreadcrumbSeparator } from '../ui/breadcrumb'
import { Button } from '../ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '../ui/dropdown-menu'
import { Separator } from '../ui/separator'
import { SidebarTrigger } from '../ui/sidebar'

export function Header() {
	const { breadcrumbs } = useBreadcrumbs()
	const navigate = useNavigate()

	return (
		<header className="flex h-16 shrink-0 items-center gap-2 border-b px-4">
			<div className="flex justify-between w-full">
				<div className="flex items-center">
					<SidebarTrigger className="-ml-1" />
					<Separator orientation="vertical" className="mr-2 h-4" />
					<Breadcrumb>
						<BreadcrumbList>
							{breadcrumbs.map((item, index) => (
								<BreadcrumbItem key={index} className={index === 0 ? '' : ''}>
									{index < breadcrumbs.length - 1 ? (
										<>
											<BreadcrumbLink asChild href={item.href ?? '#'}>
												<Link to={item.href ?? '#'}>{item.label}</Link>
											</BreadcrumbLink>
											<BreadcrumbSeparator className="" />
										</>
									) : (
										<BreadcrumbPage>{item.label}</BreadcrumbPage>
									)}
								</BreadcrumbItem>
							))}
						</BreadcrumbList>
					</Breadcrumb>
				</div>
				<div className="ml-auto flex items-center space-x-4">
					{/* Quick Actions Dropdown */}
					<DropdownMenu>
						<DropdownMenuTrigger asChild>
							<Button variant="outline" size="icon">
								<Plus className="h-4 w-4" />
							</Button>
						</DropdownMenuTrigger>
						<DropdownMenuContent align="end" className="w-56">
							<DropdownMenuItem onClick={() => navigate('/networks/fabric/create')}>
								<FabricIcon className="mr-2 h-4 w-4" />
								Fabric Create Network
							</DropdownMenuItem>
							<DropdownMenuItem onClick={() => navigate('/networks/besu/bulk-create')}>
								<BesuIcon className="mr-2 h-4 w-4" />
								Besu Create Network
							</DropdownMenuItem>
						</DropdownMenuContent>
					</DropdownMenu>
					<Button variant="ghost" asChild>
						<a href="https://docs.google.com/forms/d/e/1FAIpQLScuyWa3iVJNm49scRK7Y21h7ecZQdLOf8ppGHn37AIIUqbVDw/viewform?usp=sharing" target="_blank" rel="noopener noreferrer" className="flex items-center gap-2">
							<MessageSquare className="h-4 w-4" />
							<span>Give Feedback</span>
							<ExternalLink className="h-3 w-3" />
						</a>
					</Button>
					<ThemeToggle />
				</div>
			</div>
		</header>
	)
}
