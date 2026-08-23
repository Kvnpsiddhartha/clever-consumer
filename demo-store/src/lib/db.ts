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

export function getPool(): Pool {
  if (!config.databaseUrl) {
    throw new Error(
      "DATABASE_URL is not set -- demo-store needs the same Neon Postgres instance the main clever-consumer app uses (see demo-store/.env.example).",
    );
  }
  if (!globalThis.__demoStorePgPool) {
    globalThis.__demoStorePgPool = new Pool({
      connectionString: config.databaseUrl,
      ssl: config.databaseUrl.includes("sslmode=disable")
        ? false
        : { rejectUnauthorized: false },
      max: 5,
    });
  }
  return globalThis.__demoStorePgPool;
}
