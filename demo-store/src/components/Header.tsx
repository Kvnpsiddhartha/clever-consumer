import Link from "next/link";
import { config } from "@/lib/config";

export function Header() {
  return (
    <header className="site-header">
      <Link href="/" className="brand">
        {config.storeBrandName}
      </Link>
      <nav className="site-nav">
        <Link href="/">Shop</Link>
        <Link href="/search">Search</Link>
        <Link href="/deals">Deals</Link>
      </nav>
    </header>
  );
}
