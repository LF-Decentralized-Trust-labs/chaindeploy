import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider } from 'next-themes'
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { Suspense, lazy } from 'react'
import { client } from './api/client'
import { Header } from './components/dashboard/Header'
import AppSidebar from './components/dashboard/Sidebar'
import { ProtectedLayout } from './components/layout/ProtectedLayout'
import { ThemeWrapper } from './components/theme/ThemeWrapper'
import { SidebarInset, SidebarProvider } from './components/ui/sidebar'
import config from './config'
import { AuthProvider } from './contexts/AuthContext'
import { BreadcrumbProvider } from './contexts/BreadcrumbContext'
import './globals.css'

// Lazy load all pages
const SharedNetworksPage = lazy(() => import('@/pages/networks/fabric/shared'))
const ImportNetworkPage = lazy(() => import('@/pages/networks/import'))
const CreateBesuNodePage = lazy(() => import('@/pages/nodes/besu/create'))
const CreateFabricNodePage = lazy(() => import('@/pages/nodes/fabric/create'))
const EditFabricNodePage = lazy(() => import('@/pages/nodes/fabric/edit'))
const NodesLogsPage = lazy(() => import('@/pages/nodes/logs'))
const CertificateTemplatesPage = lazy(() => import('./pages/identity/certificates'))
const MonitoringPage = lazy(() => import('./pages/monitoring'))
const UpdateProviderPage = lazy(() => import('./pages/monitoring/providers/[id]'))
const CreateProviderPage = lazy(() => import('./pages/monitoring/providers/new'))
const NetworksPage = lazy(() => import('./pages/networks'))
const BesuPage = lazy(() => import('./pages/networks/besu'))
const BesuNetworkDetailPage = lazy(() => import('./pages/networks/besu-page'))
const CreateBesuNetworkPage = lazy(() => import('./pages/networks/besu/create'))
const FabricPage = lazy(() => import('./pages/networks/fabric'))
const FabricNetworkDetailPage = lazy(() => import('./pages/networks/fabric-page'))
const FabricCreateChannel = lazy(() => import('./pages/networks/fabric/create'))
const OrganizationsPage = lazy(() => import('./pages/networks/fabric/organizations'))
const NodesPage = lazy(() => import('./pages/nodes'))
const NodeDetailPage = lazy(() => import('./pages/nodes/[id]'))
const BulkCreateNodesPage = lazy(() => import('./pages/nodes/fabric/bulk-create'))
const NotFoundPage = lazy(() => import('./pages/not-found'))
const OrganizationDetailPage = lazy(() => import('./pages/organizations/[id]'))
const AccessControlPage = lazy(() => import('./pages/settings/access'))
const BackupsPage = lazy(() => import('./pages/settings/backups'))
const SettingsPage = lazy(() => import('./pages/settings/general'))
const KeyManagementPage = lazy(() => import('./pages/settings/keys'))
const KeyDetailPage = lazy(() => import('./pages/settings/keys/[id]'))
const NetworkConfigPage = lazy(() => import('./pages/settings/network'))
const SmartContractsPage = lazy(() => import('./pages/smart-contracts'))
const BlocksOverview = lazy(() => import('@/components/networks/blocks-overview').then(module => ({ default: module.BlocksOverview })))
const BlockDetails = lazy(() => import('@/components/networks/block-details').then(module => ({ default: module.BlockDetails })))
const ApiDocumentationPage = lazy(() => import('./pages/api-documentation'))
const BulkCreateBesuNetworkPage = lazy(() => import('./pages/networks/besu/bulk-create'))
const EditBesuNodePage = lazy(() => import('./pages/nodes/besu/edit'))
const CreateNodePage = lazy(() => import('./pages/nodes/create'))
const PluginsPage = lazy(() => import('./pages/plugins'))
const PluginDetailPage = lazy(() => import('./pages/plugins/[name]'))
const EditPluginPage = lazy(() => import('./pages/plugins/[name]/edit'))
const NewPluginPage = lazy(() => import('./pages/plugins/new'))
const UsersPage = lazy(() => import('./pages/users'))
const AccountPage = lazy(() => import('./pages/account'))
const AuditLogsPage = lazy(() => import('@/pages/settings/audit-logs'))
const AuditLogDetailPage = lazy(() => import('@/pages/settings/audit-logs/[id]'))
const AnalyticsPage = lazy(() => import('./pages/platform/analytics'))
const FabricChaincodesPage = lazy(() => import('./pages/smart-contracts/fabric'))
const BesuContractsPage = lazy(() => import('./pages/smart-contracts/besu'))
const FabricChaincodeDefinitionDetail = lazy(() => import('./pages/smart-contracts/fabric/definition'))
const ChaincodeProjectDetailPage = lazy(() => import('./pages/smart-contracts/fabric/[id]'))
const ChaincodeProjectEditorPage = lazy(() => import('./pages/smart-contracts/fabric/[id]/editor'))
const ChaincodePlaygroundPage = lazy(() => import('./pages/smart-contracts/fabric/[id]/playground'))

