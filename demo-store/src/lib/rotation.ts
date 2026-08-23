import { getPool } from "./db";
import { config } from "./config";

/**
 * One track per independently-rotating scenario. Each track has its own counter row in
 * demo_store.rotation_counters (see db/schema.sql) and its own ROTATE_EVERY_N cadence
 * (see config.ts) -- this is what lets, say, the category grid's selector/DOM structure
 * rotate on a different schedule than its visual style, or the product page's layout
 * rotate independently of where the JSON-LD block sits.
 */
export const TRACK_KEYS = [
  "category_structure",
  "category_style",
  "search_structure",
  "search_label",
  "product_layout",
  "product_jsonld_shape",
  "product_label",
] as const;

export type TrackKey = (typeof TRACK_KEYS)[number];

const ROTATE_EVERY_N: Record<TrackKey, number> = {
  category_structure: config.rotateEveryN.categoryStructure,
  category_style: config.rotateEveryN.categoryStyle,
  search_structure: config.rotateEveryN.searchStructure,
  search_label: config.rotateEveryN.searchLabel,
  product_layout: config.rotateEveryN.productLayout,
  product_jsonld_shape: config.rotateEveryN.productJsonldShape,
  product_label: config.rotateEveryN.productLabel,
};

/**
 * Atomically increments and returns a track's site-wide counter in one round trip. Safe
 * under concurrent hits (a presenter's browser and Bright Data's own fetch racing) since
 * Postgres -- not application code -- owns the atomicity.
 */
async function incrementAndGetCounter(trackKey: TrackKey): Promise<number> {
  const pool = getPool();
  const result = await pool.query<{ counter: string }>(
    `INSERT INTO demo_store.rotation_counters (track_key, counter, updated_at)
     VALUES ($1, 1, now())
     ON CONFLICT (track_key) DO UPDATE
       SET counter = demo_store.rotation_counters.counter + 1, updated_at = now()
     RETURNING counter`,
    [trackKey],
  );
  return Number(result.rows[0]?.counter ?? 1);
}

export type NextSearchParams = { [key: string]: string | string[] | undefined };

/**
 * Parses `?demoVariant=track:index[,track:index...]` (Next.js App Router's awaited
 * `searchParams` shape) into a per-track override map.
 */
export function parseDemoVariantOverrides(
  searchParams: NextSearchParams,
): Partial<Record<TrackKey, number>> {
  const value = searchParams.demoVariant;
  const raw = Array.isArray(value) ? value[0] : value;
  if (!raw) return {};
  const overrides: Partial<Record<TrackKey, number>> = {};
  for (const pair of raw.split(",")) {
    const [track, indexRaw] = pair.split(":");
    const index = Number.parseInt(indexRaw ?? "", 10);
    if (
      TRACK_KEYS.includes(track as TrackKey) &&
      Number.isFinite(index) &&
      index >= 0
    ) {
      overrides[track as TrackKey] = index;
    }
  }
  return overrides;
}

/**
 * Resolves which variant a track should render for this request: an explicit
 * `?demoVariant=` override wins (and never touches the stored counter, so natural
 * rotation resumes on the next un-parameterized load); otherwise the DB counter is
 * incremented and `floor(counter / rotateEveryN) mod numVariants` decides the variant.
 */
export async function resolveVariant(
  trackKey: TrackKey,
  numVariants: number,
  overrides: Partial<Record<TrackKey, number>>,
): Promise<{ variantIndex: number; counter: number | null; rotateEveryN: number }> {
  const rotateEveryN = ROTATE_EVERY_N[trackKey];
  const override = overrides[trackKey];
  if (override !== undefined) {
    return {
      variantIndex: override % numVariants,
      counter: null,
      rotateEveryN,
    };
  }
  const counter = await incrementAndGetCounter(trackKey);
  const variantIndex = Math.floor(counter / rotateEveryN) % numVariants;
  return { variantIndex, counter, rotateEveryN };
}

/** Read-only peek at every track's current counter, for the /admin panel. */
export async function getAllCounterStates(): Promise<
  { trackKey: TrackKey; counter: number; rotateEveryN: number }[]
> {
  const pool = getPool();
  const result = await pool.query<{ track_key: string; counter: string }>(
    `SELECT track_key, counter FROM demo_store.rotation_counters`,
  );
  const byKey = new Map(result.rows.map((row) => [row.track_key, Number(row.counter)]));
  return TRACK_KEYS.map((trackKey) => ({
    trackKey,
    counter: byKey.get(trackKey) ?? 0,
    rotateEveryN: ROTATE_EVERY_N[trackKey],
  }));
}

/** Resets every track's counter to 0 -- used by the /admin "reset demo" button. */
export async function resetAllCounters(): Promise<void> {
  const pool = getPool();
  await pool.query(
    `UPDATE demo_store.rotation_counters SET counter = 0, updated_at = now()`,
  );
}

/**
 * Persistently advances a track to its next variant -- unlike the `?demoVariant=`
 * override, this actually mutates the shared counter, so the change sticks for every
 * subsequent viewer (and Bright Data's next fetch), not just the one request. Jumps the
 * counter to the start of the next rotateEveryN-sized bucket in one atomic UPDATE, so it
 * always advances exactly one variant regardless of where in the current cycle the
 * counter happens to sit.
 */
export async function advanceTrack(trackKey: TrackKey): Promise<number> {
  const rotateEveryN = ROTATE_EVERY_N[trackKey];
  const pool = getPool();
  const result = await pool.query<{ counter: string }>(
    `INSERT INTO demo_store.rotation_counters (track_key, counter, updated_at)
     VALUES ($1, $2, now())
     ON CONFLICT (track_key) DO UPDATE
       SET counter = ((demo_store.rotation_counters.counter / $2) + 1) * $2, updated_at = now()
     RETURNING counter`,
    [trackKey, rotateEveryN],
  );
  return Number(result.rows[0]?.counter ?? rotateEveryN);
}

/**
 * Persistently pins a track to an exact variant index (used by /admin's per-variant
 * buttons). Also a real, sticky change to the shared counter, not a one-request preview.
 */
export async function setTrackVariant(
  trackKey: TrackKey,
  variantIndex: number,
  numVariants: number,
): Promise<number> {
  const rotateEveryN = ROTATE_EVERY_N[trackKey];
  const normalizedIndex = ((variantIndex % numVariants) + numVariants) % numVariants;
  const targetCounter = normalizedIndex * rotateEveryN;
  const pool = getPool();
  const result = await pool.query<{ counter: string }>(
    `INSERT INTO demo_store.rotation_counters (track_key, counter, updated_at)
     VALUES ($1, $2, now())
     ON CONFLICT (track_key) DO UPDATE
       SET counter = $2, updated_at = now()
     RETURNING counter`,
    [trackKey, targetCounter],
  );
  return Number(result.rows[0]?.counter ?? targetCounter);
}
