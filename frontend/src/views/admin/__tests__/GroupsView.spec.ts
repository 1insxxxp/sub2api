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

  it("keeps the neutral workspace surface, wrapping error content, and touch sized actions contract", () => {
    expect(groupsViewSource).toContain("admin-workbench-page");
    expect(groupsViewSource).toContain("<AdminListToolbar");
    expect(groupsViewSource).toContain("background: var(--workspace-surface)");
    expect(groupsViewSource).toContain("overflow-wrap: anywhere");
    expect(groupsViewSource).toContain("min-height: 2.75rem");
  });

  it("keeps narrow mobile group actions aligned in fixed utility and create rows", () => {
    expect(groupsViewSource).toContain("grid-template-columns: repeat(4, minmax(0, 1fr));");
    expect(groupsViewSource).toContain("grid-column: 1 / -1;");
    expect(groupsViewSource).toContain("grid-template-columns: repeat(2, minmax(0, 1fr));");
    expect(groupsViewSource).toContain(".groups-list-actions .groups-tool-button {\n    width: 100%;");
  });

  it("keeps compact group metadata in one mobile row while usage stays full width", () => {
    expect(groupsViewSource).toContain(".groups-workbench :deep(.admin-record-summary) {");
    expect(groupsViewSource).toContain("grid-template-columns: repeat(3, minmax(0, 1fr));");
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
    expect(groupsViewSource).toMatch(
      /row.platform === 'composite' &&\s+!isSystemCustomGroup\(row\)/,
    );
    expect(groupsViewSource).toContain("openSystemCustomGroup(row.id)");
  });

  it("checks selected models for pricing and blocks incomplete group saves", () => {
    expect(groupsViewSource).toContain("previewPricingCoverage");
    expect(groupsViewSource).toContain('data-testid="create-pricing-coverage"');
    expect(groupsViewSource).toContain('data-testid="edit-pricing-coverage"');
    expect(groupsViewSource).toContain('await ensurePricingCoverage("create")');
    expect(groupsViewSource).toContain('await ensurePricingCoverage("edit")');
    expect(groupsViewSource).toContain(":required-models");
  });

  it("loads reusable custom tags from all groups and passes them to every group form", () => {
    expect(groupsViewSource).toContain("buildReusableGroupTagOptions");
    expect(groupsViewSource).toContain("getAllIncludingInactive");
    expect(groupsViewSource).toContain("const refreshGroupData");
    expect(groupsViewSource).toContain(':reusable-tags="reusableGroupTags"');
    expect(groupsViewSource).toContain("void loadReusableGroupTags()");
  });
});