import { Toaster } from './components/ui/sonner'
import { AlertCircle, CheckCircle, Loader2, X } from 'lucide-react'

// Loading component for Suspense fallback
const PageLoading = () => (
	<div className="flex items-center justify-center min-h-[400px]">
		<Loader2 className="h-8 w-8 animate-spin" />
		<span className="ml-2 text-sm text-muted-foreground">Loading...</span>
	</div>
)

const queryClient = new QueryClient({
	defaultOptions: {
		queries: {
			refetchOnWindowFocus: false,
			retry: false,
		},
	},
})

client.setConfig({ baseUrl: config.apiUrl })

const App2 = () => {
	return (
		<ThemeProvider defaultTheme="system" enableSystem attribute="class">
			<ThemeWrapper>
				<QueryClientProvider client={queryClient}>
					<BrowserRouter>
						<AuthProvider>
							<ProtectedLayout>
								<BreadcrumbProvider>
									<div className="p-0">
										<Suspense fallback={<PageLoading />}>
											<Routes>
												<Route path="/">
													<Route path="/" element={<Navigate to="/nodes" replace />} />
													<Route path="account" element={<AccountPage />} />
													<Route path="nodes" element={<NodesPage />} />
													<Route path="smart-contracts" element={<SmartContractsPage />} />
													<Route path="smart-contracts/fabric" element={<FabricChaincodesPage />} />
													<Route path="smart-contracts/besu" element={<BesuContractsPage />} />
													<Route path="monitoring" element={<MonitoringPage />} />
													<Route path="monitoring/providers/new" element={<CreateProviderPage />} />
													<Route path="monitoring/providers/:id" element={<UpdateProviderPage />} />
													<Route path="networks" element={<NetworksPage />} />
													<Route path="networks/import" element={<ImportNetworkPage />} />
													<Route path="network/fabric" element={<FabricPage />} />
													<Route path="network/besu" element={<BesuPage />} />
													<Route path="settings/access" element={<AccessControlPage />} />
													<Route path="settings/network" element={<NetworkConfigPage />} />
													<Route path="settings/keys" element={<KeyManagementPage />} />
													<Route path="settings/backups" element={<BackupsPage />} />
													<Route path="settings/general" element={<SettingsPage />} />
													<Route path="settings/monitoring" element={<MonitoringPage />} />
													<Route path="identity/certificates" element={<CertificateTemplatesPage />} />
													<Route path="fabric/organizations" element={<OrganizationsPage />} />
													<Route path="nodes/fabric/create" element={<CreateFabricNodePage />} />
													<Route path="nodes/fabric/edit/:id" element={<EditFabricNodePage />} />
													<Route path="nodes/besu/edit/:id" element={<EditBesuNodePage />} />
													<Route path="nodes/:id" element={<NodeDetailPage />} />
													<Route path="networks/fabric/create" element={<FabricCreateChannel />} />
													<Route path="networks/besu/create" element={<CreateBesuNetworkPage />} />
													<Route path="networks/:id/besu" element={<BesuNetworkDetailPage />} />
													<Route path="networks/:id/fabric" element={<FabricNetworkDetailPage />} />
													<Route path="networks/:id/blocks" element={<BlocksOverview />} />
													<Route path="networks/:id/blocks/:blockNumber" element={<BlockDetails />} />
													<Route path="organizations/:id" element={<OrganizationDetailPage />} />
													<Route path="settings/keys/:id" element={<KeyDetailPage />} />
													<Route path="nodes/create" element={<CreateNodePage />} />
													<Route path="nodes/fabric/bulk" element={<BulkCreateNodesPage />} />
													<Route path="nodes/logs" element={<NodesLogsPage />} />
													<Route path="nodes/besu/create" element={<CreateBesuNodePage />} />
													<Route path="networks/fabric/shared" element={<SharedNetworksPage />} />
													<Route path="docs" element={<ApiDocumentationPage />} />
													<Route path="networks/besu/bulk-create" element={<BulkCreateBesuNetworkPage />} />
													<Route path="plugins" element={<PluginsPage />} />
													<Route path="plugins/new" element={<NewPluginPage />} />
													<Route path="plugins/:name" element={<PluginDetailPage />} />
													<Route path="plugins/:name/edit" element={<EditPluginPage />} />
													<Route path="users" element={<UsersPage />} />
													<Route path="settings/audit-logs" element={<AuditLogsPage />} />
													<Route path="settings/audit-logs/:id" element={<AuditLogDetailPage />} />
													<Route path="platform/analytics" element={<AnalyticsPage />} />
													<Route path="sc/fabric/chaincodes/:id" element={<FabricChaincodeDefinitionDetail />} />
													<Route path="sc/fabric/projects/chaincodes/:id" element={<ChaincodeProjectDetailPage />} />
													<Route path="sc/fabric/projects/chaincodes/:id/editor" element={<ChaincodeProjectEditorPage />} />
													<Route path="smart-contracts/fabric/:id/playground" element={<ChaincodePlaygroundPage />} />
												</Route>
												<Route path="*" element={<NotFoundPage />} />
											</Routes>
										</Suspense>
									</div>
								</BreadcrumbProvider>
							</ProtectedLayout>
						</AuthProvider>
					</BrowserRouter>
				</QueryClientProvider>
			</ThemeWrapper>
			<Toaster
				toastOptions={{
					closeButton: true,
				}}
				position="top-center"
			/>
		</ThemeProvider>
	)
}

