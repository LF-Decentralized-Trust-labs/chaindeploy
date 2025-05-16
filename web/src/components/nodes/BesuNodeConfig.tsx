import { ServiceBesuNodeProperties } from "@/api/client"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"

interface BesuNodeConfigProps {
  config: ServiceBesuNodeProperties
}

export function BesuNodeConfig({ config }: BesuNodeConfigProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Besu Node Configuration</CardTitle>
        <CardDescription>Besu-specific node settings</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="grid grid-cols-2 gap-4">
          <div>
            <p className="text-sm font-medium text-muted-foreground">Node Key</p>
            <p>Key ID: {config.keyId}</p>
          </div>
          <div>
            <p className="text-sm font-medium text-muted-foreground">Network ID</p>
            <p>{config.networkId}</p>
          </div>
        </div>

        <Separator />

        <div className="space-y-2">
          <p className="text-sm font-medium text-muted-foreground">Network Configuration</p>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <p className="text-sm font-medium text-muted-foreground">P2P Configuration</p>
              <p className="text-sm">Host: {config.p2pHost}</p>
              <p className="text-sm">Port: {config.p2pPort}</p>
            </div>
            <div>
              <p className="text-sm font-medium text-muted-foreground">RPC Configuration</p>
              <p className="text-sm">Host: {config.rpcHost}</p>
              <p className="text-sm">Port: {config.rpcPort}</p>
            </div>
          </div>
        </div>

        <Separator />

        <div className="space-y-2">
          <p className="text-sm font-medium text-muted-foreground">Metrics Configuration</p>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <p className="text-sm font-medium text-muted-foreground">Status</p>
              <p className="text-sm">{config.metricsEnabled ? 'Enabled' : 'Disabled'}</p>
            </div>
            {config.metricsEnabled && (
              <div>
                <p className="text-sm font-medium text-muted-foreground">Endpoint</p>
                <p className="text-sm">Host: {config.metricsHost}</p>
                <p className="text-sm">Port: {config.metricsPort}</p>
                {config.metricsProtocol && (
                  <p className="text-sm">Protocol: {config.metricsProtocol}</p>
                )}
              </div>
            )}
          </div>
        </div>

        <Separator />

        <div className="space-y-2">
          <p className="text-sm font-medium text-muted-foreground">IP Configuration</p>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <p className="text-sm font-medium text-muted-foreground">External IP</p>
              <p className="text-sm">{config.externalIp}</p>
            </div>
            <div>
              <p className="text-sm font-medium text-muted-foreground">Internal IP</p>
              <p className="text-sm">{config.internalIp}</p>
            </div>
          </div>
        </div>

        <Separator />

        <div>
          <p className="text-sm font-medium text-muted-foreground">Enode URL</p>
          <p className="text-sm break-all">{config.enodeUrl}</p>
        </div>
      </CardContent>
    </Card>
  )
} 