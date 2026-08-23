import Link from "next/link";
import {
  SEARCH_STRUCTURE_VARIANTS,
  SEARCH_LABEL_VARIANTS,
} from "@/lib/scenarios/searchFilters";
import { parseDemoVariantOverrides, resolveVariant } from "@/lib/rotation";
import { getAllProducts } from "@/lib/products";
import { FilterPanel } from "@/components/search/FilterPanel";
import { CATEGORY_STRUCTURE_VARIANTS } from "@/lib/scenarios/categoryGrid";
import { VariantCycleButton } from "@/components/VariantCycleButton";

export const dynamic = "force-dynamic";

export default async function SearchPage({
  searchParams,
}: {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}) {
  const params = await searchParams;
  const overrides = parseDemoVariantOverrides(params);

  const [structureResult, labelResult] = await Promise.all([
    resolveVariant("search_structure", SEARCH_STRUCTURE_VARIANTS.length, overrides),
    resolveVariant("search_label", SEARCH_LABEL_VARIANTS.length, overrides),
  ]);

  const structureVariant = SEARCH_STRUCTURE_VARIANTS[structureResult.variantIndex];
  const labelVariant = SEARCH_LABEL_VARIANTS[labelResult.variantIndex];
  const products = getAllProducts();
  const structureClass = CATEGORY_STRUCTURE_VARIANTS[0];

  return (
    <main className="page-shell">
      <h1 className="page-title">Search</h1>
      <div className="variant-note">
        <span className="variant-track">
          filters: {structureVariant.label} (every {structureResult.rotateEveryN} loads)
          <VariantCycleButton trackKey="search_structure" />
        </span>
        <span className="variant-track">
          labels: &quot;{labelVariant.heading}&quot; (every {labelResult.rotateEveryN} loads)
          <VariantCycleButton trackKey="search_label" />
        </span>
      </div>
      <div style={{ marginBottom: 16, color: "#6b6b6b", fontSize: "0.9rem" }}>
        {labelVariant.sortLabel}: Relevance
      </div>
      <div className={`search-layout placement-${structureVariant.placement}`}>
        <FilterPanel structure={structureVariant} labels={labelVariant} />
        <div className="search-results">
          <div className="content-list">
            {products.map((product) => (
              <Link href={`/product/${product.slug}`} className={`tile ${structureClass.wrapperClass}`} key={product.slug}>
                <div className="tile__media">
                  <img src={product.images[0]} alt={product.name} />
                </div>
                <p className="tile__name">{product.name}</p>
                <p className="tile__price">
                  {product.currency} {product.price.toLocaleString("en-IN")}
                </p>
                <span className={`pill pill--${product.availability.replace(/_/g, "-")}`}>
                  {labelVariant.availabilityLabels[product.availability]}
                </span>
              </Link>
            ))}
          </div>
        </div>
      </div>
    </main>
  );
}
