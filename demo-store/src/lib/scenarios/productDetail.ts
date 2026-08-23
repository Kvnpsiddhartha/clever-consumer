export interface ProductLayoutVariant {
  label: string;
  galleryPosition: "left" | "top" | "right";
  descriptionStyle: "tabs" | "accordion" | "scroll";
}

// Pure DOM/layout rotation -- gallery position, description widget, implied wrapper
// depth/sibling order all shift, but JSON-LD underneath is always valid (see jsonld.ts).
export const PRODUCT_LAYOUT_VARIANTS: ProductLayoutVariant[] = [
  { label: "Gallery left / tabs", galleryPosition: "left", descriptionStyle: "tabs" },
  { label: "Gallery top / accordion", galleryPosition: "top", descriptionStyle: "accordion" },
  { label: "Gallery right / single scroll", galleryPosition: "right", descriptionStyle: "scroll" },
  { label: "Gallery top / accordion (alt order)", galleryPosition: "top", descriptionStyle: "tabs" },
];

export interface ProductLabelVariant {
  ctaText: string;
  pricePrefix: string;
  priceSuffix: string;
  stockLabels: {
    in_stock: string;
    out_of_stock: string;
    preorder: string;
    unknown: string;
  };
}

// Visible text-only rotation: CTA wording, price *display* format, and stock badge text
// all change independently of product_layout -- the underlying JSON-LD price/availability
// fields are untouched, this only proves visible copy is irrelevant to extraction.
export const PRODUCT_LABEL_VARIANTS: ProductLabelVariant[] = [
  {
    ctaText: "Add to Cart",
    pricePrefix: "₹",
    priceSuffix: "",
    stockLabels: {
      in_stock: "In Stock",
      out_of_stock: "Out of Stock",
      preorder: "Pre-Order",
      unknown: "Check availability",
    },
  },
  {
    ctaText: "Buy Now",
    pricePrefix: "Rs. ",
    priceSuffix: "",
    stockLabels: {
      in_stock: "Available Now",
      out_of_stock: "Sold Out",
      preorder: "Reserve Now",
      unknown: "Availability unknown",
    },
  },
  {
    ctaText: "Add to Bag",
    pricePrefix: "",
    priceSuffix: " INR",
    stockLabels: {
      in_stock: "Ready to Ship",
      out_of_stock: "Currently Unavailable",
      preorder: "Coming Soon",
      unknown: "Ask in store",
    },
  },
];
