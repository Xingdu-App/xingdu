import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";
import { agentVersionAtLeast } from "./agent-version.ts";
const cases = JSON.parse(
  await readFile(
    new URL(
      "../../../internal/machine/testdata/versions.json",
      import.meta.url,
    ),
    "utf8",
  ),
);
for (const { actual, minimum, want } of cases) {
  test(`Agent ${actual} meets minimum ${minimum}: ${want}`, () => {
    assert.equal(agentVersionAtLeast(actual, minimum), want);
  });
}
test("missing API version requirements cannot authorize deployment", () => {
  assert.equal(agentVersionAtLeast("0.8.0", undefined), false);
  assert.equal(agentVersionAtLeast(undefined, "0.7.0-dev"), false);
});
