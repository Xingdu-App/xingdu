import definitions from "./rule-presets.json";
import type { RuleTemplateInput } from "./api";
export type RulePreset = RuleTemplateInput & {
  id: string;
  description: string;
};
// Xingdu-authored, bounded starter templates, not copies of full community sets.
export const rulePresets = definitions as RulePreset[];
export const ruleTemplateSources = [
  {
    name: "ACL4SSR",
    url: "https://github.com/ACL4SSR/ACL4SSR/blob/master/Clash/config/ACL4SSR_Online.ini",
    description: "综合分流、广告拦截与策略组配置",
  },
  {
    name: "blackmatrix7",
    url: "https://github.com/blackmatrix7/ios_rule_script",
    description: "按应用分类的多客户端规则集",
  },
  {
    name: "MetaCubeX",
    url: "https://github.com/MetaCubeX/meta-rules-dat",
    description: "Mihomo 的 GeoIP、GeoSite 与规则数据",
  },
];
