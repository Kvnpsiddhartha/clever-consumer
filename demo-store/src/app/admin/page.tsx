import { getAllCounterStates, TrackKey } from "@/lib/rotation";
import { config } from "@/lib/config";
import { CATEGORY_STRUCTURE_VARIANTS, CATEGORY_STYLE_VARIANTS } from "@/lib/scenarios/categoryGrid";
import { SEARCH_STRUCTURE_VARIANTS, SEARCH_LABEL_VARIANTS } from "@/lib/scenarios/searchFilters";
import { PRODUCT_LAYOUT_VARIANTS, PRODUCT_LABEL_VARIANTS } from "@/lib/scenarios/productDetail";
import { NUM_JSONLD_SHAPES } from "@/lib/jsonld";
import { getAllProducts } from "@/lib/products";
import { VariantPinButtons } from "@/components/VariantPinButtons";
import { VariantCycleButton } from "@/components/VariantCycleButton";

export const dynamic = "force-dynamic";

const NUM_VARIANTS: Record<TrackKey, number> = {
  category_structure: CATEGORY_STRUCTURE_VARIANTS.length,
  category_style: CATEGORY_STYLE_VARIANTS.length,
  search_structure: SEARCH_STRUCTURE_VARIANTS.length,
  search_label: SEARCH_LABEL_VARIANTS.length,
  product_layout: PRODUCT_LAYOUT_VARIANTS.length,
  product_jsonld_shape: NUM_JSONLD_SHAPES,
  product_label: PRODUCT_LABEL_VARIANTS.length,
};

export default async function AdminPage({
  searchParams,
}: {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}) {
  const params = await searchParams;
  const token = Array.isArray(params.token) ? params.token[0] : params.token;
  if (config.adminToken && token !== config.adminToken) {
    return (
      <main className="page-shell">
        <h1 className="page-title">Admin</h1>
        <p>Add <code>?token=&lt;ADMIN_TOKEN&gt;</code> to the URL to view this panel.</p>
      </main>
    );
  }

  const states = await getAllCounterStates();
  const firstProductSlug = getAllProducts()[0]?.slug ?? "";

  return (
    <main className="page-shell">
      <h1 className="page-title">Demo control panel</h1>
      <p style={{ color: "#6b6b6b", fontSize: "0.9rem" }}>
        Every track's counter is global and site-wide -- it advances on every real request
        (yours, a judge's, or Bright Data's own fetch), not per-visitor. The buttons below
        make a real, persistent change to that shared counter (not a one-request preview)
        -- "Next" jumps forward one variant, the numbered buttons jump straight to one.
      </p>
      <table className="admin-table">
        <thead>
          <tr>
            <th>Track</th>
            <th>Rotate every</th>
            <th>Counter</th>
            <th>Current variant</th>
            <th>Next</th>
            <th>Pin a variant</th>
          </tr>
        </thead>
        <tbody>
          {states.map(({ trackKey, counter, rotateEveryN }) => {
            const numVariants = NUM_VARIANTS[trackKey];
            const variantIndex = Math.floor(counter / rotateEveryN) % numVariants;
            return (
              <tr key={trackKey}>
                <td>{trackKey}</td>
                <td>{rotateEveryN}</td>
                <td>{counter}</td>
                <td>{variantIndex}</td>
                <td>
                  <VariantCycleButton trackKey={trackKey} label="Next" />
                </td>
                <td>
                  <VariantPinButtons
                    trackKey={trackKey}
                    numVariants={numVariants}
                    currentVariantIndex={variantIndex}
                    adminToken={config.adminToken ? token : undefined}
                  />
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
      <div className="admin-actions">
        <form action="/api/reset" method="post">
          {config.adminToken && <input type="hidden" name="token" value={token ?? ""} />}
          <button type="submit">Reset all counters</button>
        </form>
        <a href="/">View homepage</a>
        <a href={`/product/${firstProductSlug}`}>View a product page</a>
      </div>
    </main>
  );
}
