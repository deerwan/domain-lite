<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { getStats, getRecordLogs } from "@/api/domain-lite";
import type { ExpiringItem, RecordLog } from "@/api/domain-lite";
import type { ListItem, TabItem } from "./data";
import NoticeList from "./components/NoticeList.vue";
import BellIcon from "~icons/ep/bell";

const expiring = ref<ListItem[]>([]);
const changes = ref<ListItem[]>([]);

const tabs = computed<TabItem[]>(() => [
  {
    key: "1",
    name: "临期待办",
    list: expiring.value,
    emptyText: "暂无临期域名"
  },
  {
    key: "2",
    name: "最近变更",
    list: changes.value,
    emptyText: "暂无变更记录"
  }
]);

const total = computed(() => expiring.value.length + changes.value.length);
const activeKey = ref("1");

const getLabel = computed(
  () => (item: TabItem) =>
    item.name + (item.list.length > 0 ? `(${item.list.length})` : "")
);

function actionLabel(action: string): string {
  if (action === "create") return "新增";
  if (action === "update") return "修改";
  return "删除";
}

async function loadExpiring() {
  try {
    const res: any = await getStats();
    const list: ExpiringItem[] = res?.data?.expiring_list || [];
    expiring.value = list.map(d => ({
      avatar: "",
      title: d.domain,
      description: `${d.account_name || "—"} · 剩余 ${d.days_left} 天`,
      datetime: (d.expire_at || "").slice(0, 10),
      type: "1",
      extra: d.days_left < 0 ? "已过期" : "临期",
      status: d.days_left < 0 ? "danger" : "warning"
    }));
  } catch {
    /* ignore */
  }
}

async function loadChanges() {
  try {
    const res: any = await getRecordLogs();
    const list: RecordLog[] = (res?.data || []).slice(0, 20);
    changes.value = list.map(d => ({
      avatar: "",
      title: `${actionLabel(d.action)} ${d.record_name || ""}`.trim(),
      description: `${d.zone || ""} · ${d.record_type || ""}：${
        d.content_before || "—"
      } → ${d.content_after || "—"}`,
      datetime: (d.created_at || "").replace("T", " ").slice(0, 16),
      type: "2",
      extra: d.account_name,
      status:
        d.action === "delete"
          ? "danger"
          : d.action === "create"
            ? "success"
            : "warning"
    }));
  } catch {
    /* ignore */
  }
}

async function refresh() {
  await Promise.all([loadExpiring(), loadChanges()]);
}

onMounted(refresh);
</script>

<template>
  <el-dropdown
    trigger="click"
    placement="bottom-end"
    @visible-change="v => v && refresh()"
  >
    <span
      :class="[
        'dropdown-badge',
        'navbar-bg-hover',
        'select-none',
        Number(total) !== 0 && 'mr-[10px]'
      ]"
    >
      <el-badge :value="Number(total) === 0 ? '' : total" :max="99">
        <span class="header-notice-icon">
          <IconifyIconOffline :icon="BellIcon" />
        </span>
      </el-badge>
    </span>
    <template #dropdown>
      <el-dropdown-menu>
        <el-tabs
          v-model="activeKey"
          :stretch="true"
          class="dropdown-tabs"
          :style="{ width: tabs.length === 0 ? '200px' : '380px' }"
        >
          <el-empty
            v-if="tabs.length === 0"
            description="暂无消息"
            :image-size="60"
          />
          <span v-else>
            <template v-for="item in tabs" :key="item.key">
              <el-tab-pane :label="getLabel(item)" :name="item.key">
                <el-scrollbar max-height="330px">
                  <div class="noticeList-container">
                    <NoticeList :list="item.list" :emptyText="item.emptyText" />
                  </div>
                </el-scrollbar>
              </el-tab-pane>
            </template>
          </span>
        </el-tabs>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<style lang="scss" scoped>
.dropdown-badge {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 48px;
  cursor: pointer;

  .header-notice-icon {
    font-size: 18px;
  }
}

.dropdown-tabs {
  .noticeList-container {
    padding: 15px 24px 0;
  }

  :deep(.el-tabs__header) {
    margin: 0;
  }

  :deep(.el-tabs__nav-wrap)::after {
    height: 1px;
  }

  :deep(.el-tabs__nav-wrap) {
    padding: 0 36px;
  }
}
</style>
