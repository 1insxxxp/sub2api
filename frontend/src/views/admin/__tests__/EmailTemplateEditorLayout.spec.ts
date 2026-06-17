import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

const currentDir = dirname(fileURLToPath(import.meta.url));
const editorSource = readFileSync(
  resolve(currentDir, "../settings/EmailTemplateEditor.vue"),
  "utf8",
);

describe("admin EmailTemplateEditor layout", () => {
  it("uses shared admin surfaces instead of legacy gray panels", () => {
    expect(editorSource).not.toMatch(/rounded-lg border border-gray-200 bg-gray-50/);
    expect(editorSource).not.toMatch(/rounded-lg border border-gray-200 bg-white/);
    expect(editorSource).not.toContain("rounded-full border border-gray-200 bg-white");
    expect(editorSource).not.toContain("h-[36rem] w-full rounded-md border border-gray-200 bg-white");
    expect(editorSource).toContain("admin-form-section");
    expect(editorSource).toContain("admin-surface");
    expect(editorSource).toContain("admin-choice-card");
  });
});
