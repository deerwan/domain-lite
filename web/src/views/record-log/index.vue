<template>
  <div>
    <el-card>
      <div class="flex justify-between items-center mb-3">
        <span class="text-lg font-medium">解析记录变更日志</span>
        <el-button :loading="loading" @click="load">刷新</el-button>
      </div>

      <el-table v-loading="loading" :data="logs" stripe border size="default">
        <el-table-column prop="created_at" label="时间" width="180">
          <template #default="{ row }">
            {{
              row.created_at
                ? row.created_at.replace("T", " ").slice(0, 19)
                : ""
            }}
          </template>
        </el-table-column>
        <el-table-column prop="account_name" label="DNS 账户" min-width="140" />
        <el-table-column prop="zone" label="域名" min-width="160" />
        <el-table-column label="操作" width="90">
          <template #default="{ row }">
            <el-tag :type="actionType(row.action)" size="small">
              {{ actionLabel(row.action) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="record_type" label="类型" width="90" />
        <el-table-column prop="record_name" label="记录名" min-width="160" />
        <el-table-column label="变更前" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">{{
            row.content_before || "—"
          }}</template>
        </el-table-column>
        <el-table-column label="变更后" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">{{ row.content_after || "—" }}</template>
        </el-table-column>
        <template #empty>
          <span class="text-gray-400">暂无变更记录</span>
        </template>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { getRecordLogs, type RecordLog } from "@/api/domain-lite";

const loading = ref(false);
const logs = ref<RecordLog[]>([]);

async function load() {
  loading.value = true;
  try {
    const res: any = await getRecordLogs();
    logs.value = res.data || [];
  } catch (e: any) {
    ElMessage.error(
      "加载变更日志失败：" + (e?.response?.data?.message || e?.message)
    );
  } finally {
    loading.value = false;
  }
}

function actionType(action: string): "success" | "warning" | "danger" {
  if (action === "create") return "success";
  if (action === "update") return "warning";
  return "danger";
}
function actionLabel(action: string): string {
  if (action === "create") return "新增";
  if (action === "update") return "修改";
  return "删除";
}

onMounted(load);
</script>
