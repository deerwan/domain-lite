<template>
  <div>
    <el-card>
      <div class="flex flex-wrap items-center gap-3 mb-4">
        <span class="text-lg font-medium mr-2">DNS 解析</span>
        <el-select
          v-model="selectedAccount"
          placeholder="选择 DNS 账户"
          clearable
          style="width: 220px"
          @change="onAccountChange"
        >
          <el-option
            v-for="a in accounts"
            :key="a.id"
            :label="`${a.name}（${typeLabel(a.type)}）`"
            :value="a.id"
          />
        </el-select>
        <el-select
          v-model="selectedZone"
          placeholder="选择域名 / Zone"
          clearable
          :disabled="!selectedAccount || zones.length === 0"
          style="width: 260px"
          @change="onZoneChange"
        >
          <el-option
            v-for="z in zones"
            :key="z.id"
            :label="z.name"
            :value="z.id"
          />
        </el-select>
        <el-button :disabled="!selectedZone" type="primary" @click="openAdd">
          新增记录
        </el-button>
        <el-button :disabled="!selectedZone" @click="loadRecords"
          >刷新</el-button
        >
      </div>

      <el-alert
        v-if="!selectedAccount"
        type="info"
        :closable="false"
        title="请先选择 DNS 账户，再选择要管理的域名。"
        class="mb-3"
      />

      <el-alert
        v-else-if="!loadingZones && zones.length === 0"
        type="warning"
        :closable="false"
        class="mb-3"
        title="该账户未返回任何域名"
        description="请确认 Cloudflare API Token 拥有 Zone · DNS · Read 权限，且「Zone Resources」作用域包含你的域名（建议设为 All zones）。也可在终端用 curl 验证：curl -H 'Authorization: Bearer <你的token>' https://api.cloudflare.com/client/v4/zones"
      />

      <el-table
        v-else
        v-loading="loadingRecords"
        :data="records"
        border
        empty-text="该域名下暂无解析记录"
      >
        <el-table-column prop="name" label="主机记录" min-width="160" />
        <el-table-column prop="type" label="类型" width="100" />
        <el-table-column prop="content" label="记录值" min-width="200" />
        <el-table-column prop="ttl" label="TTL" width="90" />
        <el-table-column
          v-if="isCloudflare"
          prop="proxied"
          label="代理"
          width="90"
        >
          <template #default="{ row }">
            <el-tag :type="row.proxied ? 'success' : 'info'" size="small">
              {{ row.proxied ? "已代理" : "仅 DNS" }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150">
          <template #default="{ row }">
            <el-button type="primary" link @click="openEdit(row)"
              >编辑</el-button
            >
            <el-button type="danger" link @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog
      v-model="dialog"
      :title="editingId ? '编辑记录' : '新增记录'"
      width="480px"
    >
      <el-form :model="form" label-width="110px">
        <el-form-item label="主机记录">
          <el-input v-model="form.name" placeholder="@ 或 www 或完整域名" />
        </el-form-item>
        <el-form-item label="记录类型">
          <el-select v-model="form.type" style="width: 100%">
            <el-option
              v-for="t in recordTypes"
              :key="t"
              :label="t"
              :value="t"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="记录值">
          <el-input v-model="form.content" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="TTL">
          <el-input-number v-model="form.ttl" :min="1" :max="2147483647" />
        </el-form-item>
        <el-form-item label="优先级">
          <el-input-number v-model="form.priority" :min="0" :max="65535" />
        </el-form-item>
        <el-form-item v-if="isCloudflare" label="Cloudflare 代理">
          <el-switch v-model="form.proxied" />
          <span class="ml-2 text-gray-400 text-xs"
            >开启后流量经 Cloudflare 边缘（橙色云）</span
          >
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save"
          >保存</el-button
        >
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { useRoute } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  getDnsAccounts,
  getZones,
  getRecords,
  createRecord,
  updateRecord,
  deleteRecord,
  type DnsAccountItem,
  type Zone,
  type DnsRecord
} from "@/api/domain-lite";

const accounts = ref<DnsAccountItem[]>([]);
const zones = ref<Zone[]>([]);
const records = ref<DnsRecord[]>([]);
const selectedAccount = ref<number | undefined>(undefined);
const selectedZone = ref<string>("");
const loadingZones = ref(false);
const loadingRecords = ref(false);
const dialog = ref(false);
const saving = ref(false);
const editingId = ref<string | undefined>(undefined);

const recordTypes = [
  "A",
  "AAAA",
  "CNAME",
  "MX",
  "TXT",
  "NS",
  "SRV",
  "CAA",
  "HTTPS",
  "SVCB"
];

const form = reactive({
  name: "",
  type: "A",
  content: "",
  ttl: 600,
  priority: 0,
  proxied: false
});

