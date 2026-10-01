import test from "node:test";
import assert from "node:assert/strict";
import {
  consoleAnalyticsLocation,
  interactionAction,
} from "./interaction-analytics.ts";

test("console routes strip all tenant and detail values from analytics", () => {
  assert.deepEqual(
    consoleAnalyticsLocation(
      "https://example.com/app/hosts?organization=private-org&host=private-address",
    ),
    { page: "hosts", location: "https://example.com/app/hosts" },
  );
  assert.deepEqual(
    consoleAnalyticsLocation(
      "https://example.com/app/?page=billing&organization=secret",
    ),
    { page: "billing", location: "https://example.com/app/billing" },
  );
  for (const href of [
    "https://example.com/app?invite=secret",
    "https://example.com/app#invite=secret",
    "https://example.com/app?code=secret",
    "https://example.com/app/auth/callback",
    "https://example.com/api/v1/hosts",
    "https://example.com/app/secret",
    "invalid",
  ])
    assert.equal(consoleAnalyticsLocation(href), null, href);
});
test("clicks send fixed actions and never arbitrary control content", () => {
  assert.equal(interactionAction("修改权限", "button"), "edit_permissions");
  assert.equal(
    interactionAction(" Edit permissions ", "button"),
    "edit_permissions",
  );
  assert.equal(
    interactionAction("Save host", "button", { 保存服务器: "Save host" }),
    "save",
  );
  for (const label of [
    "User private-name",
    "xd_key_secret",
    "https://example.com/subscription/secret",
    "203.0.113.123",
    "private@example.com",
  ])
    assert.equal(interactionAction(label, "button"), "button");
  assert.equal(interactionAction("private value", "option"), "option");
  assert.equal(interactionAction("Save", "text-input"), null);
});
