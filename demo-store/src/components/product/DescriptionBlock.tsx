import type { Product } from "@/lib/products";
import type { ProductLayoutVariant } from "@/lib/scenarios/productDetail";

export function DescriptionBlock({
  product,
  style,
}: {
  product: Product;
  style: ProductLayoutVariant["descriptionStyle"];
}) {
  if (style === "tabs") {
    return (
      <div className="pdp-description tabs">
        <div className="tab-bar">
          <span className="active">Description</span>
          <span>Specs</span>
          <span>Reviews ({product.rating.toFixed(1)}★)</span>
        </div>
        <p>{product.description}</p>
      </div>
    );
  }
  if (style === "accordion") {
    return (
      <div className="pdp-description accordion">
        <details open>
          <summary>Description</summary>
          <p>{product.description}</p>
        </details>
        <details>
          <summary>Reviews ({product.rating.toFixed(1)}★)</summary>
        </details>
      </div>
    );
  }
  return (
    <div className="pdp-description scroll">
      <p>{product.description}</p>
      <p>Rated {product.rating.toFixed(1)} out of 5 by shoppers.</p>
    </div>
  );
}
