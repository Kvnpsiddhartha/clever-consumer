"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import type { TrackKey } from "@/lib/rotation";

/**
 * Admin-only: jumps a track straight to an exact variant, persisted to the shared
 * counter (POST /api/set-variant, gated by ADMIN_TOKEN when one is configured -- same
 * gate as the /admin page itself).
 */
export function VariantPinButtons({
  trackKey,
  numVariants,
  currentVariantIndex,
  adminToken,
}: {
  trackKey: TrackKey;
  numVariants: number;
  currentVariantIndex: number;
  adminToken?: string;
}) {
  const router = useRouter();
  const [pendingIndex, setPendingIndex] = useState<number | null>(null);

  async function pin(index: number) {
    setPendingIndex(index);
    try {
      await fetch("/api/set-variant", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(adminToken ? { "x-admin-token": adminToken } : {}),
        },
        body: JSON.stringify({ track: trackKey, index, numVariants }),
      });
      router.refresh();
    } finally {
      setPendingIndex(null);
    }
  }

  return (
    <>
      {Array.from({ length: numVariants }).map((_, i) => (
        <button
          type="button"
          key={i}
          onClick={() => pin(i)}
          disabled={pendingIndex !== null}
          className="variant-pin-btn"
          aria-current={i === currentVariantIndex}
        >
          {pendingIndex === i ? "…" : `[${i}]`}
        </button>
      ))}
    </>
  );
}
