import Link from "next/link";
import { getAllProducts } from "@/lib/products";

// Lower-priority page, no dedicated rotation track: a lightweight, always-on visual
// style to round out the store without adding scope beyond what the config knobs cover.
export default function DealsPage() {
  const deals = getAllProducts().slice(0, 8);
  return (
    <main className="page-shell">
      <h1 className="page-title">Today&apos;s deals</h1>
      <div className="deals-strip">
        {deals.map((product) => (
          <Link href={`/product/${product.slug}`} className="deal-card" key={product.slug}>
            <img src={product.images[0]} alt={product.name} />
            <p style={{ margin: "8px 0 2px", fontWeight: 600, fontSize: "0.9rem" }}>
              {product.name}
            </p>
            <p style={{ margin: 0, fontWeight: 700 }}>
              {product.currency} {product.price.toLocaleString("en-IN")}
            </p>
          </Link>
        ))}
      </div>
    </main>
  );
}
