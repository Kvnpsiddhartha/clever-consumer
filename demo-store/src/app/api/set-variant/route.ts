import { NextRequest, NextResponse } from "next/server";
import { setTrackVariant, TRACK_KEYS, TrackKey } from "@/lib/rotation";
import { config } from "@/lib/config";

// Admin-only (same gate as /admin itself): jumps a track straight to an exact variant,
// persisted to the shared counter. Used by /admin's per-variant pin buttons.
export async function POST(request: NextRequest) {
  if (config.adminToken) {
    const token =
      request.headers.get("x-admin-token") ?? new URL(request.url).searchParams.get("token");
    if (token !== config.adminToken) {
      return NextResponse.json({ error: "unauthorized" }, { status: 401 });
    }
  }
  const body = await request.json().catch(() => ({}));
  const track = body.track as TrackKey;
  const index = Number(body.index);
  const numVariants = Number(body.numVariants);
  if (!TRACK_KEYS.includes(track) || !Number.isFinite(index) || !Number.isFinite(numVariants) || numVariants <= 0) {
    return NextResponse.json({ error: "invalid request" }, { status: 400 });
  }
  const counter = await setTrackVariant(track, index, numVariants);
  return NextResponse.json({ counter });
}
