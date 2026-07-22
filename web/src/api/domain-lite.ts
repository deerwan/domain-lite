import { http } from "@/utils/http";

export type DnsAccountItem = {
  id: number;
  type: string;
  name: string;
  access_key: string;
  created_at: string;
};

export const getDnsAccounts = (): Promise<DnsAccountItem[]> => {
  return http.get("/api/dns-accounts").then((res: any) => res.data);
};

export const createDnsAccount = (data: {
  type: string;
  name: string;
  access_key: string;
  secret_key: string;
  ext?: string;
}) => {
  return http.post("/api/dns-accounts", { data });
};

export const deleteDnsAccount = (id: number) => {
  return http.request("delete", `/api/dns-accounts/${id}`);
};

export const getDomains = () => {
  return http.get("/api/domains").then((res: any) => res.data);
};

export const createDomain = (data: {
  domain: string;
  dns_account_id?: number;
  zone_id?: string;
  note?: string;
}) => {
  return http.post("/api/domains", { data });
};

export const deleteDomain = (id: number) => {
  return http.request("delete", `/api/domains/${id}`);
};

// ===== 域名列表：聚合所有 DNS 账户已识别到的域名 =====
export type DiscoveredDomain = {
  domain: string;
  zone_id: string;
  account_id: number;
  account_name: string;
  account_type: string;
  registrar?: string;
  expire_at?: string;
  status?: string;
  whois_manual?: boolean;
  privacy?: boolean;
};

/** 聚合当前用户所有 DNS 账户(zones) 下识别到的域名 */
export const getDiscoveredDomains = (
  refresh = false
): Promise<ApiResult<DiscoveredDomain[]>> => {
  return http.get(`/api/domains/discovered${refresh ? "?refresh=1" : ""}`);
};

/** 手动设置某域名的 WHOIS 信息（注册商/到期日/状态），钉住后不被自动同步覆盖 */
export const setDomainWhois = (data: {
  domain: string;
  dns_account_id: number;
  registrar?: string;
  expire_at?: string | null;
  status?: string;
}) => {
  return http.request("put", "/api/domains/whois", { data });
};

/** 取消手动钉住，恢复自动 WHOIS 同步 */
export const clearDomainWhois = (data: {
  domain: string;
  dns_account_id: number;
}) => {
  return http.request("delete", "/api/domains/whois", { data });
};

/** 对所有已识别域名查询 WHOIS，填充注册商/到期日/状态 */
export type EnrichResult = {
  total: number;
  success: number;
  failed: number;
  failedList: string[];
};
export const enrichDomainsWhois = (): Promise<ApiResult<EnrichResult>> => {
  return http.post("/api/domains/enrich-whois", {});
};

// ===== DNS 服务商侧：域名(zones) 与 解析记录(records) =====

export type Zone = {
  id: string;
  name: string;
};

export type DnsRecord = {
  id: string;
  name: string;
  type: string;
  content: string;
  ttl: number;
  priority?: number;
  proxied?: boolean;
};

type ApiResult<T> = { code: number; message: string; data: T; errors?: any[] };

/** 列出某 DNS 账户下的域名（zones） */
export const getZones = (accountId: number): Promise<ApiResult<Zone[]>> => {
  return http.get(`/api/dns-accounts/${accountId}/zones`);
};

/** 列出某 zone 下的解析记录 */
export const getRecords = (
  accountId: number,
  zone: string
): Promise<ApiResult<DnsRecord[]>> => {
  return http.get(
    `/api/dns-accounts/${accountId}/zones/${encodeURIComponent(zone)}/records`
  );
};

/** 新增解析记录 */
export const createRecord = (
  accountId: number,
  zone: string,
  data: Partial<DnsRecord>
) => {
  return http.post(
    `/api/dns-accounts/${accountId}/zones/${encodeURIComponent(zone)}/records`,
    { data }
  );
};

/** 更新解析记录 */
export const updateRecord = (
  accountId: number,
  zone: string,
  recordId: string,
  data: Partial<DnsRecord>
) => {
  return http.request(
    "put",
    `/api/dns-accounts/${accountId}/zones/${encodeURIComponent(
      zone
    )}/records/${encodeURIComponent(recordId)}`,
    { data }
  );
};

/** 删除解析记录 */
export const deleteRecord = (
  accountId: number,
  zone: string,
  recordId: string
) => {
  return http.request(
    "delete",
    `/api/dns-accounts/${accountId}/zones/${encodeURIComponent(
      zone
    )}/records/${encodeURIComponent(recordId)}`
  );
};

// ===== 通知设置（前端可配置，覆盖 env 默认值） =====

export type NotifySettings = {
  enabled: boolean;
  type: string;
  chat_id: string;
  threshold_days: number;
  sync_interval_min: number;
  has_webhook: boolean;
  webhook_masked: string;
};

export const getNotifySettings = (): Promise<ApiResult<NotifySettings>> => {
  return http.get("/api/notify-settings");
};

export const updateNotifySettings = (data: {
  enabled: boolean;
  type: string;
  webhook_url?: string;
  chat_id: string;
  threshold_days: number;
  sync_interval_min: number;
}) => {
  return http.request("put", "/api/notify-settings", { data });
};

export const testNotifySettings = (data: {
  type: string;
  webhook_url?: string;
  chat_id: string;
  threshold_days: number;
}) => {
  return http.post("/api/notify-settings/test", { data });
};

// ===== 仪表盘统计 =====

export type ExpiringItem = {
  domain: string;
  expire_at: string;
  days_left: number;
  registrar: string;
  status: string;
  account_name: string;
};

export type StatsResult = {
  total_domains: number;
  expiring_soon: number;
  expired: number;
  accounts: number;
  threshold_days: number;
  by_account: { account_id: number; account_name: string; count: number }[];
  expiring_list: ExpiringItem[];
};

export const getStats = (
  refresh = false
): Promise<ApiResult<StatsResult>> => {
  return http.get(`/api/stats${refresh ? "?refresh=1" : ""}`);
};

// ===== 解析记录变更日志 =====

export type RecordLog = {
  id: number;
  account_name: string;
  zone: string;
  action: "create" | "update" | "delete";
  record_type: string;
  record_name: string;
  content_before: string;
  content_after: string;
  created_at: string;
};

export const getRecordLogs = (params?: {
  account?: number | string;
  zone?: string;
}): Promise<ApiResult<RecordLog[]>> => {
  return http.get("/api/record-logs", { params: params || {} });
};
