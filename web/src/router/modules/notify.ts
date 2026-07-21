export default [
  {
    path: "/notify",
    name: "NotifySettings",
    component: () => import("@/views/notify/index.vue"),
    meta: {
      title: "通知设置",
      icon: "ri:notification-3-line",
      rank: 4
    }
  }
] satisfies Array<RouteConfigsTable>;
