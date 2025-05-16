import { Button } from '@/components/ui/button'
import { ChevronLeft, ChevronRight, MoreHorizontal } from 'lucide-react'
import { cn } from '@/lib/utils'

interface PaginationProps {
	currentPage: number
	totalItems: number
	pageSize: number
	onPageChange: (page: number) => void
	siblingsCount?: number
}

export function Pagination({ currentPage, totalItems, pageSize, onPageChange, siblingsCount = 1 }: PaginationProps) {
	const totalPages = Math.max(1, Math.ceil(totalItems / pageSize))

	const generatePagesArray = (from: number, to: number) => {
		return Array.from({ length: to - from + 1 }, (_, index) => from + index)
	}

	const renderPageButtons = () => {
		const leftSiblingIndex = Math.max(currentPage - siblingsCount, 1)
		const rightSiblingIndex = Math.min(currentPage + siblingsCount, totalPages)

		const shouldShowLeftDots = leftSiblingIndex > 2
		const shouldShowRightDots = rightSiblingIndex < totalPages - 1

		if (totalPages <= 7) {
			return generatePagesArray(1, totalPages)
		}

		if (!shouldShowLeftDots && shouldShowRightDots) {
			const leftItemCount = 3 + 2 * siblingsCount
			const leftRange = generatePagesArray(1, leftItemCount)
			return [...leftRange, 'dots', totalPages]
		}

		if (shouldShowLeftDots && !shouldShowRightDots) {
			const rightItemCount = 3 + 2 * siblingsCount
			const rightRange = generatePagesArray(totalPages - rightItemCount + 1, totalPages)
			return [1, 'dots', ...rightRange]
		}

		if (shouldShowLeftDots && shouldShowRightDots) {
			const middleRange = generatePagesArray(leftSiblingIndex, rightSiblingIndex)
			return [1, 'dots', ...middleRange, 'dots', totalPages]
		}
	}

	return (
		<div className={cn('flex items-center justify-center gap-2')}>
			<Button variant="outline" size="icon" onClick={() => onPageChange(currentPage - 1)} disabled={currentPage === 1}>
				<ChevronLeft className="h-4 w-4" />
			</Button>

			<div className="flex items-center gap-1">
				{renderPageButtons()?.map((page, index) => {
					if (page === 'dots') {
						return (
							<Button key={`dots-${index}`} variant="ghost" size="icon" disabled className="cursor-default">
								<MoreHorizontal className="h-4 w-4" />
							</Button>
						)
					}

					return (
						<Button key={page} variant={currentPage === page ? 'default' : 'outline'} size="icon" onClick={() => onPageChange(page as number)}>
							{page}
						</Button>
					)
				})}
			</div>

			<Button variant="outline" size="icon" onClick={() => onPageChange(currentPage + 1)} disabled={currentPage === totalPages}>
				<ChevronRight className="h-4 w-4" />
			</Button>
		</div>
	)
}
