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
  it("removes the redundant page hero and keeps the toolbar structure", () => {
    expect(groupsViewSource).not.toContain('data-test="admin-page-hero"');
    expect(groupsViewSource).not.toContain("admin-page-hero");
    expect(groupsViewSource).toContain("admin-toolbar");
    expect(groupsViewSource).toContain("admin-toolbar-group");
  });

  it("renders the empty response refund policy in both group forms", () => {
    expect(groupsViewSource).toContain('v-model="createForm.empty_response_compensation_enabled"');
    expect(groupsViewSource).toContain('v-model="editForm.empty_response_compensation_enabled"');
    expect(groupsViewSource).toContain("admin.groups.emptyResponseCompensation.hint");
  });

  it("keeps system custom group orchestration thin and visible in the group list", () => {
    expect(groupsViewSource).toContain('data-testid="system-custom-create"');
    expect(groupsViewSource).toContain("SystemCustomGroupDialog");
    expect(groupsViewSource).toContain('data-testid="system-custom-type-badge"');
    expect(groupsViewSource).toContain('data-testid="system-custom-manage"');
    expect(groupsViewSource).toContain("isSystemCustomGroup(row)");
    expect(groupsViewSource).toContain('@saved="handleSystemCustomSaved"');
    expect(groupsViewSource).toContain('@deleted="handleSystemCustomDeleted"');
    expect(groupsViewSource).toContain("await loadGroups()");
  });

  it("does not expose ordinary edit, composite route, or delete controls for system groups", () => {
    expect(groupsViewSource).toContain('v-if="!isSystemCustomGroup(row)"');
    expect(groupsViewSource).toContain(
      'row.platform === \'composite\' && !isSystemCustomGroup(row)',
    );
    expect(groupsViewSource).toContain("openSystemCustomGroup(row.id)");
  });
});
