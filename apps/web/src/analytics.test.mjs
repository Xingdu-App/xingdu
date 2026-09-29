import test from "node:test";
import assert from "node:assert/strict";
import {
  analyticsAISource,
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

test("AI source tags are allowlisted and removed from page URLs", () => {
  const href = "https://example.com/docs/api?utm_source=chatgpt.com";
  assert.equal(
    analyticsPage(href, ["/docs/api"]),
    "https://example.com/docs/api",
  );
  assert.equal(analyticsAISource(href, ""), "chatgpt.com");
  assert.equal(
    analyticsAISource(
      "https://example.com/",
      "https://www.perplexity.ai/search/private?token=secret",
    ),
    "perplexity.ai",
  );
  assert.equal(
    analyticsAISource("https://example.com/", "https://chatgpt.com.evil.test/"),
    undefined,
  );
  for (const suffix of [
    "&token=secret",
    "&utm_source=secret",
    "#invite=secret",
  ]) {
    assert.equal(analyticsPage(href + suffix, ["/docs/api"]), null);
    assert.equal(analyticsAISource(href + suffix, ""), undefined);
  }
  assert.equal(
    analyticsPage("https://example.com/app?utm_source=chatgpt.com", [
      "/docs/api",
    ]),
    null,
  );
});
