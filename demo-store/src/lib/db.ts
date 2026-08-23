import { Pool } from "pg";
import { config } from "./config";

// Render's demo-store service is a long-lived Node process (not edge/serverless-per-
// request), so a plain module-scoped connection pool is the right fit -- no special
// serverless HTTP driver needed. `globalThis` caching avoids creating a fresh pool on
// every hot-reload in dev.
declare global {
  // eslint-disable-next-line no-var
  var __demoStorePgPool: Pool | undefined;
}

// Recent pg-connection-string versions treat a `sslmode=` query param baked into the
// connection string itself (e.g. Supabase's pooler URL) as authoritative and IGNORE the
// `ssl` option passed to `new Pool(...)` -- specifically, `sslmode=require` is now
// treated as an alias for `verify-full`, which does full certificate-chain verification
// and fails against Supabase's pooler cert with "self-signed certificate in certificate
// chain". Stripping `sslmode` from the URL makes our explicit `ssl` object below
// authoritative instead.
function stripSslMode(raw: string): { url: string; hadSslMode: boolean } {
  try {
    const url = new URL(raw.replace(/^postgres:/, "postgresql:"));
    const hadSslMode = url.searchParams.has("sslmode");
    const disabled = url.searchParams.get("sslmode") === "disable";
    url.searchParams.delete("sslmode");
    return { url: url.toString(), hadSslMode: hadSslMode && !disabled };
  } catch {
    return { url: raw, hadSslMode: !raw.includes("sslmode=disable") };
  }
}

export function getPool(): Pool {
  if (!config.databaseUrl) {
    throw new Error(
      "DATABASE_URL is not set -- demo-store needs the same Postgres instance the main clever-consumer app uses (see demo-store/.env.example).",
    );
  }
  if (!globalThis.__demoStorePgPool) {
    const { url, hadSslMode } = stripSslMode(config.databaseUrl);
    globalThis.__demoStorePgPool = new Pool({
      connectionString: url,
      // Encrypted but not strictly certificate-chain-verified -- fine for this
      // read/write-a-counter demo table; swap in verify-full + a CA bundle if this
      // ever needs to be stricter.
      ssl: hadSslMode ? { rejectUnauthorized: false } : false,
      max: 5,
    });
  }
  return globalThis.__demoStorePgPool;
}
