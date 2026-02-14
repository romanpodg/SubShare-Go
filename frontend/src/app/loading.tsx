export default function Loading() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-bg">
      <div className="flex gap-1.5">
        <span className="h-2.5 w-2.5 rounded-full bg-accent animate-pulse [animation-delay:0ms]" />
        <span className="h-2.5 w-2.5 rounded-full bg-accent animate-pulse [animation-delay:150ms]" />
        <span className="h-2.5 w-2.5 rounded-full bg-accent animate-pulse [animation-delay:300ms]" />
      </div>
    </div>
  );
}
