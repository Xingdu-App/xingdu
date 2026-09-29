export const paidPlans = [
  { id: "start", name: "Starter", monthly: 5, yearly: 40, servers: 10 },
  { id: "premium", name: "Premium", monthly: 20, yearly: 200, servers: 50 },
] as const;
export type PaidPlan = (typeof paidPlans)[number]["id"];
export const enterpriseContact =
  "mailto:info@xingdu.app?subject=Xingdu%20Enterprise";
