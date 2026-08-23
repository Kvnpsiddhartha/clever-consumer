import {
  CATEGORY_STRUCTURE_VARIANTS,
  CATEGORY_STYLE_VARIANTS,
} from "@/lib/scenarios/categoryGrid";
import { parseDemoVariantOverrides, resolveVariant } from "@/lib/rotation";
import { CATEGORIES, categoryLabel, getProductsByCategory } from "@/lib/products";
import { ProductTile } from "@/components/category/ProductTile";
import { VariantCycleButton } from "@/components/VariantCycleButton";

export const dynamic = "force-dynamic";

export default async function HomePage({
  searchParams,
}: {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}) {
  const params = await searchParams;
  const overrides = parseDemoVariantOverrides(params);

  const [structureResult, styleResult] = await Promise.all([
    resolveVariant("category_structure", CATEGORY_STRUCTURE_VARIANTS.length, overrides),
    resolveVariant("category_style", CATEGORY_STYLE_VARIANTS.length, overrides),
  ]);

  const structureVariant = CATEGORY_STRUCTURE_VARIANTS[structureResult.variantIndex];
  const styleVariant = CATEGORY_STYLE_VARIANTS[styleResult.variantIndex];

  return (
    <main className="page-shell">
      <h1 className="page-title">Shop everything</h1>
      <div className="variant-note">
        <span className="variant-track">
          structure: {structureVariant.label} (every {structureResult.rotateEveryN} loads)
          <VariantCycleButton trackKey="category_structure" />
        </span>
        <span className="variant-track">
          style: {styleVariant.label} (every {styleResult.rotateEveryN} loads)
          <VariantCycleButton trackKey="category_style" />
        </span>
      </div>
      {CATEGORIES.map((category) => (
        <section key={category} style={{ marginBottom: 40 }}>
          <h2 style={{ fontSize: "1.1rem", marginBottom: 16 }}>{categoryLabel(category)}</h2>
          <div className={`category-grid ${styleVariant.style}`}>
            {getProductsByCategory(category).map((product) => (
              <ProductTile
                key={product.slug}
                product={product}
                structureVariant={structureVariant}
                style={styleVariant.style}
              />
            ))}
          </div>
        </section>
      ))}
    </main>
  );
}
