import { Button } from '@/components/ui/button'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2, ExternalLink } from 'lucide-react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import logo from '../../../public/logo.svg'

const DOCS_URL = 'https://docs.chainlaunch.dev'

const formSchema = z.object({
	username: z.string().min(1, 'Username is required'),
	password: z.string().min(1, 'Password is required'),
})

type FormValues = z.infer<typeof formSchema>

interface LoginFormProps {
	onSubmit: (data: FormValues) => void
	isLoading?: boolean
}

export function LoginForm({ onSubmit, isLoading }: LoginFormProps) {
	const form = useForm<FormValues>({
		resolver: zodResolver(formSchema),
		defaultValues: {
			username: '',
			password: '',
		},
	})

	return (
		<div className="w-full max-w-sm space-y-8">
			<div className="space-y-2 text-center">
				<div className="flex items-center justify-center gap-2 mb-4">
					<div className="flex aspect-square size-8 items-center justify-center rounded-lg dark:text-sidebar-primary-foreground bg-black dark:bg-transparent">
						<img src={logo} alt="logo" className="size-full" />
					</div>
					<h1 className="text-xl font-semibold">ChainLaunch</h1>
				</div>
				<p className="text-sm text-muted-foreground">Enter your credentials to access your account</p>
			</div>

			<Form {...form}>
				<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
					<FormField
						control={form.control}
						name="username"
						render={({ field }) => (
							<FormItem>
								<FormLabel>Username</FormLabel>
								<FormControl>
									<Input placeholder="Enter your username" {...field} />
								</FormControl>
								<FormMessage />
							</FormItem>
						)}
					/>
					<FormField
						control={form.control}
						name="password"
						render={({ field }) => (
							<FormItem>
								<FormLabel>Password</FormLabel>
								<FormControl>
									<Input type="password" placeholder="Enter your password" {...field} />
								</FormControl>
								<FormMessage />
							</FormItem>
						)}
					/>
					<Button type="submit" className="w-full" disabled={isLoading}>
						{isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
						Sign in
					</Button>
				</form>
			</Form>

			<div className="space-y-3 pt-2">
				<p className="text-xs text-center text-muted-foreground">
					First time? Your credentials are in{' '}
					<code className="bg-muted px-1 py-0.5 rounded text-[11px]">~/.chainlaunch/credentials.txt</code>
				</p>
				<div className="flex justify-center">
					<a
						href={DOCS_URL}
						target="_blank"
						rel="noopener noreferrer"
						className="text-xs text-muted-foreground hover:text-foreground inline-flex items-center gap-1 transition-colors"
					>
						Documentation
						<ExternalLink className="h-3 w-3" />
					</a>
				</div>
			</div>
		</div>
	)
}
