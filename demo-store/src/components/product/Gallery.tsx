import type { Product } from "@/lib/products";

export function Gallery({ product }: { product: Product }) {
  return (
    <div className="pdp-gallery">
      <img src={product.images[0]} alt={product.name} />
      {product.images.length > 1 && (
        <div className="thumbs">
          {product.images.slice(1).map((src) => (
            <img src={src} alt="" key={src} />
          ))}
        </div>
      )}
    </div>
  );
}
