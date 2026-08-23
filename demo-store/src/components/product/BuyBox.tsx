import type { Product } from "@/lib/products";
import type { ProductLabelVariant } from "@/lib/scenarios/productDetail";

export function BuyBox({ product, label }: { product: Product; label: ProductLabelVariant }) {
  return (
    <div className="pdp-buybox">
      <p className="brand">{product.brand}</p>
      <h1>{product.name}</h1>
      <span className={`pill pill--${product.availability.replace(/_/g, "-")}`}>
        {label.stockLabels[product.availability]}
      </span>
      <p className="price">
        {label.pricePrefix}
        {product.price.toLocaleString("en-IN")}
        {label.priceSuffix}
      </p>
      <button type="button" className="cta">
        {label.ctaText}
      </button>
    </div>
  );
}