const App = () => {
	const location = useLocation()
	// Regex to match /sc/fabric/projects/chaincodes/:id/editor
	const hideLayout = /^\/sc\/fabric\/projects\/chaincodes\/[^/]+\/editor$/.test(location.pathname)

	return (
		<ThemeProvider defaultTheme="system" enableSystem attribute="class">
			<ThemeWrapper>
				<QueryClientProvider client={queryClient}>
					<AuthProvider>
						<ProtectedLayout>
							<BreadcrumbProvider>
								<SidebarProvider>
									{!hideLayout && <AppSidebar />}
									<SidebarInset>
										{!hideLayout && <Header />}
										<div className="p-0">
											<Suspense fallback={<PageLoading />}>
												<Routes>
													<Route path="/">
														<Route path="/" element={<Navigate to="/nodes" replace />} />
														<Route path="account" element={<AccountPage />} />
														<Route path="nodes" element={<NodesPage />} />
														<Route path="smart-contracts" element={<SmartContractsPage />} />
														<Route path="smart-contracts/fabric" element={<FabricChaincodesPage />} />
														<Route path="smart-contracts/besu" element={<BesuContractsPage />} />
														<Route path="monitoring" element={<MonitoringPage />} />
														<Route path="monitoring/providers/new" element={<CreateProviderPage />} />
														<Route path="monitoring/providers/:id" element={<UpdateProviderPage />} />
														<Route path="networks" element={<NetworksPage />} />
														<Route path="networks/import" element={<ImportNetworkPage />} />
														<Route path="network/fabric" element={<FabricPage />} />
														<Route path="network/besu" element={<BesuPage />} />
														<Route path="settings/access" element={<AccessControlPage />} />
														<Route path="settings/network" element={<NetworkConfigPage />} />
														<Route path="settings/keys" element={<KeyManagementPage />} />
														<Route path="settings/backups" element={<BackupsPage />} />
														<Route path="settings/general" element={<SettingsPage />} />
														<Route path="settings/monitoring" element={<MonitoringPage />} />
														<Route path="identity/certificates" element={<CertificateTemplatesPage />} />
														<Route path="fabric/organizations" element={<OrganizationsPage />} />
														<Route path="nodes/fabric/create" element={<CreateFabricNodePage />} />
														<Route path="nodes/fabric/edit/:id" element={<EditFabricNodePage />} />
														<Route path="nodes/besu/edit/:id" element={<EditBesuNodePage />} />
														<Route path="nodes/:id" element={<NodeDetailPage />} />
														<Route path="networks/fabric/create" element={<FabricCreateChannel />} />
														<Route path="networks/besu/create" element={<CreateBesuNetworkPage />} />
														<Route path="networks/:id/besu" element={<BesuNetworkDetailPage />} />
														<Route path="networks/:id/fabric" element={<FabricNetworkDetailPage />} />
														<Route path="networks/:id/blocks" element={<BlocksOverview />} />
														<Route path="networks/:id/blocks/:blockNumber" element={<BlockDetails />} />
														<Route path="organizations/:id" element={<OrganizationDetailPage />} />
														<Route path="settings/keys/:id" element={<KeyDetailPage />} />
														<Route path="nodes/create" element={<CreateNodePage />} />
														<Route path="nodes/fabric/bulk" element={<BulkCreateNodesPage />} />
														<Route path="nodes/logs" element={<NodesLogsPage />} />
														<Route path="nodes/besu/create" element={<CreateBesuNodePage />} />
														<Route path="networks/fabric/shared" element={<SharedNetworksPage />} />
														<Route path="docs" element={<ApiDocumentationPage />} />
														<Route path="networks/besu/bulk-create" element={<BulkCreateBesuNetworkPage />} />
														<Route path="plugins" element={<PluginsPage />} />
														<Route path="plugins/new" element={<NewPluginPage />} />
														<Route path="plugins/:name" element={<PluginDetailPage />} />
														<Route path="plugins/:name/edit" element={<EditPluginPage />} />
														<Route path="users" element={<UsersPage />} />
														<Route path="settings/audit-logs" element={<AuditLogsPage />} />
														<Route path="settings/audit-logs/:id" element={<AuditLogDetailPage />} />
														<Route path="platform/analytics" element={<AnalyticsPage />} />
														<Route path="sc/fabric/chaincodes/:id" element={<FabricChaincodeDefinitionDetail />} />
														<Route path="sc/fabric/projects/chaincodes/:id" element={<ChaincodeProjectDetailPage />} />
														<Route path="sc/fabric/projects/chaincodes/:id/editor" element={<ChaincodeProjectEditorPage />} />
														<Route path="smart-contracts/fabric/:id/playground" element={<ChaincodePlaygroundPage />} />
													</Route>
													<Route path="*" element={<NotFoundPage />} />
												</Routes>
											</Suspense>
										</div>
									</SidebarInset>
								</SidebarProvider>
							</BreadcrumbProvider>
						</ProtectedLayout>
					</AuthProvider>
				</QueryClientProvider>
			</ThemeWrapper>
			<Toaster
				icons={{
					success: <CheckCircle className="h-4 w-4" />,
					error: <AlertCircle className="h-4 w-4" />,
					loading: <Loader2 className="h-4 w-4 animate-spin" />,
					close: <X className="h-4 w-4" />,
				}}
				toastOptions={{
					closeButton: true,
				}}
				position="top-center"
			/>
		</ThemeProvider>
	)
}

export default App
