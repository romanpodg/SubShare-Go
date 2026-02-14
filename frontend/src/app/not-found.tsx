import Link from "next/link";

export default function NotFound() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-bg px-4">
      <div className="text-center">
        <h1 className="text-8xl font-bold text-accent mb-4">404</h1>
        <p className="text-xl text-zinc-400 mb-8">Страница не найдена</p>
        <Link
          href="/"
          className="inline-block rounded-lg bg-accent px-6 py-2.5 text-sm font-medium text-white transition-colors hover:bg-accent-hover"
        >
          На главную
        </Link>
      </div>
    </div>
  );
}
