export default [
  {
    path: "/dns-accounts",
    name: "DnsAccounts",
    component: () => import("@/views/dns-accounts/index.vue"),
    meta: {
      title: "DNS 账户",
      icon: "ri:server-line",
      rank: 1
    }
  },
  {
    path: "/domains",
    name: "Domains",
    component: () => import("@/views/domains/index.vue"),
    meta: {
      title: "域名集合",
      icon: "ri:earth-line",
      rank: 2
    }
  },
  {
    path: "/dns-records",
    name: "DnsRecords",
    component: () => import("@/views/dns-records/index.vue"),
    meta: {
      title: "DNS 解析",
      icon: "ri:list-check",
      rank: 3
    }
  }
] satisfies Array<RouteConfigsTable>;
