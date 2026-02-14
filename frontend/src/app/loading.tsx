import { LoadingSpinner } from "@/components/ui/LoadingSpinner";

export default function Loading() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-bg">
      <LoadingSpinner size="lg" />
    </div>
  );
}
