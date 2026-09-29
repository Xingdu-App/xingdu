import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";
import { applyRoutingPreset, selectSubscriptionNodes, removeRoutingGroup } from "./subscription-routing.ts";
const catalog = JSON.parse(await readFile(new URL("../../../internal/subscription/routing-catalog.json", import.meta.url), "utf8"));
const input = { name: "Fixture", format: "stash", node_ids: ["a", "b"], rules: [{type:"domain",value:"example.com",target:"direct"}], final_action:"proxy", enabled:true };
test("template application keeps selected nodes and custom rules without sharing mutable preset state", () => {
 const next=applyRoutingPreset(input,catalog.presets[1],true);
 assert.deepEqual(next.node_ids,["a","b"]);assert.equal(next.routing.groups[0].name,"Default proxy");assert.deepEqual(next.rules,input.rules);
 next.routing.groups[1].name="Changed";assert.equal(catalog.presets[1].groups[1].name,"YouTube");
});
test("deselecting a node removes every group membership and preserves other selections",()=>{
 let next=applyRoutingPreset(input,catalog.presets[1],false);
 next.routing.groups[1].node_ids=["a","b"];next.routing.groups[2].node_ids=["a"];
 next=selectSubscriptionNodes(next,["b"]);
 assert.deepEqual(next.routing.groups[1].node_ids,["b"]);assert.deepEqual(next.routing.groups[2].node_ids,[]);
});
test("removing a group repairs rules, source targets and catchall with no dangling reference",()=>{
 const preset=catalog.presets[1];let next=applyRoutingPreset(input,preset,false);
 next.rules=[{type:"domain",value:"example.com",target:"group:youtube"}];next.routing.final="group:youtube";next.routing.targets.netflix="group:youtube";
 next=removeRoutingGroup(next,"youtube",preset);
 assert.equal(next.rules[0].target,"proxy");assert.equal(next.routing.final,"group:proxy");assert.equal(next.routing.targets.youtube,"group:proxy");assert.equal(next.routing.targets.netflix,"group:proxy");assert.equal(next.routing.groups.some(g=>g.id==="youtube"),false);
 assert.equal(removeRoutingGroup(next,"proxy",preset),next);
});
test("changing or removing templates repairs incompatible custom targets",()=>{
 let next=applyRoutingPreset(input,catalog.presets[1],false);next.rules=[{type:"domain",value:"example.com",target:"group:youtube"}];
 next=applyRoutingPreset(next,catalog.presets[2],false);assert.equal(next.rules[0].target,"proxy");
 next=applyRoutingPreset(next,null,false);assert.equal(next.routing,null);assert.deepEqual(next.node_ids,["a","b"]);
});

test("group names reject reserved policies and case-insensitive collisions", async()=>{
 const {validRoutingNames}=await import('./subscription-routing.ts');
 let next=applyRoutingPreset(input,catalog.presets[1],false);
 assert.equal(validRoutingNames(next),true);
 next.routing.groups[1].name='GLOBAL';assert.equal(validRoutingNames(next),false);
 next.routing.groups[1].name='Netflix';next.routing.groups[2].name='netflix';assert.equal(validRoutingNames(next),false);
});
