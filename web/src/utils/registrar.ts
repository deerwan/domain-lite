export interface RegistrarLink {
  url: string;
  label: string;
}

// 根据注册商名称启发式匹配续费控制台链接，命中返回 {url,label}，否则 null
export function registrarRenewal(
  registrar?: string | null
): RegistrarLink | null {
  if (!registrar) return null;
  const r = registrar.toLowerCase();

  if (r.includes("aliyun") || r.includes("万网") || r.includes("阿里云")) {
    return {
      url: "https://dc.console.aliyun.com/next/renewal",
      label: "阿里云续费"
    };
  }
  if (
    r.includes("tencent") ||
    r.includes("腾讯云") ||
    r.includes("dnspod") ||
    r.includes("dnsla")
  ) {
    return {
      url: "https://console.cloud.tencent.com/domain/renew",
      label: "腾讯云续费"
    };
  }
  if (r.includes("godaddy")) {
    return {
      url: "https://account.godaddy.com/products",
      label: "GoDaddy 续费"
    };
  }
  if (r.includes("namecheap")) {
    return {
      url: "https://www.namecheap.com/myaccount/renew/",
      label: "Namecheap 续费"
    };
  }
  if (r.includes("namesilo")) {
    return {
      url: "https://www.namesilo.com/account/domains",
      label: "NameSilo 续费"
    };
  }
  if (r.includes("cloudflare")) {
    return {
      url: "https://dash.cloudflare.com/?to=/:account/domains",
      label: "Cloudflare 续费"
    };
  }
  return null;
}
