import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";
import { applyRoutingPreset, selectSubscriptionNodes, removeRoutingGroup } from "./subscription-routing.ts";
const catalog = JSON.parse(await readFile(new URL("../../../internal/subscription/routing-catalog.json", import.meta.url), "utf8"));
const streaming = catalog.presets.find(p => p.id === "streaming-v1");
const developer = catalog.presets.find(p => p.id === "developer-v1");
const input = { name: "Fixture", format: "stash", node_ids: ["a", "b"], rules: [{type:"domain",value:"example.com",target:"direct"}], final_action:"proxy", enabled:true };
test("template application keeps selected nodes and custom rules without sharing mutable preset state", () => {
 const next=applyRoutingPreset(input,streaming,true);
 assert.deepEqual(next.node_ids,["a","b"]);assert.equal(next.routing.groups[0].name,"Default proxy");assert.deepEqual(next.rules,input.rules);
 next.routing.groups[1].name="Changed";assert.equal(streaming.groups[1].name,"YouTube");
});
test("deselecting a node removes every group membership and preserves other selections",()=>{
 let next=applyRoutingPreset(input,streaming,false);
 next.routing.groups[1].node_ids=["a","b"];next.routing.groups[2].node_ids=["a"];
 next=selectSubscriptionNodes(next,["b"]);
 assert.deepEqual(next.routing.groups[1].node_ids,["b"]);assert.deepEqual(next.routing.groups[2].node_ids,[]);
});
test("removing a group repairs rules, source targets and catchall with no dangling reference",()=>{
 const preset=streaming;let next=applyRoutingPreset(input,preset,false);
 next.rules=[{type:"domain",value:"example.com",target:"group:youtube"}];next.routing.final="group:youtube";next.routing.targets.netflix="group:youtube";
 next=removeRoutingGroup(next,"youtube",preset);
 assert.equal(next.rules[0].target,"proxy");assert.equal(next.routing.final,"group:proxy");assert.equal(next.routing.targets.youtube,"group:proxy");assert.equal(next.routing.targets.netflix,"group:proxy");assert.equal(next.routing.groups.some(g=>g.id==="youtube"),false);
 assert.equal(removeRoutingGroup(next,"proxy",preset),next);
});
test("changing or removing templates repairs incompatible custom targets",()=>{
 let next=applyRoutingPreset(input,streaming,false);next.rules=[{type:"domain",value:"example.com",target:"group:youtube"}];
 next=applyRoutingPreset(next,developer,false);assert.equal(next.rules[0].target,"proxy");
 next=applyRoutingPreset(next,null,false);assert.equal(next.routing,null);assert.deepEqual(next.node_ids,["a","b"]);
});

test("group names reject reserved policies and case-insensitive collisions", async()=>{
 const {validRoutingNames}=await import('./subscription-routing.ts');
 let next=applyRoutingPreset(input,streaming,false);
 assert.equal(validRoutingNames(next),true);
 next.routing.groups[1].name='GLOBAL';assert.equal(validRoutingNames(next),false);
 next.routing.groups[1].name='Netflix';next.routing.groups[2].name='netflix';assert.equal(validRoutingNames(next),false);
});


test("combined preset supports development and movies together while preserving existing group assignments",()=>{
 let next=applyRoutingPreset(input,developer,false);
 next.routing.groups.find(g=>g.id==="ai").node_ids=["a"];
 const preset=catalog.presets.find(p=>p.id==="allround-v1");
 next=applyRoutingPreset(next,preset,false);
 assert.equal(preset.recommended,true);
 assert.deepEqual(next.routing.groups.find(g=>g.id==="ai").node_ids,["a"]);
 for(const id of ["ai","developer","media","social","games","work"]) assert.ok(next.routing.groups.some(g=>g.id===id));
 assert.equal(preset.bindings.find(b=>b.source==="github").target,"group:developer");
 assert.equal(preset.bindings.find(b=>b.source==="netflix").target,"group:media");
 assert.equal(next.routing.final,"group:proxy");
});

test("direct-first is explicit and does not become a proxy catchall",()=>{
 const preset=catalog.presets.find(p=>p.id==="direct-first-v1");
 const next=applyRoutingPreset(input,preset,true);
 assert.equal(next.routing.final,"direct");
 assert.match(preset.description_en,/Unmatched traffic connects directly/);
});

test("all preset groups apply shipped default icons and retain custom icons", async () => {
 const { access } = await import("node:fs/promises");
 for (const preset of catalog.presets) {
  const next = applyRoutingPreset(input, preset, false);
  for (const group of next.routing.groups) {
   assert.match(group.icon, /^https:\/\/xingdu\.app\/subscription-icons\/v2\/[a-z-]+\.png$/);
   await access(new URL(`../public/subscription-icons/v2/${group.id}.png`, import.meta.url));
  }
 }
 let next = applyRoutingPreset(input, streaming, false);
 next.routing.groups[0].icon = "https://assets.example.com/custom.png";
 next.routing.groups[1].icon = undefined;
 next = applyRoutingPreset(next, streaming, true);
 assert.equal(next.routing.groups[0].icon, "https://assets.example.com/custom.png");
 assert.equal(next.routing.groups[1].icon, streaming.groups[1].icon);
 assert.equal(streaming.groups[0].icon, "https://xingdu.app/subscription-icons/v2/proxy.png");
});


test("reapplying presets upgrades shipped v1 icons without changing custom URLs", () => {
 let next = applyRoutingPreset(input, streaming, false);
 next.routing.groups[0].icon = "https://xingdu.app/subscription-icons/v1/proxy.png";
 next.routing.groups[1].icon = "https://assets.example.com/youtube.png";
 next = applyRoutingPreset(next, streaming, false);
 assert.equal(next.routing.groups[0].icon, "https://xingdu.app/subscription-icons/v2/proxy.png");
 assert.equal(next.routing.groups[1].icon, "https://assets.example.com/youtube.png");
});
