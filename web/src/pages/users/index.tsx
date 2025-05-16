import { deleteUsersByIdMutation, getUsersOptions, postUsersMutation, putUsersByIdMutation } from '@/api/client/@tanstack/react-query.gen'
import { AuthUpdateUserRequest } from '@/api/client/types.gen'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { CreateUserDialog } from '@/components/users/create-user-dialog'
import { EditUserDialog } from '@/components/users/edit-user-dialog'
import { useAuth } from '@/contexts/AuthContext'
import { useMutation, useQuery } from '@tanstack/react-query'
import { EllipsisVertical, UserPlus } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'

const UsersPage = () => {
	const [userToDelete, setUserToDelete] = useState<number | null>(null)
	const { user: currentUser } = useAuth()
	const isAdmin = currentUser?.role === 'admin'

	// Fetch users
	const { data: users, isLoading, error, refetch } = useQuery(getUsersOptions())

	// Create user mutation
	const createUserMutation = useMutation({
		...postUsersMutation(),
		onSuccess: () => {
			toast.success('User created successfully')
			refetch()
		},
		onError: (error: any) => {
			toast.error(`Failed to create user: ${error.message}`)
		},
	})

	// Update user role mutation
	const updateUserRoleMutation = useMutation({
		...putUsersByIdMutation(),
		onSuccess: () => {
			toast.success('User role updated successfully')
			refetch()
		},
		onError: (error: any) => {
			toast.error(`Failed to update user role: ${error.message}`)
		},
	})

	// Delete user mutation
	const deleteUserMutation = useMutation({
		...deleteUsersByIdMutation(),
		onSuccess: () => {
			toast.success('User deleted successfully')
			refetch()
		},
		onError: (error: any) => {
			toast.error(`Failed to delete user: ${error.message}`)
		},
	})

	const handleUpdateUser = async (userId: number, data: AuthUpdateUserRequest) => {
		await updateUserRoleMutation.mutateAsync({
			path: { id: userId },
			body: data,
		})
	}

	const handleDelete = async (userId: number) => {
		try {
			await deleteUserMutation.mutateAsync({
				path: { id: userId },
			})
		} catch (error) {
			// Error is handled by the mutation
		} finally {
			setUserToDelete(null)
		}
	}

	if (isLoading) {
		return (
			<div className="container p-8">
				{/* Loading skeleton */}
				<div className="space-y-4">
					<div className="h-8 w-32 bg-muted animate-pulse rounded" />
					<div className="h-96 bg-muted animate-pulse rounded" />
				</div>
			</div>
		)
	}

	if (error) {
		return (
			<div className="container p-8">
				<Card className="p-6 border-destructive">
					<div className="text-destructive">Error loading users: {error.message}</div>
				</Card>
			</div>
		)
	}

	return (
		<div className="container p-8">
			<div className="flex justify-between items-center mb-6">
				<h1 className="text-2xl font-bold">Users</h1>
				{isAdmin && <CreateUserDialog onSubmit={(data) => createUserMutation.mutateAsync({ body: data })} isLoading={createUserMutation.isPending} />}
			</div>

			{!users?.length ? (
				<Card className="flex flex-col items-center justify-center py-16">
					<div className="flex flex-col items-center gap-4 text-center">
						<div className="rounded-full bg-muted p-4">
							<UserPlus className="h-8 w-8 text-muted-foreground" />
						</div>
						<div className="space-y-2">
							<h3 className="text-xl font-semibold">No users found</h3>
							<p className="text-muted-foreground">Get started by adding your first user.</p>
						</div>
						{isAdmin && <CreateUserDialog onSubmit={(data) => createUserMutation.mutateAsync({ body: data })} isLoading={createUserMutation.isPending} />}
					</div>
				</Card>
			) : (
				<Card>
					<Table>
						<TableHeader>
							<TableRow>
								<TableHead>Username</TableHead>
								<TableHead>Role</TableHead>
								<TableHead>Created At</TableHead>
								<TableHead>Last Login</TableHead>
								{isAdmin && <TableHead className="w-[50px]"></TableHead>}
							</TableRow>
						</TableHeader>
						<TableBody>
							{users.map((user) => (
								<TableRow key={user.id}>
									<TableCell className="font-medium">{user.username}</TableCell>
									<TableCell>
										<span
											className={`inline-flex items-center rounded-full px-2 py-1 text-xs font-medium
                      ${user.role === 'admin' ? 'bg-blue-50 text-blue-700' : user.role === 'manager' ? 'bg-purple-50 text-purple-700' : 'bg-gray-50 text-gray-700'}`}
										>
											{user.role}
										</span>
									</TableCell>
									<TableCell>{new Date(user.created_at!).toLocaleDateString()}</TableCell>
									<TableCell>{user.last_login_at ? new Date(user.last_login_at).toLocaleDateString() : 'Never'}</TableCell>
									{isAdmin && (
										<TableCell>
											{user.id !== currentUser?.id && (
												<DropdownMenu>
													<DropdownMenuTrigger asChild>
														<Button variant="ghost" size="icon">
															<EllipsisVertical className="h-4 w-4" />
														</Button>
													</DropdownMenuTrigger>
													<DropdownMenuContent align="end">
														<EditUserDialog
															user={user}
															onSubmit={(data) => handleUpdateUser(user.id!, data)}
															isLoading={updateUserRoleMutation.isPending}
															trigger={<DropdownMenuItem onSelect={(e) => e.preventDefault()}>Edit User</DropdownMenuItem>}
														/>
														<DropdownMenuSeparator />
														<DropdownMenuItem className="text-destructive" onClick={() => setUserToDelete(user.id!)}>
															Delete User
														</DropdownMenuItem>
													</DropdownMenuContent>
												</DropdownMenu>
											)}
										</TableCell>
									)}
								</TableRow>
							))}
						</TableBody>
					</Table>
				</Card>
			)}

			<AlertDialog open={!!userToDelete} onOpenChange={(open) => !open && setUserToDelete(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Are you sure?</AlertDialogTitle>
						<AlertDialogDescription>This will permanently delete this user. This action cannot be undone.</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction className="bg-destructive text-destructive-foreground hover:bg-destructive/90" onClick={() => userToDelete && handleDelete(userToDelete)}>
							Delete
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	)
}

export default UsersPage
