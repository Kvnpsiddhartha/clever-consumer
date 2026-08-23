import type { JsonLdShape } from "@/lib/jsonld";

// Standard guard against a stray "</script>" inside stringified JSON prematurely closing
// the tag in the raw HTML.
function safeJson(value: unknown): string {
  return JSON.stringify(value).replace(/</g, "\\u003c");
}

export function JsonLdScripts({ shape }: { shape: JsonLdShape }) {
  return (
    <>
      {shape.blocks.map((block) => (
        <script
          key={block.key}
          type="application/ld+json"
          // eslint-disable-next-line react/no-danger
          dangerouslySetInnerHTML={{ __html: safeJson(block.json) }}
        />
      ))}
    </>
  );
}
