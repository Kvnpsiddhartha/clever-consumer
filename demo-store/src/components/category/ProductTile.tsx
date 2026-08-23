import Link from "next/link";
import type { Product } from "@/lib/products";
import type { CategoryStructureVariant, CategoryStyle } from "@/lib/scenarios/categoryGrid";
import { AvailabilityPill } from "../AvailabilityPill";

/**
 * Renders one product tile for the category grid. The rotating `structureVariant`
 * decides the wrapper/title/price className *values* (proving selector renaming doesn't
 * matter) and, via `depth`/`order`, how many wrapper <div>s surround the content and what
 * DOM order the image/name/price/CTA appear in (proving depth/sibling-order doesn't
 * matter either). The `style` prop decides the actual visual layout (grid/list/etc.),
 * driven by shared CSS classes in globals.css.
 */
export function ProductTile({
  product,
  structureVariant,
  style,
}: {
  product: Product;
  structureVariant: CategoryStructureVariant;
  style: CategoryStyle;
}) {
  const media = (
    <div className="tile__media" key="image">
      <img src={product.images[0]} alt={product.name} />
    </div>
  );
  const name = (
    <p className={`tile__name ${structureVariant.titleClass}`} key="name">
      {product.name}
    </p>
  );
  const price = (
    <p className={`tile__price ${structureVariant.priceClass}`} key="price">
      {product.currency} {product.price.toLocaleString("en-IN")}
    </p>
  );
  const cta = (
    <button className="tile__cta" key="cta" type="button">
      View
    </button>
  );

  const partsByKey = { image: media, name, price, cta };
  const orderedBody = structureVariant.order
    .filter((k) => k !== "image")
    .map((k) => partsByKey[k]);

  const body = <div className="tile__body">{orderedBody}</div>;

  const showImageFirst = structureVariant.order[0] === "image";
  const content = showImageFirst ? [media, body] : [body, media];

  const inner =
    structureVariant.depth === "nested" ? (
      <div className="tile__shell">
        <div className="tile__inner">{content}</div>
      </div>
    ) : (
      content
    );

  return (
    <Link href={`/product/${product.slug}`} className={`tile ${structureVariant.wrapperClass}`}>
      {inner}
      <AvailabilityPill availability={product.availability} />
    </Link>
  );
}
