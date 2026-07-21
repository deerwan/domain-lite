<template>
  <div>
    <el-card>
      <div class="flex justify-between mb-4 items-center">
        <span class="text-lg font-medium">域名列表</span>
        <div class="flex items-center gap-2">
          <span class="text-sm text-gray-400">共 {{ list.length }} 个域名</span>
          <el-button :loading="enriching" @click="enrichWhois">
            刷新到期信息
          </el-button>
          <el-button type="primary" :loading="loading" @click="load">
            刷新
          </el-button>
        </div>
      </div>

      <el-alert
        v-if="!loading && discoveredErrors.length"
        type="warning"
        :closable="false"
        class="mb-3"
        :title="`${discoveredErrors.length} 个账户识别失败`"
        :description="
          discoveredErrors
            .map((e: any) => `${e.account}: ${e.error}`)
            .join('；')
        "
      />

      <el-table
        v-loading="loading"
        :data="list"
        border
        empty-text="暂无域名，请先在「DNS 账户」添加账户并测试通过"
      >
        <el-table-column prop="domain" label="域名" min-width="200" />
        <el-table-column label="来源" min-width="220">
          <template #default="{ row }">
            <el-tag
              size="small"
              :type="row.account_type === 'cloudflare' ? 'warning' : 'success'"
            >
              {{ typeLabel(row.account_type) }} · {{ row.account_name }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          prop="zone_id"
          label="Zone ID"
          min-width="160"
          show-overflow-tooltip
        />
        <el-table-column prop="registrar" label="注册商" min-width="150">
          <template #default="{ row }">{{ row.registrar || "—" }}</template>
        </el-table-column>
        <el-table-column label="到期日" min-width="200">
          <template #default="{ row }">
            <template v-if="row.expire_at">
              <span
                :style="{ color: expireColor(row.expire_at), fontWeight: 600 }"
              >
                {{ row.expire_at.slice(0, 10) }}
              </span>
              <span
                class="ml-1 text-xs"
                :style="{ color: expireColor(row.expire_at) }"
              >
                {{ expireText(row.expire_at) }}
              </span>
            </template>
            <span v-else class="text-gray-400">未查询</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" min-width="130">
          <template #default="{ row }">
            <el-tag
              v-if="row.status"
              size="small"
              :type="statusType(row.status)"
            >
              {{ row.status }}
            </el-tag>
            <span v-else class="text-gray-400">—</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link @click="manage(row)">
              管理解析
            </el-button>
            <el-button
              v-if="renewalOf(row.registrar)"
              type="success"
              link
              @click="openRenewal(row.registrar)"
            >
              续费
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import {
  getDiscoveredDomains,
  enrichDomainsWhois,
  type DiscoveredDomain
} from "@/api/domain-lite";
import { registrarRenewal } from "@/utils/registrar";

const router = useRouter();
const list = ref<any[]>([]);
const loading = ref(false);
const enriching = ref(false);
const discoveredErrors = ref<any[]>([]);

const typeLabel = (t: string) =>
  t === "aliyun" ? "阿里云 DNS" : t === "cloudflare" ? "Cloudflare" : t;

// 聚合：各 DNS 账户自动识别到的域名
async function load() {
  loading.value = true;
  try {
    const disc = await getDiscoveredDomains();
    discoveredErrors.value = disc.errors || [];
    list.value = (disc.data || []).map((d: DiscoveredDomain) => ({
      domain: d.domain,
      account_type: d.account_type,
      account_name: d.account_name,
      account_id: d.account_id,
      zone_id: d.zone_id,
      registrar: d.registrar,
      expire_at: d.expire_at,
      status: d.status
    }));
  } catch (e: any) {
    const detail = e?.response?.data?.message || e?.message || "加载失败";
    ElMessage.error(`加载域名列表失败：${detail}`);
  } finally {
    loading.value = false;
  }
}

// 对所有已识别域名查询 WHOIS，填充注册商/到期日/状态后刷新列表
async function enrichWhois() {
  enriching.value = true;
  try {
    const res = await enrichDomainsWhois();
    const { total, success, failed, failedList } = res.data;
    if (failed > 0) {
      ElMessage.warning(
        `WHOIS 刷新完成：成功 ${success}/${total}，失败 ${failed}`
      );
      console.warn("WHOIS 查询失败：", failedList);
    } else {
      ElMessage.success(`WHOIS 刷新完成：成功 ${success}/${total}`);
    }
    await load();
  } catch (e: any) {
    const detail = e?.response?.data?.message || e?.message || "刷新失败";
    ElMessage.error(`刷新到期信息失败：${detail}`);
  } finally {
    enriching.value = false;
  }
}

// 到期日展示辅助
function daysUntil(expire: string): number {
  return Math.floor((new Date(expire).getTime() - Date.now()) / 86400000);
}
function expireColor(expire: string): string {
  const d = daysUntil(expire);
  if (d < 0) return "#f56c6c"; // 已过期
  if (d <= 30) return "#f56c6c"; // 临期
  if (d <= 90) return "#e6a23c"; // 预警
  return "#67c23a"; // 正常
}
function expireText(expire: string): string {
  const d = daysUntil(expire);
  return d < 0 ? `已过期 ${-d} 天` : `剩 ${d} 天`;
}
function statusType(s: string): any {
  const low = s.toLowerCase();
  if (
    low.includes("hold") ||
    low.includes("redemption") ||
    low.includes("pendingdelete")
  )
    return "danger";
  if (low.includes("ok") || low.includes("active")) return "success";
  return "warning";
}

// 跳转到解析记录页并自动选中对应账户与域名
function manage(row: any) {
  router.push({
    path: "/dns-records",
    query: { account: String(row.account_id), zone: row.zone_id }
  });
}

// 注册商续费链接（启发式匹配，命中才显示「续费」）
function renewalOf(registrar: string) {
  return registrarRenewal(registrar);
}
function openRenewal(registrar: string) {
  const link = registrarRenewal(registrar);
  if (link) window.open(link.url, "_blank", "noopener");
}

onMounted(load);
</script>
