import { notFound } from "next/navigation";
import { getProductBySlug } from "@/lib/products";
import { parseDemoVariantOverrides, resolveVariant } from "@/lib/rotation";
import {
  PRODUCT_LAYOUT_VARIANTS,
  PRODUCT_LABEL_VARIANTS,
} from "@/lib/scenarios/productDetail";
import { NUM_JSONLD_SHAPES, buildJsonLdShape } from "@/lib/jsonld";
import { Gallery } from "@/components/product/Gallery";
import { BuyBox } from "@/components/product/BuyBox";
import { DescriptionBlock } from "@/components/product/DescriptionBlock";
import { JsonLdScripts } from "@/components/product/JsonLdScripts";
import { VariantCycleButton } from "@/components/VariantCycleButton";

export const dynamic = "force-dynamic";

export default async function ProductPage({
  params,
  searchParams,
}: {
  params: Promise<{ slug: string }>;
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}) {
  const { slug } = await params;
  const product = getProductBySlug(slug);
  if (!product) notFound();

  const overrides = parseDemoVariantOverrides(await searchParams);

  const [layoutResult, jsonldResult, labelResult] = await Promise.all([
    resolveVariant("product_layout", PRODUCT_LAYOUT_VARIANTS.length, overrides),
    resolveVariant("product_jsonld_shape", NUM_JSONLD_SHAPES, overrides),
    resolveVariant("product_label", PRODUCT_LABEL_VARIANTS.length, overrides),
  ]);

  const layout = PRODUCT_LAYOUT_VARIANTS[layoutResult.variantIndex];
  const label = PRODUCT_LABEL_VARIANTS[labelResult.variantIndex];
  // Ties the plain/full-URL availability form to the same jsonld_shape rotation instead
  // of spending a whole extra track on it.
  const urlAvailabilityForm = jsonldResult.variantIndex % 2 === 1;
  const jsonLdShape = buildJsonLdShape(product, jsonldResult.variantIndex, urlAvailabilityForm);

  const main = (
    <div className={`pdp gallery-${layout.galleryPosition}`}>
      <Gallery product={product} />
      <div style={{ flex: 1, minWidth: 0 }}>
        <BuyBox product={product} label={label} />
        <DescriptionBlock product={product} style={layout.descriptionStyle} />
      </div>
    </div>
  );

  return (
    <main className="page-shell">
      {jsonLdShape.placement === "top" && <JsonLdScripts shape={jsonLdShape} />}
      <div className="variant-note">
        <span className="variant-track">
          layout: {layout.label} (every {layoutResult.rotateEveryN} loads)
          <VariantCycleButton trackKey="product_layout" />
        </span>
        <span className="variant-track">
          ld+json: {jsonLdShape.label} (every {jsonldResult.rotateEveryN} loads)
          <VariantCycleButton trackKey="product_jsonld_shape" />
        </span>
        <span className="variant-track">
          copy: &quot;{label.ctaText}&quot; (every {labelResult.rotateEveryN} loads)
          <VariantCycleButton trackKey="product_label" />
        </span>
      </div>
      {main}
      {jsonLdShape.placement === "middle" && <JsonLdScripts shape={jsonLdShape} />}
      {jsonLdShape.placement === "bottom" && <JsonLdScripts shape={jsonLdShape} />}
    </main>
  );
}
