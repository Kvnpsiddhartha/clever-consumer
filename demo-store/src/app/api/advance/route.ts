import { NextRequest, NextResponse } from "next/server";
import { advanceTrack, TRACK_KEYS, TrackKey } from "@/lib/rotation";

// Intentionally NOT gated by ADMIN_TOKEN: this is the persistent "change layout now"
// button shown on the public-facing pages (home/search/product), meant for anyone
// running the live demo to use, at the same trust level as a normal page load (which
// already advances the counter by 1 on its own). Only /admin and /api/reset are gated.
export async function POST(request: NextRequest) {
  const body = await request.json().catch(() => ({}));
  const track = body.track as TrackKey;
  if (!TRACK_KEYS.includes(track)) {
    return NextResponse.json({ error: "invalid track" }, { status: 400 });
  }
  const counter = await advanceTrack(track);
  return NextResponse.json({ counter });
}
