import assert from "node:assert/strict";
import test from "node:test";
import {
  subscriptionImportURL,
  copySubscriptionURL,
} from "./subscription-access.ts";
test("import embeds the entire private URL without losing format or token", () => {
  const url =
    "https://control.example.invalid/api/v1/subscriptions/sub_fixture/content?token=private&format=stash";
  for (const [format, prefix] of [
    ["stash", "stash://install-config?url="],
    ["surge", "surge:///install-config?url="],
  ]) {
    const link = subscriptionImportURL(format, url);
    assert.ok(link.startsWith(prefix));
    assert.equal(decodeURIComponent(link.slice(prefix.length)), url);
  }
  assert.equal(subscriptionImportURL("loon", url), null);
  assert.equal(subscriptionImportURL("mihomo", url), null);
  assert.equal(subscriptionImportURL("stash", "javascript:alert(1)"), null);
});
test("clipboard denial uses a temporary text field and cleans it up", async () => {
  const nav = Object.getOwnPropertyDescriptor(globalThis, "navigator");
  const doc = Object.getOwnPropertyDescriptor(globalThis, "document");
  let removed = false,
    focused = false,
    selected = false;
  const field = {
    value: "",
    style: { cssText: "" },
    select() {
      selected = true;
    },
    remove() {
      removed = true;
    },
  };
  Object.defineProperty(globalThis, "navigator", {
    configurable: true,
    value: {
      clipboard: {
        async writeText() {
          throw Error("denied");
        },
      },
    },
  });
  Object.defineProperty(globalThis, "document", {
    configurable: true,
    value: {
      activeElement: {
        focus() {
          focused = true;
        },
      },
      createElement() {
        return field;
      },
      body: { appendChild() {} },
      execCommand(action) {
        assert.equal(action, "copy");
        return selected;
      },
    },
  });
  try {
    assert.equal(await copySubscriptionURL("private-url"), true);
    assert.equal(field.value, "private-url");
    assert.ok(removed && focused);
  } finally {
    if (nav) Object.defineProperty(globalThis, "navigator", nav);
    else delete globalThis.navigator;
    if (doc) Object.defineProperty(globalThis, "document", doc);
    else delete globalThis.document;
  }
});
