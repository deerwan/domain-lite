type Result = {
  success: boolean;
  data: Array<any>;
};

/** 获取后端动态路由。本系统使用静态路由，直接返回空数组（菜单由 src/router/modules 生成） */
export const getAsyncRoutes = () => {
  return Promise.resolve<Result>({ success: true, data: [] });
};
