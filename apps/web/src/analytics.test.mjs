import test from "node:test";
import assert from "node:assert/strict";
import {
  analyticsID,
  analyticsPage,
  analyticsReferrer,
  analyticsTarget,
} from "./analytics.ts";
test("only clean allowlisted public pages can load analytics", () => {
  for (const path of [
    "/app",
    "/api/subscriptions/token",
    "/?invite=secret",
    "/#invite=secret",
    "/?utm_source=secret",
    "/unknown",
  ])
    assert.equal(
      analyticsPage(`https://example.com${path}`, ["/", "/pricing"]),
      null,
    );
  assert.equal(
    analyticsPage("https://example.com/pricing", ["/pricing"]),
    "https://example.com/pricing",
  );
});
test("only measurement IDs, referrer origins and fixed CTA labels are accepted", () => {
  assert.equal(analyticsID("G-ABC123"), "G-ABC123");
  for (const id of [undefined, "", "UA-123", "G-X&secret=123"])
    assert.equal(analyticsID(id), null);
  assert.equal(
    analyticsReferrer("https://example.com/private?token=secret#secret"),
    "https://example.com",
  );
  assert.equal(analyticsReferrer(""), "");
  assert.equal(analyticsTarget("/app", "https://example.com"), "console");
  assert.equal(
    analyticsTarget("/app?invite=secret", "https://example.com"),
    null,
  );
  assert.equal(
    analyticsTarget("https://other.example/app", "https://example.com"),
    null,
  );
});
