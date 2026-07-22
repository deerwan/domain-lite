export default [
  {
    path: "/dns-accounts",
    name: "DnsAccounts",
    component: () => import("@/views/dns-accounts/index.vue"),
    meta: {
      title: "DNS 账户",
      icon: "ep/office-building",
      rank: 1
    }
  },
  {
    path: "/domains",
    name: "Domains",
    component: () => import("@/views/domains/index.vue"),
    meta: {
      title: "域名列表",
      icon: "ep/document",
      rank: 2
    }
  },
  {
    path: "/dns-records",
    name: "DnsRecords",
    component: () => import("@/views/dns-records/index.vue"),
    meta: {
      title: "DNS 解析",
      icon: "ep/link",
      rank: 3
    }
  }
] satisfies Array<RouteConfigsTable>;
