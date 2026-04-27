import { cn } from '@/lib/utils'
import { ReactNode } from 'react'

type MaxWidth = 'form' | 'detail' | 'dashboard' | 'wide' | 'full'

const maxWidthClass: Record<MaxWidth, string> = {
	form: 'max-w-2xl',
	detail: 'max-w-4xl',
	dashboard: 'max-w-6xl',
	wide: 'max-w-7xl',
	full: 'max-w-none',
}

interface PageShellProps {
	children: ReactNode
	maxWidth?: MaxWidth
	className?: string
}

/**
 * Standard two-element page shell for dashboard pages.
 * See chaindeploy/web/DESIGN.md for full rules.
 */
export function PageShell({ children, maxWidth = 'dashboard', className }: PageShellProps) {
	return (
		<div className="flex-1 py-8">
			<div className={cn('mx-auto w-full px-4 sm:px-6 lg:px-8', maxWidthClass[maxWidth], className)}>{children}</div>
		</div>
	)
}

interface PageHeaderProps {
	title: string
	description?: string
	children?: ReactNode
	className?: string
}

/**
 * Standard page header. Exactly one per page.
 * See DESIGN.md §5.
 */
export function PageHeader({ title, description, children, className }: PageHeaderProps) {
	return (
		<div className={cn('mb-8 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between', className)}>
			<div>
				<h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
				{description && <p className="mt-1 text-sm text-muted-foreground">{description}</p>}
			</div>
			{children && <div className="flex flex-wrap items-center gap-2">{children}</div>}
		</div>
	)
}
