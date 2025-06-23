import { Card } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'

interface PolicyValue {
  identities?: Array<{
    principal: {
      msp_identifier: string
      role: string
    }
    principal_classification: string
  }>
  rule: {
    n_out_of?: {
      n: number
      rules: Array<{
        signed_by: number
      }>
    }
    rule?: string
    sub_policy?: string
  }
  version: number
}

interface Policy {
  type: number
  value: PolicyValue
}

interface PolicyCardProps {
  name: string
  policy: Policy
  modPolicy?: string
}

export function PolicyCard({ name, policy, modPolicy }: PolicyCardProps) {
  const getPolicyTypeName = (type: number) => {
    switch (type) {
      case 1:
        return 'Signature'
      case 2:
        return 'ImplicitMeta'
      case 3:
        return 'ImplicitMeta'
      default:
        return 'Unknown'
    }
  }

  const renderIdentities = (identities: PolicyValue['identities']) => {
    if (!identities) return null

    return (
      <div className="space-y-2">
        <p className="text-xs font-medium text-muted-foreground">Identities:</p>
        <div className="space-y-1">
          {identities.map((identity, index) => (
            <div key={index} className="flex items-center gap-2 text-xs">
              <Badge variant="outline" className="font-mono">
                {identity.principal.msp_identifier}
              </Badge>
              <span className="text-muted-foreground">as</span>
              <Badge variant="secondary">{identity.principal.role}</Badge>
            </div>
          ))}
        </div>
      </div>
    )
  }

  const renderRule = (rule: PolicyValue['rule']) => {
    if (!rule) return null

    if (rule.n_out_of) {
      return (
        <div className="space-y-2">
          <p className="text-xs font-medium text-muted-foreground">Rule:</p>
          <div className="space-y-1">
            <p className="text-xs">
              Require <span className="font-medium">{rule.n_out_of.n}</span> out of{' '}
              <span className="font-medium">{rule.n_out_of.rules.length}</span> signatures
            </p>
            {rule.n_out_of.rules.map((r, index) => (
              <p key={index} className="text-xs text-muted-foreground">
                • Signature {index + 1} must be from identity {r.signed_by + 1}
              </p>
            ))}
          </div>
        </div>
      )
    }

    if (rule.rule) {
      return (
        <div className="space-y-2">
          <p className="text-xs font-medium text-muted-foreground">Rule:</p>
          <div className="space-y-1">
            <Badge variant="secondary">{rule.rule}</Badge>
            {rule.sub_policy && (
              <p className="text-xs text-muted-foreground">
                Sub-policy: <span className="font-medium">{rule.sub_policy}</span>
              </p>
            )}
          </div>
        </div>
      )
    }

    return null
  }

  return (
    <Card className="p-4">
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-medium">{name}</h3>
          <Badge variant="outline">{getPolicyTypeName(policy.type)}</Badge>
        </div>

        <ScrollArea className="h-[200px] pr-4">
          <div className="space-y-4">
            {renderIdentities(policy.value.identities)}
            {renderRule(policy.value.rule)}
            {modPolicy && (
              <div className="pt-2 border-t">
                <p className="text-xs text-muted-foreground">
                  Modified by: <span className="font-medium">{modPolicy}</span>
                </p>
              </div>
            )}
          </div>
        </ScrollArea>
      </div>
    </Card>
  )
} 