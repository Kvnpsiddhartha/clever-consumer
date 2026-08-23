function intEnv(key: string, fallback: number): number {
  const raw = process.env[key];
  if (!raw) return fallback;
  const value = Number.parseInt(raw, 10);
  return Number.isFinite(value) && value > 0 ? value : fallback;
}

export const config = {
  databaseUrl: process.env.DATABASE_URL ?? "",
  defaultCurrency: process.env.DEFAULT_CURRENCY ?? "INR",
  storeBrandName: process.env.STORE_BRAND_NAME ?? "Stridex Market",
  adminToken: process.env.ADMIN_TOKEN ?? "",

  rotateEveryN: {
    default: intEnv("ROTATE_EVERY_N", 4),
    categoryStructure: intEnv(
      "ROTATE_EVERY_N_CATEGORY_STRUCTURE",
      intEnv("ROTATE_EVERY_N", 4),
    ),
    categoryStyle: intEnv(
      "ROTATE_EVERY_N_CATEGORY_STYLE",
      intEnv("ROTATE_EVERY_N", 4),
    ),
    searchStructure: intEnv(
      "ROTATE_EVERY_N_SEARCH_STRUCTURE",
      intEnv("ROTATE_EVERY_N", 4),
    ),
    searchLabel: intEnv(
      "ROTATE_EVERY_N_SEARCH_LABEL",
      intEnv("ROTATE_EVERY_N", 4),
    ),
    productLayout: intEnv(
      "ROTATE_EVERY_N_PRODUCT_LAYOUT",
      intEnv("ROTATE_EVERY_N", 4),
    ),
    productJsonldShape: intEnv(
      "ROTATE_EVERY_N_PRODUCT_JSONLD_SHAPE",
      intEnv("ROTATE_EVERY_N", 4),
    ),
    productLabel: intEnv(
      "ROTATE_EVERY_N_PRODUCT_LABEL",
      intEnv("ROTATE_EVERY_N", 4),
    ),
  },
};
