import type { RoutingPreset, SubscriptionInput } from "./api";

export function applyRoutingPreset(
  input: SubscriptionInput,
  preset: RoutingPreset | null,
  english: boolean,
): SubscriptionInput {
  const ids = new Set(preset?.groups.map((g) => g.id) ?? []);
  return {
    ...input,
    rules: input.rules.map((rule) =>
      rule.target.startsWith("group:") && !ids.has(rule.target.slice(6))
        ? { ...rule, target: "proxy" }
        : rule,
    ),
    routing: preset
      ? {
          preset: preset.id,
          groups: preset.groups.map((g) => ({
            id: g.id,
            icon: input.routing?.groups.find((old) => old.id === g.id)?.icon ?? g.icon,
            name: english ? g.name_en : g.name,
            type: g.type,
            node_ids:
              input.routing?.groups
                .find((old) => old.id === g.id)
                ?.node_ids.filter((id) => input.node_ids.includes(id)) ?? [],
          })),
          targets: {},
          final: preset.final,
        }
      : null,
  };
}

export function selectSubscriptionNodes(
  input: SubscriptionInput,
  ids: string[],
): SubscriptionInput {
  return {
    ...input,
    node_ids: ids,
    routing: input.routing
      ? {
          ...input.routing,
          groups: input.routing.groups.map((g) => ({
            ...g,
            node_ids: g.node_ids.filter((id) => ids.includes(id)),
          })),
        }
      : input.routing,
  };
}

export function removeRoutingGroup(
  input: SubscriptionInput,
  id: string,
  preset: RoutingPreset,
): SubscriptionInput {
  if (!input.routing || id === "proxy") return input;
  const target = `group:${id}`;
  const targets = { ...input.routing.targets };
  for (const b of preset.bindings)
    if ((targets[b.source] ?? b.target) === target)
      targets[b.source] = "group:proxy";
  return {
    ...input,
    rules: input.rules.map((r) =>
      r.target === target ? { ...r, target: "proxy" } : r,
    ),
    routing: {
      ...input.routing,
      groups: input.routing.groups.filter((g) => g.id !== id),
      targets,
      final:
        input.routing.final === target ? "group:proxy" : input.routing.final,
    },
  };
}

export function validRoutingNames(input: SubscriptionInput): boolean {
  if (!input.routing) return true;
  const names = new Set<string>();
  return input.routing.groups.every((group) => {
    const name = group.name;
    if (
      !name ||
      name.trim() !== name ||
      [...name].length > 48 ||
      ["direct", "reject", "reject-drop", "pass", "global"].includes(
        name.toLowerCase(),
      ) ||
      names.has(name.toLowerCase()) ||
      [...name].some(
        (c) => ',=[]#;/"\\'.includes(c) || /[\p{Cc}\p{Zl}\p{Zp}]/u.test(c),
      )
    )
      return false;
    names.add(name.toLowerCase());
    return true;
  });
}
