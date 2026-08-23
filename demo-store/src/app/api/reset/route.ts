import { NextRequest, NextResponse } from "next/server";
import { resetAllCounters } from "@/lib/rotation";
import { config } from "@/lib/config";

export async function POST(request: NextRequest) {
  if (config.adminToken) {
    const contentType = request.headers.get("content-type") ?? "";
    let token = new URL(request.url).searchParams.get("token");
    if (!token && contentType.includes("form")) {
      const form = await request.formData();
      token = (form.get("token") as string | null) ?? null;
    }
    if (token !== config.adminToken) {
      return NextResponse.json({ error: "unauthorized" }, { status: 401 });
    }
  }
  await resetAllCounters();
  const redirectTo = new URL("/admin", request.url);
  if (config.adminToken) redirectTo.searchParams.set("token", config.adminToken);
  return NextResponse.redirect(redirectTo, { status: 303 });
}
