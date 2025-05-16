import { Sidebar, SidebarContent, SidebarFooter, SidebarGroup, SidebarGroupLabel, SidebarHeader, SidebarMenu, SidebarMenuButton, SidebarMenuItem, useSidebar } from '@/components/ui/sidebar'
import { BadgeCheck, Bell, Building, ChevronsUpDown, DatabaseBackup, FileText, Globe, Key, LogOut, Network, Puzzle, Server, Settings, Share2, User } from 'lucide-react'
;('use client')

// import { Project } from '@/api/client'
import { useAuth } from '@/contexts/AuthContext'
// import { useProjects } from '@/contexts/ProjectsContext'
import { type LucideIcon } from 'lucide-react'
import { useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import logo from '../../../public/logo.svg'
import { ProBadge } from '../pro/ProBadge'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '../ui/alert-dialog'
import { Avatar, AvatarFallback } from '../ui/avatar'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from '../ui/dropdown-menu'

type NavItem = {
	title: string
	url: string
	icon: LucideIcon | React.FC<{ className?: string }>
	isPro?: boolean
	roles?: string[]
}

const data = {
	navMain: [
		{
			title: 'Platform',
			items: [
				{
					title: 'Nodes',
					url: '/nodes',
					icon: Server,
				},
				{
					title: 'Plugins',
					url: '/plugins',
					icon: Puzzle,
					roles: ['admin', 'manager'],
				},
				{
					title: 'Monitoring',
					url: '/settings/monitoring',
					icon: Bell,
				},
				{
					title: 'Networks',
					url: '/networks',
					icon: Network,
				},
			],
		},
		{
			title: 'Fabric',
			items: [
				{
					title: 'Organizations',
					url: '/fabric/organizations',
					icon: Building,
				},
			],
		},
		{
			title: 'Settings',
			items: [
				{
					title: 'Key management',
					url: '/settings/keys',
					icon: Key,
				},
				{
					title: 'Users',
					url: '/users',
					icon: BadgeCheck,
					roles: ['admin'],
				},
				{
					title: 'Backups',
					url: '/settings/backups',
					icon: DatabaseBackup,
					roles: ['admin', 'manager'],
				},
				{
					title: 'Settings',
					url: '/settings/general',
					icon: Settings,
					roles: ['admin', 'manager'],
				},
				{
					title: 'Audit Logs',
					url: '/settings/audit-logs',
					icon: FileText,
					roles: ['admin'],
				},
			],
		},
		{
			title: 'API',
			items: [
				{
					title: 'API Documentation',
					url: '/docs',
					icon: FileText,
					roles: ['admin', 'manager', 'viewer'],
				},
			],
		},
		{
			title: 'Connect',
			items: [
				{
					title: 'Connect',
					url: '/connect',
					icon: Globe,
					isPro: true,
					roles: ['admin', 'manager'],
				},
				{
					title: 'External Nodes',
					url: '/external-nodes',
					icon: Network,
					isPro: true,
					roles: ['admin', 'manager', 'viewer'],
				},
				{
					title: 'Shared Networks',
					url: '/networks/fabric/shared',
					icon: Share2,
					isPro: true,
					roles: ['admin', 'manager'],
				},
			],
		},
	],
}
function NavMain({
	items,
}: {
	items: {
		title: string
		items: NavItem[]
	}[]
}) {
	const location = useLocation()
	const { user } = useAuth()

	return (
		<>
			{items.map((section) => {
				// Filter items based on user role
				const filteredItems = section.items.filter((item) => {
					// If no roles specified, show to everyone
					if (!item.roles) return true
					// Otherwise only show if user has required role
					return item.roles.includes(user?.role || '')
				})

				// Don't render section if no visible items
				if (filteredItems.length === 0) return null

				return (
					<SidebarGroup key={section.title}>
						<SidebarGroupLabel>{section.title}</SidebarGroupLabel>
						<SidebarMenu>
							{filteredItems.map((item) => {
								const isActive = location.pathname.startsWith(item.url)
								const Icon = item.icon
								return (
									<SidebarMenuItem key={item.title}>
										<SidebarMenuButton asChild tooltip={item.title} className={isActive ? 'bg-sidebar-accent text-sidebar-accent-foreground' : ''}>
											<Link to={item.url}>
												<Icon className="size-4" />
												<span>{item.title}</span>
												{item.isPro && <ProBadge />}
											</Link>
										</SidebarMenuButton>
									</SidebarMenuItem>
								)
							})}
						</SidebarMenu>
					</SidebarGroup>
				)
			})}
		</>
	)
}

export default function AppSidebar() {
	return (
		<Sidebar>
			<SidebarHeader>
				<SidebarMenu>
					<SidebarMenuItem>
						<div className="flex items-center gap-2">
							<div className="flex aspect-square size-8 items-center justify-center rounded-lg  dark:text-sidebar-primary-foreground bg-black dark:bg-transparent">
								<img src={logo} alt="logo" className="size-full" />
							</div>
							<div className="grid flex-1 text-left text-sm leading-tight">
								<span className="truncate font-semibold">ChainLaunch</span>
								<span className="truncate text-xs">v1.0.0</span>
							</div>
						</div>
					</SidebarMenuItem>
				</SidebarMenu>
			</SidebarHeader>
			<SidebarContent>
				<NavMain items={data.navMain} />
			</SidebarContent>
			<SidebarFooter>
				<NavUser />
			</SidebarFooter>
		</Sidebar>
	)
}

function NavUser() {
	const { user } = useAuth()
	const { isMobile } = useSidebar()
	const { logout } = useAuth()
	const [isLoggingOut, setIsLoggingOut] = useState(false)
	const [loggedOutOpen, setLoggedOutOpen] = useState(false)
	if (!user) return null

	const handleLogout = async () => {
		setIsLoggingOut(true)
		try {
			await logout()
		} finally {
			setIsLoggingOut(false)
		}
	}

	return (
		<>
			<AlertDialog open={loggedOutOpen} onOpenChange={setLoggedOutOpen}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Are you sure you want to log out?</AlertDialogTitle>
						<AlertDialogDescription>You will need to log in again to access your account.</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction onClick={handleLogout} disabled={isLoggingOut} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
							{isLoggingOut ? 'Logging out...' : 'Log out'}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
			<SidebarMenu>
				<SidebarMenuItem>
					<DropdownMenu>
						<DropdownMenuTrigger asChild>
							<SidebarMenuButton size="lg" className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground">
								<Avatar className="h-8 w-8 rounded-lg">
									<AvatarFallback className="rounded-lg">{user.username?.slice(0, 2).toUpperCase() || 'U'}</AvatarFallback>
								</Avatar>
								<div className="grid flex-1 text-left text-sm leading-tight">
									<span className="truncate font-semibold">{user.username || 'User'}</span>
								</div>
								<ChevronsUpDown className="ml-auto size-4" />
							</SidebarMenuButton>
						</DropdownMenuTrigger>
						<DropdownMenuContent className="w-[--radix-dropdown-menu-trigger-width] min-w-56 rounded-lg" side={isMobile ? 'bottom' : 'right'} align="end" sideOffset={4}>
							<DropdownMenuLabel className="p-0 font-normal">
								<div className="flex items-center gap-2 px-1 py-1.5 text-left text-sm">
									<Avatar className="h-8 w-8 rounded-lg">
										<AvatarFallback className="rounded-lg">{user.username?.slice(0, 2).toUpperCase() || 'U'}</AvatarFallback>
									</Avatar>
									<div className="grid flex-1 text-left text-sm leading-tight">
										<span className="truncate font-semibold">{user.username || 'User'}</span>
									</div>
								</div>
							</DropdownMenuLabel>
							<DropdownMenuSeparator />
							<DropdownMenuItem asChild>
								<Link to="/account">
									<User className="mr-2 h-4 w-4" />
									Account
								</Link>
							</DropdownMenuItem>
							<DropdownMenuItem className="cursor-pointer text-destructive focus:text-destructive" onClick={() => setLoggedOutOpen(true)}>
								<LogOut className="mr-2 h-4 w-4" />
								Log out
							</DropdownMenuItem>
						</DropdownMenuContent>
					</DropdownMenu>
				</SidebarMenuItem>
			</SidebarMenu>
		</>
	)
}
