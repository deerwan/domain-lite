// 这里存放本地图标，在 src/layout/index.vue 文件中加载，避免在首启动加载
import { getSvgInfo } from "@pureadmin/utils";
import { addIcon } from "@iconify/vue/dist/offline";

// https://icon-sets.iconify.design/ep/?keyword=ep
import EpHomeFilled from "~icons/ep/home-filled?raw";
import EpOfficeBuilding from "~icons/ep/office-building?raw";
import EpDocument from "~icons/ep/document?raw";
import EpLink from "~icons/ep/link?raw";
import EpBell from "~icons/ep/bell?raw";
import EpCalendar from "~icons/ep/calendar?raw";
import EpClock from "~icons/ep/clock?raw";

// https://icon-sets.iconify.design/ri/?keyword=ri
import RiSearchLine from "~icons/ri/search-line?raw";
import RiInformationLine from "~icons/ri/information-line?raw";

const icons = [
  // Element Plus Icon: https://github.com/element-plus/element-plus-icons
  ["ep/home-filled", EpHomeFilled],
  ["ep/office-building", EpOfficeBuilding],
  ["ep/document", EpDocument],
  ["ep/link", EpLink],
  ["ep/bell", EpBell],
  ["ep/calendar", EpCalendar],
  ["ep/clock", EpClock],
  // Remix Icon: https://github.com/Remix-Design/RemixIcon
  ["ri/search-line", RiSearchLine],
  ["ri/information-line", RiInformationLine]
];

// 本地菜单图标，后端在路由的 icon 中返回对应的图标字符串并且前端在此处使用 addIcon 添加即可渲染菜单图标
icons.forEach(([name, icon]) => {
  addIcon(name as string, getSvgInfo(icon as string));
});
