# Subscription configuration templates

The subscription page offers nine complete routing presets. Select a preset,
choose nodes, optionally assign them to category groups, then save the
subscription. Existing subscriptions without a `routing` object keep their
original export behavior.

| Preset | Policy groups | Rule sets |
| --- | --- | --- |
| `allround-v1` (recommended) | 7 purpose groups: AI, development, streaming, social, gaming, work and default | All 34 catalog sources; work and entertainment coexist |
| `allround-fine-v1` | 19 groups with individual AI and media services | Same coverage, more detailed node assignment |
| `entertainment-v1` | 12 groups, including YouTube, Netflix, Disney+, HBO and Spotify | Includes AI, development, social, gaming and work sources |
| `gaming-v1` | 10 groups with Steam, Epic, consoles and game downloads | Domestic Steam downloads connect directly; also covers work and media |
| `work-cloud-v1` | 11 groups with Microsoft, Google, GitHub, Docker and AI | Includes streaming, social and gaming sources |
| `direct-first-v1` | 7 purpose groups | Listed overseas services use proxies; unmatched traffic is DIRECT |
| `balanced-v1` | Default proxy | LAN, service exceptions, ads, Apple, global services, China domains and IP ranges |
| `streaming-v1` | Default proxy, YouTube, Netflix, Telegram | Balanced sources plus those three services |
| `developer-v1` | Default proxy, OpenAI, GitHub, Microsoft, Telegram | Balanced sources plus those four services |

