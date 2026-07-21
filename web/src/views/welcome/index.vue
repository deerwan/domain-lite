<template>
  <div>
    <div class="flex justify-between items-center mb-4">
      <h1 class="text-xl font-semibold">域名概览</h1>
      <div>
        <el-button :loading="syncing" @click="syncNow"
          >立即同步 WHOIS</el-button
        >
        <el-button :loading="loading" @click="load">刷新</el-button>
      </div>
    </div>

    <el-row :gutter="16">
      <el-col :xs="12" :sm="6">
        <el-card shadow="hover">
          <el-statistic title="域名总数" :value="stats.total_domains" />
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-card shadow="hover">
          <el-statistic
            title="临期（≤阈值）"
            :value="stats.expiring_soon"
            class="text-amber-500"
          />
          <div class="text-gray-400 text-xs mt-1">
            阈值 {{ stats.threshold_days }} 天
          </div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-card shadow="hover">
          <el-statistic
            title="已过期"
            :value="stats.expired"
            class="text-red-500"
          />
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="6">
        <el-card shadow="hover">
          <el-statistic title="DNS 账户" :value="stats.accounts" />
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="mt-4">
      <el-col :xs="24" :md="14">
        <el-card shadow="never">
          <div class="font-medium mb-3">临期域名</div>
          <el-table
            v-loading="loading"
            :data="stats.expiring_list"
            border
            empty-text="没有临期域名 🎉"
          >
            <el-table-column prop="domain" label="域名" min-width="160" />
            <el-table-column prop="account_name" label="账户" width="140" />
            <el-table-column prop="registrar" label="注册商" width="140" />
            <el-table-column label="剩余" width="120">
              <template #default="{ row }">
                <el-tag :type="row.days_left < 0 ? 'danger' : 'warning'">
                  {{ row.days_left < 0 ? "已过期" : row.days_left + " 天" }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="expire_at" label="到期日" width="120" />
            <el-table-column label="续费" width="100">
              <template #default="{ row }">
                <el-button
                  v-if="renewalOf(row.registrar)"
                  type="success"
                  link
                  @click="openRenewal(row.registrar)"
                >
                  {{ renewalOf(row.registrar)?.label }}
                </el-button>
                <span v-else class="text-gray-400">—</span>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
      <el-col :xs="24" :md="10">
        <el-card shadow="never">
          <div class="font-medium mb-3">按账户分布</div>
          <el-table :data="stats.by_account" border empty-text="暂无账户">
            <el-table-column prop="account_name" label="账户" />
            <el-table-column prop="count" label="域名数" width="100" />
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import {
  getStats,
  enrichDomainsWhois,
  type StatsResult
} from "@/api/domain-lite";
import { registrarRenewal } from "@/utils/registrar";

const loading = ref(false);
const syncing = ref(false);
const stats = reactive<StatsResult>({
  total_domains: 0,
  expiring_soon: 0,
  expired: 0,
  accounts: 0,
  threshold_days: 30,
  by_account: [],
  expiring_list: []
});

async function load() {
  loading.value = true;
  try {
    const res: any = await getStats();
    if (res.code === 0 && res.data) {
      Object.assign(stats, res.data);
    }
  } finally {
    loading.value = false;
  }
}

async function syncNow() {
  syncing.value = true;
  try {
    const res: any = await enrichDomainsWhois();
    if (res.code === 0 && res.data) {
      ElMessage.success(
        `同步完成：成功 ${res.data.success} / 失败 ${res.data.failed}`
      );
    }
    await load();
  } finally {
    syncing.value = false;
  }
}

onMounted(load);

function renewalOf(registrar: string) {
  return registrarRenewal(registrar);
}
function openRenewal(registrar: string) {
  const link = registrarRenewal(registrar);
  if (link) window.open(link.url, "_blank", "noopener");
}
</script>
