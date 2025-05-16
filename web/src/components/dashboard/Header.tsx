import { ThemeToggle } from '@/components/theme/ThemeToggle'
import { useBreadcrumbs } from '@/contexts/BreadcrumbContext'
import { Link } from 'react-router-dom'
import { Breadcrumb, BreadcrumbItem, BreadcrumbLink, BreadcrumbList, BreadcrumbPage, BreadcrumbSeparator } from '../ui/breadcrumb'
import { Separator } from '../ui/separator'
import { SidebarTrigger } from '../ui/sidebar'
import { Button } from '../ui/button'
import { MessageSquare, ExternalLink } from 'lucide-react'

export function Header() {
	const { breadcrumbs } = useBreadcrumbs()

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
