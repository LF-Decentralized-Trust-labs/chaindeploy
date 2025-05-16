import { cn } from "@/lib/utils";

interface MetricsGridProps {
  children: React.ReactNode;
  className?: string;
}

export function MetricsGrid({ children, className }: MetricsGridProps) {
  return (
    <div className={cn(
      "grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4",
      className
    )}>
      {children}
    </div>
  );
} 