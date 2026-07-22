export default [
  {
    path: "/record-log",
    name: "RecordLog",
    component: () => import("@/views/record-log/index.vue"),
    meta: {
      title: "变更日志",
      icon: "ep/clock",
      rank: 6
    }
  }
] satisfies Array<RouteConfigsTable>;
