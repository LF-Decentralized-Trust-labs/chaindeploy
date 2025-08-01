import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { CertificateViewer } from "@/components/ui/certificate-viewer"
import { Eye, Network } from "lucide-react"

interface ConsenterConfigProps {
  consenters: Array<{
    host: string
    port: number
    client_tls_cert: string
    server_tls_cert: string
    identity?: string
    id?: number
    msp_id?: string
  }>
}

function ConsenterDetailsModal({ consenter }: { consenter: ConsenterConfigProps['consenters'][0] }) {
  console.log(consenter)
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="outline" size="sm">
          <Eye className="h-4 w-4 mr-1" />
          View Details
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-4xl max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Consenter Details</DialogTitle>
          <DialogDescription>
            Certificate information for {consenter.host}:{consenter.port}
          </DialogDescription>
        </DialogHeader>
        
        <div className="space-y-6">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <h4 className="text-sm font-medium mb-1">Host</h4>
              <p className="text-sm text-muted-foreground">{consenter.host}</p>
            </div>
            <div>
              <h4 className="text-sm font-medium mb-1">Port</h4>
              <p className="text-sm text-muted-foreground">{consenter.port}</p>
            </div>
            {consenter.id && (
              <div>
                <h4 className="text-sm font-medium mb-1">ID</h4>
                <p className="text-sm text-muted-foreground">{consenter.id}</p>
              </div>
            )}
            {consenter.msp_id && (
              <div>
                <h4 className="text-sm font-medium mb-1">MSP ID</h4>
                <p className="text-sm text-muted-foreground">{consenter.msp_id}</p>
              </div>
            )}
          </div>

          <Tabs defaultValue="identity" className="w-full">
            <TabsList className="grid w-full grid-cols-3">
              <TabsTrigger value="identity">Identity</TabsTrigger>
              <TabsTrigger value="client-tls">Client TLS</TabsTrigger>
              <TabsTrigger value="server-tls">Server TLS</TabsTrigger>
            </TabsList>
            
            <TabsContent value="identity" className="space-y-4">
              {consenter.identity ? (
                <CertificateViewer certificate={consenter.identity} label="Identity Certificate" />
              ) : (
                <p className="text-sm text-muted-foreground">No identity certificate available</p>
              )}
            </TabsContent>
            
            <TabsContent value="client-tls" className="space-y-4">
              <CertificateViewer certificate={consenter.client_tls_cert} label="Client TLS Certificate" />
            </TabsContent>
            
            <TabsContent value="server-tls" className="space-y-4">
              <CertificateViewer certificate={consenter.server_tls_cert} label="Server TLS Certificate" />
            </TabsContent>
          </Tabs>
        </div>
      </DialogContent>
    </Dialog>
  )
}

export function ConsenterConfig({ consenters }: ConsenterConfigProps) {
  return (
    <div className="space-y-4">
      {consenters.map((consenter, index) => (
        <Card key={index} className="p-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="h-8 w-8 rounded-md bg-primary/10 flex items-center justify-center">
                <Network className="h-4 w-4 text-primary" />
              </div>
              <div>
                <h4 className="font-medium">{consenter.host}</h4>
                <p className="text-sm text-muted-foreground">Port: {consenter.port}</p>
                {consenter.msp_id && (
                  <p className="text-xs text-muted-foreground">MSP: {consenter.msp_id}</p>
                )}
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Badge variant="outline">Active</Badge>
              <ConsenterDetailsModal consenter={consenter} />
            </div>
          </div>
        </Card>
      ))}
    </div>
  )
} 