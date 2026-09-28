import assert from "node:assert/strict";
import test from "node:test";
import {
  organizationURL,
  detailURL,
  detailFromURL,
  clearDetail,
} from "./navigation.ts";

test("switching organization clears stale resource details while preserving unrelated state", () => {
  const before = new URL(
    "https://xingdu.example/app/nodes?organization=old&node=old-node&host=old-host&view=protocols&filter=online#invite=token",
  );
  const next = organizationURL(before, "new");
  assert.equal(next.searchParams.get("organization"), "new");
  assert.deepEqual(detailFromURL(next), {
    node: "",
    host: "",
    protocols: false,
  });
  assert.equal(next.searchParams.get("filter"), "online");
  assert.equal(next.hash, "#invite=token");
  assert.equal(before.searchParams.get("organization"), "old");
});
test("initial organization normalization preserves direct detail links", () => {
  const url = organizationURL(
    new URL("https://xingdu.example/app/hosts?host=machine"),
    "org",
    false,
  );
  assert.equal(url.searchParams.get("host"), "machine");
  assert.equal(url.searchParams.get("organization"), "org");
});
test("node and server detail links round trip and replace each other", () => {
  const base = new URL("https://xingdu.example/app/nodes?organization=org");
  const node = detailURL(base, { node: "node-1" });
  assert.deepEqual(detailFromURL(new URL(node.href)), {
    node: "node-1",
    host: "",
    protocols: true,
  });
  const host = detailURL(node, { host: "host-1" });
  assert.deepEqual(detailFromURL(host), {
    node: "",
    host: "host-1",
    protocols: false,
  });
  const protocols = detailURL(host, { host: "host-1", protocols: true });
  assert.deepEqual(detailFromURL(protocols), {
    node: "",
    host: "host-1",
    protocols: true,
  });
  assert.equal(detailURL(protocols).href, base.href);
  assert.equal(
    clearDetail(new URL(protocols)).searchParams.get("organization"),
    "org",
  );
});

test("typed resource IDs remain opaque across organization and detail navigation", () => {
  const org = "org_1234567890abcdef1234567890abcdef";
  const server = "srv_0123456789abcdef0123456789abcdef";
  const node = "node_fedcba0987654321fedcba0987654321";
  const selected = organizationURL(
    new URL("https://xingdu.example/app/nodes"),
    org,
  );
  const detail = detailURL(selected, { node });
  const refreshed = new URL(detail.href);
  assert.equal(refreshed.searchParams.get("organization"), org);
  assert.equal(detailFromURL(refreshed).node, node);
  const host = detailURL(refreshed, { host: server, protocols: true });
  assert.deepEqual(detailFromURL(new URL(host.href)), {
    node: "",
    host: server,
    protocols: true,
  });
  assert.equal(host.searchParams.get("organization"), org);
});