const selectedAccountType = computed(
  () => accounts.value.find(a => a.id === selectedAccount.value)?.type ?? ""
);
const isCloudflare = computed(() => selectedAccountType.value === "cloudflare");

const typeLabel = (t: string) => {
  switch (t) {
    case "aliyun":
      return "阿里云 DNS";
    case "cloudflare":
      return "Cloudflare";
    case "godaddy":
      return "GoDaddy";
    case "dnspod":
      return "DNSPod（腾讯云）";
    case "namecheap":
      return "Namecheap";
    case "spaceship":
      return "Spaceship";
    default:
      return t;
  }
};

function fail(msg: string, e: any) {
  const detail = e?.response?.data?.message || e?.message || "请求失败";
  ElMessage.error(`${msg}：${detail}`);
}

async function loadAccounts() {
  try {
    accounts.value = await getDnsAccounts();
  } catch (e: any) {
    fail("加载账户失败", e);
  }
}

async function onAccountChange() {
  selectedZone.value = "";
  zones.value = [];
  records.value = [];
  if (!selectedAccount.value) return;
  loadingZones.value = true;
  try {
    const res: any = await getZones(selectedAccount.value);
    if (res.code !== 0) {
      fail("加载域名列表失败", { response: { data: res } });
      zones.value = [];
    } else {
      zones.value = res.data || [];
    }
  } catch (e: any) {
    fail("加载域名列表失败", e);
    zones.value = [];
  } finally {
    loadingZones.value = false;
  }
}

async function onZoneChange() {
  records.value = [];
  if (selectedZone.value) loadRecords();
}

async function loadRecords() {
  if (!selectedAccount.value || !selectedZone.value) return;
  loadingRecords.value = true;
  try {
    const res: any = await getRecords(
      selectedAccount.value,
      selectedZone.value
    );
    if (res.code !== 0) {
      fail("加载解析记录失败", { response: { data: res } });
      records.value = [];
    } else {
      records.value = res.data || [];
    }
  } catch (e: any) {
    fail("加载解析记录失败", e);
    records.value = [];
  } finally {
    loadingRecords.value = false;
  }
}

function resetForm() {
  form.name = "";
  form.type = "A";
  form.content = "";
  form.ttl = 600;
  form.priority = 0;
  form.proxied = false;
}

function openAdd() {
  editingId.value = undefined;
  resetForm();
  dialog.value = true;
}

function openEdit(row: DnsRecord) {
  editingId.value = row.id;
  form.name = row.name;
  form.type = row.type;
  form.content = row.content;
  form.ttl = row.ttl || 600;
  form.priority = row.priority || 0;
  form.proxied = !!row.proxied;
  dialog.value = true;
}

function buildPayload() {
  const base: Record<string, unknown> = {
    name: form.name,
    type: form.type,
    content: form.content,
    ttl: form.ttl,
    priority: form.priority
  };
  if (isCloudflare.value) base.proxied = form.proxied;
  return base;
}

async function save() {
  if (!selectedAccount.value || !selectedZone.value) return;
  if (!form.name || !form.type || !form.content) {
    ElMessage.warning("请填写主机记录、类型和记录值");
    return;
  }
  saving.value = true;
  try {
    const payload = buildPayload();
    let res: any;
    if (editingId.value) {
      res = await updateRecord(
        selectedAccount.value,
        selectedZone.value,
        editingId.value,
        payload
      );
    } else {
      res = await createRecord(
        selectedAccount.value,
        selectedZone.value,
        payload
      );
    }
    if (res.code !== 0) {
      fail(editingId.value ? "更新失败" : "创建失败", {
        response: { data: res }
      });
      return;
    }
    ElMessage.success(editingId.value ? "已更新" : "已添加");
    dialog.value = false;
    loadRecords();
  } catch (e: any) {
    fail(editingId.value ? "更新失败" : "创建失败", e);
  } finally {
    saving.value = false;
  }
}

async function remove(row: DnsRecord) {
  if (!selectedAccount.value || !selectedZone.value) return;
  try {
    await ElMessageBox.confirm(
      `确认删除记录「${row.name} ${row.type} ${row.content}」？`,
      "提示",
      { type: "warning" }
    );
  } catch {
    return;
  }
  try {
    const res: any = await deleteRecord(
      selectedAccount.value,
      selectedZone.value,
      row.id
    );
    if (res.code !== 0) {
      fail("删除失败", { response: { data: res } });
      return;
    }
    ElMessage.success("已删除");
    loadRecords();
  } catch (e: any) {
    fail("删除失败", e);
  }
}

const route = useRoute();

onMounted(async () => {
  await loadAccounts();
  // 支持从「域名列表」跳转并自动选中账户与域名
  const qa = route.query.account;
  if (qa) {
    selectedAccount.value = Number(qa);
    await onAccountChange();
    const qz = route.query.zone;
    if (qz) {
      selectedZone.value = String(qz);
      loadRecords();
    }
  }
});
</script>
