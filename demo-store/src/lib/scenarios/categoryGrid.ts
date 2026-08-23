export interface CategoryStructureVariant {
  label: string;
  wrapperClass: string;
  titleClass: string;
  priceClass: string;
  depth: "flat" | "nested";
  order: Array<"image" | "name" | "price" | "cta">;
}

// Selector/class-name rename + DOM wrapper depth + sibling order, bundled per variant so
// each rotation is a genuinely distinct combination. None of this affects scrapability --
// the real backend never fetches this page directly, it's purely a visible DOM-resilience
// proof for the live audience.
export const CATEGORY_STRUCTURE_VARIANTS: CategoryStructureVariant[] = [
  {
    label: ".product-card / flat / image-name-price-cta",
    wrapperClass: "product-card",
    titleClass: "card-title",
    priceClass: "card-price",
    depth: "flat",
    order: ["image", "name", "price", "cta"],
  },
  {
    label: ".item-tile / nested / name-image-cta-price",
    wrapperClass: "item-tile",
    titleClass: "tile-name",
    priceClass: "tile-cost",
    depth: "nested",
    order: ["name", "image", "cta", "price"],
  },
  {
    label: ".card-product / flat / image-price-name-cta",
    wrapperClass: "card-product",
    titleClass: "product-title",
    priceClass: "product-amount",
    depth: "flat",
    order: ["image", "price", "name", "cta"],
  },
  {
    label: ".pc__wrap (BEM) / nested / price-image-name-cta",
    wrapperClass: "pc__wrap",
    titleClass: "pc__name",
    priceClass: "pc__price",
    depth: "nested",
    order: ["price", "image", "name", "cta"],
  },
];

export type CategoryStyle = "grid-card" | "list-row" | "content-list" | "image-only" | "image-text";

export const CATEGORY_STYLE_VARIANTS: { style: CategoryStyle; label: string }[] = [
  { style: "grid-card", label: "Grid cards" },
  { style: "list-row", label: "List rows" },
  { style: "content-list", label: "Dense content list" },
  { style: "image-only", label: "Image-only masonry" },
  { style: "image-text", label: "Image + text minimal" },
];