These are Xingdu-authored compositions informed by
[ACL4SSR's full and mini configurations](https://github.com/ACL4SSR/ACL4SSR/tree/master/Clash/config).
They reference ACL4SSR's public rule files rather than copying an entire
community profile or distributing its contents. The catalog is embedded from
`internal/subscription/routing-catalog.json`; the authenticated
`GET /api/v1/subscription-presets` endpoint serves that same catalog to the UI.
Keep existing versioned preset definitions stable; introduce a new ID when
changing group defaults or rule-set ordering.

## Combined presets and references

The six expanded presets cover the same set of applications. Their different
policy groups determine how finely users assign exit nodes, rather than
requiring users to choose between development and watching movies. The three
original preset IDs and definitions remain unchanged for existing subscriptions.
All-purpose everyday is marked as recommended; the editor collapses the template
catalog after selection so users can proceed directly to node assignments.

Research also reviewed [blackmatrix7's per-application catalog](https://github.com/blackmatrix7/ios_rule_script/blob/master/rule/Clash/README.md)
and [Sukka's modular rules](https://github.com/SukkaW/Surge). These provide
references for application categories and modular composition. The shipped
presets still reference ACL4SSR sources only; they do not copy those projects'
complete profiles, scripts, DNS settings or license terms into this repository.

Specific AI and streaming sources precede broad Google rules; GitHub precedes
Microsoft, Docker precedes general developer rules, and Steam China and game
CDN rules precede broader game platform rules. Common and domestic catchalls
come later. Game download CDNs have their own proxy group in the gaming preset;
only the Steam China source defaults to direct access. This is routing
classification, not a guarantee of game latency or streaming access.

Additional rule sources were checked for reachability and classical text
syntax. No new source requires process matching, rewriting, scripts or MITM.
AI includes OpenAI, Claude, Gemini and the upstream AI collection. Rules only
cover entries maintained upstream, not every application or domestic address.

## Groups and targets

The default group (`proxy`) always contains every selected, available node.
Groups support manual selection, lowest-latency selection and ordered failover.
The latter two use a 300-second test interval and
`https://www.gstatic.com/generate_204`. Availability of that endpoint is not a
streaming or AI service reachability test.

Users can rename groups, add groups, assign one node to several groups, choose
rule-set targets, disable individual rule sets and change the final target.
Failover membership order follows the order in which nodes were checked.
Deleting a group in the UI repairs affected targets to the default proxy.
Deselecting a node removes its memberships; deleted deployments are pruned from
memberships when subscription metadata is read. Empty category groups,
including groups whose nodes are currently unavailable, are exported as an
explicit select alias to the default proxy, never as an empty group that a
client could treat as DIRECT. If no nodes remain available, export fails.

Custom rules execute first, in their editable order; preset rule sets follow
in catalog order, and the final target is last. A custom target can be `proxy`,
`direct`, `reject`, or `group:<id>`. Remote URLs cannot be supplied by users:
rule-set overrides refer only to the sources of the selected catalog preset.
The API rejects unknown groups, invalid/duplicate names, duplicate memberships,
unselected nodes and unsupported modes. The existing tenant-scoped node checks
and PostgreSQL RLS still apply.

Subscriptions persist the optional `routing` object:

```json
{
  "preset": "balanced-v1",
  "groups": [
    { "id": "proxy", "name": "Default proxy", "type": "url-test", "node_ids": [] }
  ],
  "targets": { "ads": "disabled" },
  "final": "group:proxy"
}
```

The legacy `rules` array holds custom rules; `final_action` remains required for
backward compatibility, while `routing.final` controls configured exports.
Omitting or setting `routing` to null when saving returns to legacy routing;
custom group targets must be converted to legacy targets at the same time.
Migration 035 adds this object to subscriptions and runs through the existing
API startup migration flow. It introduces no new database privileges.

## Client adapters and external resources

- Stash and Mihomo use classical text `rule-providers` and `RULE-SET` rules.
- Surge uses URL-based `RULE-SET` rules and the global `proxy-test-url` setting.
- Loon uses `[Remote Rule]`, preserving the existing trusted-certificate-chain
  requirement. Local custom rules precede remote rules; FINAL is the fallback.
- Hysteria 2 share URIs cannot represent groups or rules. Both saving this format
  with a template and requesting a URI override on such a subscription fail.

The client downloads and caches the external lists. Stash and Mihomo providers
request a daily refresh; Surge and Loon follow their rule update behavior.
Initial import requires access to GitHub's raw content host. A source outage,
upstream rule change or device refresh setting can affect routing. Xingdu
neither fetches these lists on the API server nor sends subscription tokens or
links to their hosts. This is not a server-side mirror or immutable snapshot
service, and it does not guarantee ad coverage or streaming unblocking.

Syntax references: [Stash rule sets](https://stash.wiki/rules/rule-set),
[Mihomo providers](https://wiki.metacubex.one/config/rule-providers/),
[Surge rule sets](https://manual.nssurge.com/rules/ruleset.html),
[Surge automatic groups](https://manual.nssurge.com/policy-groups/url-test.html),
and [Loon's official example](https://github.com/Loon0x00/LoonExampleConfig/blob/master/example.conf).

The test suite covers grouping, ordering, source and target overrides,
namespace collisions, unavailable members, injection boundaries, legacy
compatibility, tenant isolation and persistence. Optional Mihomo binary tests
parse each preset using local rule caches without contacting live nodes.
Loon's renderer test uses an isolated fixture trust store. These checks do not
establish real Stash/Surge/Loon app acceptance or deployed connectivity.

## Default policy-group icons

All nine presets include colored purpose icons for their 24 distinct groups.
The glyphs come from [Lucide](https://github.com/lucide-icons/lucide), with its
ISC license and inherited Feather MIT notices retained alongside the assets.
Xingdu adds colored rounded backgrounds and serves 128px PNGs from
`/subscription-icons/v1/`. These represent categories, not official service logos.
SVG sources, the pinned upstream revision and the rendering manifest are checked
in; maintainers can regenerate PNGs with `scripts/build-subscription-icons.py`
using CairoSVG 2.8.2. Normal web builds need no image renderer or upstream download.

Applying a preset fills missing group icons and preserves existing custom URLs.
Saved subscriptions are snapshots: this change does not rewrite them. Reapply
and save a preset to fill its missing icons. Users can still edit or remove URLs.
Stash and Mihomo export `icon`; Mihomo display depends on the dashboard.
Surge/Loon do not export this field and retain the existing capability warning.

Default URLs point to `https://xingdu.app`; the corresponding web assets must
be deployed before client downloads can succeed. Self-hosted operators can use
HTTPS URLs on their own domain. Client rendering still needs App acceptance.

The earlier personal Stash configuration used
[Koolson/Qure Color](https://github.com/Koolson/Qure) and some
[Semporia hand-painted region icons](https://github.com/Semporia/Hand-Painted-icon).
Qure prohibits commercial use and the latter is described for personal use;
those images are not bundled as Xingdu SaaS defaults.
