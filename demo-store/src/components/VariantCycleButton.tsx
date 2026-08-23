"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import type { TrackKey } from "@/lib/rotation";

/**
 * A visible "change this layout now" button, for the live demo: instead of reloading the
 * page ROTATE_EVERY_N times to see the next variant, click once. Persistently advances
 * the track's shared DB counter (POST /api/advance) -- this is a real, sticky change:
 * every subsequent viewer, and Bright Data's next fetch, sees the new variant too, not
 * just the browser that clicked. Then refreshes the current Server Component render to
 * reflect it.
 */
export function VariantCycleButton({
  trackKey,
  label,
}: {
  trackKey: TrackKey;
  label?: string;
}) {
  const router = useRouter();
  const [pending, setPending] = useState(false);

  async function handleClick() {
    setPending(true);
    try {
      await fetch("/api/advance", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ track: trackKey }),
      });
      router.refresh();
    } finally {
      setPending(false);
    }
  }

  return (
    <button
      type="button"
      className="variant-cycle-btn"
      onClick={handleClick}
      disabled={pending}
    >
      {pending ? "…" : (label ?? "Change layout")} →
    </button>
  );
}
