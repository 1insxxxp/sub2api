import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

const currentDir = dirname(fileURLToPath(import.meta.url));
const groupsViewSource = readFileSync(
  resolve(currentDir, "../GroupsView.vue"),
  "utf8",
);

describe("admin GroupsView layout", () => {
  it("uses the shared admin page hero and toolbar structure", () => {
    expect(groupsViewSource).toContain('data-test="admin-page-hero"');
    expect(groupsViewSource).toContain("admin-page-hero");
    expect(groupsViewSource).toContain("admin-page-meta-chip");
    expect(groupsViewSource).toContain("admin-toolbar");
    expect(groupsViewSource).toContain("admin-toolbar-group");
  });
});
