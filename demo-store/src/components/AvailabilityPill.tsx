import type { Availability } from "@/lib/products";

export function AvailabilityPill({
  availability,
  label,
}: {
  availability: Availability;
  label?: string;
}) {
  const className = `pill pill--${availability.replace(/_/g, "-")}`;
  const fallback: Record<Availability, string> = {
    in_stock: "In Stock",
    out_of_stock: "Out of Stock",
    preorder: "Pre-Order",
    unknown: "Unknown",
  };
  return <span className={className}>{label ?? fallback[availability]}</span>;
}
