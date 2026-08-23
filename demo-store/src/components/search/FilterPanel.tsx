import type { SearchStructureVariant, SearchLabelVariant } from "@/lib/scenarios/searchFilters";
import { CATEGORIES, categoryLabel } from "@/lib/products";

/**
 * Renders the same set of category facets using four different widget markups
 * (checkboxes / native select / a custom listbox / toggle buttons) and four different
 * placements (sidebar / top bar / drawer / chip row) -- a different kind of DOM-churn
 * scenario than the category grid's selector renaming.
 */
export function FilterPanel({
  structure,
  labels,
}: {
  structure: SearchStructureVariant;
  labels: SearchLabelVariant;
}) {
  return (
    <aside className="filter-panel">
      <h3>{labels.heading}</h3>
      {structure.widget === "checkbox" && (
        <ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
          {CATEGORIES.map((c) => (
            <li key={c} style={{ marginBottom: 6 }}>
              <label>
                <input type="checkbox" /> {categoryLabel(c)}
              </label>
            </li>
          ))}
        </ul>
      )}
      {structure.widget === "select" && (
        <select defaultValue="">
          <option value="" disabled>
            {categoryLabel(CATEGORIES[0])}
          </option>
          {CATEGORIES.map((c) => (
            <option key={c} value={c}>
              {categoryLabel(c)}
            </option>
          ))}
        </select>
      )}
      {structure.widget === "listbox" && (
        <div role="listbox" aria-label={labels.heading}>
          {CATEGORIES.map((c) => (
            <div role="option" aria-selected="false" key={c} style={{ padding: "4px 0" }}>
              {categoryLabel(c)}
            </div>
          ))}
        </div>
      )}
      {structure.widget === "toggle" &&
        CATEGORIES.map((c) => (
          <button type="button" className="filter-chip" key={c}>
            {categoryLabel(c)}
          </button>
        ))}
    </aside>
  );
}
