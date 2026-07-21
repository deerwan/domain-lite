import { http } from "@/utils/http";
import { getToken } from "@/utils/auth";

export type UserResult = {
  success: boolean;
  message?: string;
  data: {
    /** 头像 */
    avatar?: string;
    /** 用户名 */
    username?: string;
    /** 昵称 */
    nickname?: string;
    /** 当前登录用户的角色 */
    roles?: Array<string>;
    /** 按钮级别权限 */
    permissions?: Array<string>;
    /** `token` */
    accessToken: string;
    /** 用于调用刷新`accessToken`的接口时所需的`token` */
    refreshToken: string;
    /** `accessToken`的过期时间 */
    expires: Date;
  };
};

export type RefreshTokenResult = {
  success: boolean;
  data: {
    /** `token` */
    accessToken: string;
    /** 用于调用刷新`accessToken`的接口时所需的`token` */
    refreshToken: string;
    /** `accessToken`的过期时间 */
    expires: Date;
  };
};

/** 登录：对接 Go 后端 /api/auth/login */
export const getLogin = (data: { username: string; password: string }) => {
  return http.post("/api/auth/login", { data }).then((res: any) => {
    if (res && res.code === 0) {
      const t = res.data.token;
      const u = res.data.user;
      const expires = new Date(Date.now() + 7 * 24 * 3600 * 1000);
      return {
        success: true,
        data: {
          avatar: "",
          username: u.username,
          nickname: u.username,
          roles: [u.role || "admin"],
          permissions: [],
          accessToken: t,
          refreshToken: t,
          expires
        }
      } as UserResult;
    }
    return { success: false, message: res?.message, data: null } as any;
  });
};

/** 刷新`token`：后端无刷新接口，直接复用已存 token */
export const refreshTokenApi = (_data?: { refreshToken?: string }) => {
  const info = getToken();
  const expires = new Date(info.expires);
  return Promise.resolve({
    success: true,
    data: {
      accessToken: info.accessToken,
      refreshToken: info.refreshToken,
      expires
    }
  } as RefreshTokenResult);
};

/** 获取用户信息：对接 /api/auth/me */
export const getUserInfo = () => {
  return http.get("/api/auth/me").then((res: any) => {
    if (res && res.code === 0) {
      return {
        success: true,
        data: {
          avatar: "",
          username: res.data.username,
          nickname: res.data.username,
          roles: [res.data.role || "admin"],
          permissions: []
        }
      };
    }
    return { success: false, data: null };
  });
};
