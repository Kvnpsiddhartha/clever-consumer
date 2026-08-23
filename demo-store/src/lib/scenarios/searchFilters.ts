export interface SearchStructureVariant {
  label: string;
  placement: "sidebar" | "topbar" | "drawer" | "chips";
  widget: "checkbox" | "select" | "listbox" | "toggle";
}

// A deliberately different kind of scenario than the category grid's: filter placement
// and facet widget markup rotate together, independent of the results list (which always
// renders in the dense "content-list" style so it visually reads as a different page).
export const SEARCH_STRUCTURE_VARIANTS: SearchStructureVariant[] = [
  { label: "Sidebar / checkboxes", placement: "sidebar", widget: "checkbox" },
  { label: "Top bar / dropdowns", placement: "topbar", widget: "select" },
  { label: "Drawer / custom listbox", placement: "drawer", widget: "listbox" },
  { label: "Chip row / toggle buttons", placement: "chips", widget: "toggle" },
];

export interface SearchLabelVariant {
  heading: string;
  sortLabel: string;
  availabilityLabels: {
    in_stock: string;
    out_of_stock: string;
    preorder: string;
    unknown: string;
  };
}

// Purely cosmetic text rotation -- proves the parser never reads visible label text, only
// the underlying JSON-LD/JSON fields.
export const SEARCH_LABEL_VARIANTS: SearchLabelVariant[] = [
  {
    heading: "Filter",
    sortLabel: "Sort by",
    availabilityLabels: {
      in_stock: "In Stock",
      out_of_stock: "Out of Stock",
      preorder: "Pre-Order",
      unknown: "Check availability",
    },
  },
  {
    heading: "Refine",
    sortLabel: "Order by",
    availabilityLabels: {
      in_stock: "Available Now",
      out_of_stock: "Sold Out",
      preorder: "Reserve Now",
      unknown: "Availability unknown",
    },
  },
  {
    heading: "Narrow Results",
    sortLabel: "Arrange",
    availabilityLabels: {
      in_stock: "Ready to Ship",
      out_of_stock: "Currently Unavailable",
      preorder: "Coming Soon",
      unknown: "Ask in store",
    },
  },
];
