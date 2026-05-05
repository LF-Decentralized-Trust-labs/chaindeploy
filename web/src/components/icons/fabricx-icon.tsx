import fabricLogo from '../../../public/blockchains/fabric_stroke.svg'

interface FabricXIconProps {
  className?: string
}

export function FabricXIcon({ className }: FabricXIconProps) {
  return (
    <span className={`relative inline-flex items-center justify-center ${className ?? ''}`}>
      <img
        src={fabricLogo}
        alt="FabricX"
        className="w-full h-full [&_*]:fill-black dark:[&_*]:fill-white"
      />
      <span
        className="absolute -bottom-0.5 -right-0.5 flex items-center justify-center rounded-sm bg-cyan-500 text-white font-bold leading-none"
        style={{
          width: '55%',
          height: '55%',
          fontSize: '0.55em',
        }}
        aria-hidden
      >
        X
      </span>
    </span>
  )
}
