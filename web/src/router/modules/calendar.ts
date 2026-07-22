export default [
  {
    path: "/calendar",
    name: "ExpiryCalendar",
    component: () => import("@/views/calendar/index.vue"),
    meta: {
      title: "到期日历",
      icon: "ep/calendar",
      rank: 5
    }
  }
] satisfies Array<RouteConfigsTable>;
