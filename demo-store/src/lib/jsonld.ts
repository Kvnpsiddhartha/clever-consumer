import type { Availability, Product } from "./products";
import { config } from "./config";

/**
 * Rotates *where and how* the ld+json block sits in the page (see manager/brightdata.go's
 * decodeProductHTML: it regex-scans every <script type="application/ld+json"> tag and
 * returns the first one that parses as product JSON). Every shape below is always valid
 * -- this track proves the parser's "scan every script tag, unwrap @graph/arrays" logic
 * is robust, without ever failing a scrape.
 */
export const NUM_JSONLD_SHAPES = 4;

const SCHEMA_AVAILABILITY: Record<Availability, string> = {
  in_stock: "InStock",
  out_of_stock: "OutOfStock",
  preorder: "PreOrder",
  unknown: "InStock",
};

function availabilityValue(availability: Availability, urlForm: boolean): string {
  const suffix = SCHEMA_AVAILABILITY[availability];
  return urlForm ? `http://schema.org/${suffix}` : suffix;
}

function productJsonLd(product: Product, urlForm: boolean): Record<string, unknown> {
  return {
    "@context": "http://schema.org/",
    "@type": "Product",
    name: product.name,
    image: product.images,
    description: product.description,
    brand: product.brand,
    offers: {
      "@type": "Offer",
      priceCurrency: product.currency || config.defaultCurrency,
      price: product.price,
      availability: availabilityValue(product.availability, urlForm),
    },
  };
}

function organizationJsonLd(): Record<string, unknown> {
  return {
    "@context": "http://schema.org/",
    "@type": "Organization",
    name: config.storeBrandName,
  };
}

function breadcrumbJsonLd(product: Product): Record<string, unknown> {
  return {
    "@context": "http://schema.org/",
    "@type": "BreadcrumbList",
    itemListElement: [
      { "@type": "ListItem", position: 1, name: "Home" },
      { "@type": "ListItem", position: 2, name: product.category },
      { "@type": "ListItem", position: 3, name: product.name },
    ],
  };
}

export type JsonLdPlacement = "top" | "middle" | "bottom";

export interface JsonLdBlock {
  key: string;
  json: Record<string, unknown>;
}

export interface JsonLdShape {
  label: string;
  placement: JsonLdPlacement;
  blocks: JsonLdBlock[];
}

/** Builds the exact set of <script type="application/ld+json"> payloads for a shape variant. */
export function buildJsonLdShape(
  product: Product,
  shapeIndex: number,
  urlAvailabilityForm: boolean,
): JsonLdShape {
  switch (shapeIndex % NUM_JSONLD_SHAPES) {
    case 0:
      // Single Product object, rendered near the top of the page.
      return {
        label: "single object, top of page",
        placement: "top",
        blocks: [{ key: "product", json: productJsonLd(product, urlAvailabilityForm) }],
      };
    case 1:
      // Array-wrapped, rendered mid-page.
      return {
        label: "array-wrapped, mid-page",
        placement: "middle",
        blocks: [
          { key: "product-array", json: [productJsonLd(product, urlAvailabilityForm)] as unknown as Record<string, unknown> },
        ],
      };
    case 2:
      // @graph-wrapped alongside unrelated Organization/BreadcrumbList entries, at the
      // end of the page -- proves the parser's recursive @graph unwrapping finds the
      // Product entry even when it's not the only (or first) thing in the graph.
      return {
        label: "@graph-wrapped with extra entries, end of page",
        placement: "bottom",
        blocks: [
          {
            key: "graph",
            json: {
              "@context": "http://schema.org/",
              "@graph": [
                organizationJsonLd(),
                breadcrumbJsonLd(product),
                productJsonLd(product, urlAvailabilityForm),
              ],
            },
          },
        ],
      };
    case 3:
    default:
      // Two separate <script> tags -- one pure Organization block, one pure Product
      // block -- proves extra non-product ld+json tags elsewhere on the page don't
      // confuse the "first script tag that parses as product JSON" scan.
      return {
        label: "two separate script tags, end of page",
        placement: "bottom",
        blocks: [
          { key: "organization", json: organizationJsonLd() },
          { key: "product", json: productJsonLd(product, urlAvailabilityForm) },
        ],
      };
  }
}
