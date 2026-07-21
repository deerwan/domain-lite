<template>
  <div>
    <el-card>
      <div class="flex justify-between items-center mb-3">
        <span class="text-lg font-medium">到期日历</span>
        <div class="flex items-center gap-3 text-xs text-gray-400">
          <span><i class="dot expired" /> 已过期</span>
          <span><i class="dot soon" /> 30 天内</span>
          <span><i class="dot warn" /> 90 天内</span>
          <span><i class="dot ok" /> 正常</span>
          <el-button :loading="loading" @click="load">刷新</el-button>
        </div>
      </div>

      <el-calendar v-loading="loading">
        <template #date-cell="{ data }">
          <div class="h-full flex flex-col">
            <span class="text-xs">{{ data.day.slice(5) }}</span>
            <div class="flex flex-col gap-0.5 mt-1 overflow-hidden">
              <div
                v-for="d in map[data.day] || []"
                :key="d.domain"
                class="text-[11px] leading-tight truncate rounded px-1"
                :style="{
                  background: dotColor(d.expire_at) + '22',
                  color: dotColor(d.expire_at)
                }"
                :title="`${d.domain}（剩 ${daysUntil(d.expire_at)} 天）`"
                @click="openRenewal(d.registrar)"
              >
                {{ d.domain }}
              </div>
            </div>
          </div>
        </template>
      </el-calendar>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { getDiscoveredDomains } from "@/api/domain-lite";
import { registrarRenewal } from "@/utils/registrar";

const loading = ref(false);
const map = reactive<Record<string, any[]>>({});

async function load() {
  loading.value = true;
  try {
    const disc: any = await getDiscoveredDomains();
    const data: any[] = disc.data || [];
    for (const k of Object.keys(map)) delete map[k];
    for (const d of data) {
      if (!d.expire_at) continue;
      const day = d.expire_at.slice(0, 10);
      (map[day] ||= []).push(d);
    }
  } catch (e: any) {
    ElMessage.error(
      "加载日历失败：" + (e?.response?.data?.message || e?.message)
    );
  } finally {
    loading.value = false;
  }
}

function daysUntil(expire: string): number {
  return Math.floor((new Date(expire).getTime() - Date.now()) / 86400000);
}
function dotColor(expire: string): string {
  const d = daysUntil(expire);
  if (d < 0) return "#f56c6c";
  if (d <= 30) return "#f56c6c";
  if (d <= 90) return "#e6a23c";
  return "#67c23a";
}
function openRenewal(registrar: string) {
  const link = registrarRenewal(registrar);
  if (link) window.open(link.url, "_blank", "noopener");
}

onMounted(load);
</script>

<style scoped>
.dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  margin-right: 4px;
  border-radius: 50%;
}

.dot.expired,
.dot.soon {
  background: #f56c6c;
}

.dot.warn {
  background: #e6a23c;
}

.dot.ok {
  background: #67c23a;
}
</style>
