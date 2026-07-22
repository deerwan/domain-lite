<template>
  <div>
    <el-card>
      <div class="flex justify-between mb-4">
        <span class="text-lg font-medium">DNS 账户</span>
        <el-button type="primary" @click="openAdd">新增账户</el-button>
      </div>
      <el-table v-loading="loading" :data="list" border>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="type" label="类型" width="140">
          <template #default="{ row }">
            <el-tag>{{ typeLabel(row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="备注名" />
        <el-table-column prop="access_key" label="AccessKey(脱敏)" />
        <el-table-column prop="created_at" label="创建时间" />
        <el-table-column label="操作" width="260">
          <template #default="{ row }">
            <el-button type="primary" link @click="testConn(row)"
              >测试</el-button
            >
            <el-button type="danger" link @click="remove(row)">删除</el-button>
            <el-link
              v-if="providerConsoleUrl(row.type)"
              type="info"
              :href="providerConsoleUrl(row.type)"
              target="_blank"
              :underline="false"
              class="ml-1"
              >控制台 ↗</el-link
            >
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialog" title="新增 DNS 账户" width="460px">
      <el-form :model="form" label-width="120px">
        <el-form-item label="服务商类型">
          <el-select v-model="form.type" placeholder="选择服务商">
            <el-option label="Cloudflare" value="cloudflare" />
            <el-option label="阿里云 DNS" value="aliyun" />
            <el-option label="GoDaddy" value="godaddy" />
            <el-option label="DNSPod（腾讯云）" value="dnspod" />
            <el-option label="Namecheap" value="namecheap" />
            <el-option label="Spaceship" value="spaceship" />
          </el-select>
        </el-form-item>
        <el-form-item
          v-if="form.type && providerConsoleUrl(form.type)"
          label="获取凭证"
        >
          <el-link
            type="primary"
            :href="providerConsoleUrl(form.type)"
            target="_blank"
            :underline="false"
            >前往 {{ typeLabel(form.type) }} 控制台获取 API 凭证 ↗</el-link
          >
        </el-form-item>
        <el-form-item label="备注名">
          <el-input v-model="form.name" placeholder="如：我的 Cloudflare" />
        </el-form-item>
        <el-form-item v-if="needsAccessKey" :label="accessKeyLabel">
          <el-input
            v-model="form.access_key"
            :placeholder="accessKeyPlaceholder"
          />
        </el-form-item>
        <el-form-item :label="secretLabel">
          <el-input
            v-model="form.secret_key"
            type="password"
            show-password
            :placeholder="secretPlaceholder"
          />
        </el-form-item>
        <el-form-item v-if="form.type === 'namecheap'" label="Client IP">
          <el-input
            v-model="form.client_ip"
            placeholder="Namecheap 后台白名单中填写的本机出口 IP"
          />
          <div class="text-gray-400 text-xs mt-1">
            Namecheap API 要求把调用方 IP 加入白名单，并作为 ClientIp 参数传入。
          </div>
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
import { ElMessage, ElMessageBox } from "element-plus";
import {
  getDnsAccounts,
  createDnsAccount,
  deleteDnsAccount,
  getZones
} from "@/api/domain-lite";

const list = ref<any[]>([]);
const loading = ref(false);
const dialog = ref(false);
const saving = ref(false);
const form = reactive({
  type: "cloudflare",
  name: "",
  access_key: "",
  secret_key: "",
  client_ip: ""
});

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

// 各服务商官方控制台 / API 凭证获取页，便于快速跳转配置
const providerConsoleUrl = (t: string): string | null => {
  switch (t) {
    case "cloudflare":
      return "https://dash.cloudflare.com/profile/api-tokens";
    case "aliyun":
      return "https://ram.console.aliyun.com/manage/ak";
    case "godaddy":
      return "https://developer.godaddy.com/keys/";
    case "dnspod":
      return "https://console.dnspod.cn/account/token/token";
    case "namecheap":
      return "https://www.namecheap.com/settings/tools/api/";
    case "spaceship":
      return "https://www.spaceship.com/application/api-manager/";
    default:
      return null;
  }
};

// 各服务商凭据字段提示
const needsAccessKey = computed(
  () => !["cloudflare", "dnspod"].includes(form.type)
);
const accessKeyLabel = computed(() => {
  if (form.type === "aliyun") return "AccessKeyId";
  if (form.type === "namecheap") return "ApiUser";
  if (form.type === "spaceship") return "Api Key";
  return "API Key";
});
const accessKeyPlaceholder = computed(() => {
  if (form.type === "namecheap") return "Namecheap ApiUser（与 UserName 相同）";
  if (form.type === "spaceship") return "Spaceship API Key";
  return "API Key / AccessKeyId";
});
const secretLabel = computed(() => {
  if (form.type === "dnspod") return "login_token";
  if (form.type === "aliyun") return "AccessKeySecret";
  if (form.type === "namecheap") return "ApiKey";
  if (form.type === "spaceship") return "Api Secret";
  return "API Token / Secret";
});
const secretPlaceholder = computed(() => {
  if (form.type === "dnspod") return "格式：ID,Token（DNSPod 控制台获取）";
  if (form.type === "namecheap") return "Namecheap ApiKey";
  if (form.type === "spaceship") return "Spaceship API Secret";
  return "API Token / Secret";
});

async function load() {
  loading.value = true;
  try {
    list.value = await getDnsAccounts();
  } finally {
    loading.value = false;
  }
}

function openAdd() {
  form.type = "cloudflare";
  form.name = "";
  form.access_key = "";
  form.secret_key = "";
  form.client_ip = "";
  dialog.value = true;
}

async function save() {
  if (!form.name || !form.secret_key) {
    ElMessage.warning("请填写完整信息");
    return;
  }
  if (form.type === "aliyun" && !form.access_key) {
    ElMessage.warning("请填写 AccessKeyId");
    return;
  }
  if (form.type === "namecheap" && !form.client_ip) {
    ElMessage.warning("请填写 Namecheap Client IP");
    return;
  }
  const payload: any = {
    type: form.type,
    name: form.name,
    access_key: form.access_key,
    secret_key: form.secret_key
  };
  if (form.type === "namecheap") {
    payload.ext = JSON.stringify({ client_ip: form.client_ip });
  }
  saving.value = true;
  try {
    await createDnsAccount(payload);
    ElMessage.success("已添加");
    dialog.value = false;
    load();
  } finally {
    saving.value = false;
  }
}

async function remove(row: any) {
  await ElMessageBox.confirm(`确认删除账户「${row.name}」？`, "提示", {
    type: "warning"
  });
  await deleteDnsAccount(row.id);
  ElMessage.success("已删除");
  load();
}

// 测试账户能否连通并取到域名（不离开当前页，便于快速排查）
async function testConn(row: any) {
  try {
    const res: any = await getZones(row.id);
    if (res.code !== 0) {
      ElMessage.error(`测试失败：${res.message}`);
      return;
    }
    const zones = res.data || [];
    if (zones.length === 0) {
      ElMessage.warning(
        `连接成功，但未返回任何域名。请检查 Cloudflare API Token 的 Zone·DNS·Read 权限与 Zone 作用域（建议 All zones）。`
      );
    } else {
      ElMessage.success(`连接成功，获取到 ${zones.length} 个域名`);
    }
  } catch (e: any) {
    const detail = e?.response?.data?.message || e?.message || "请求失败";
    ElMessage.error(`测试失败：${detail}`);
  }
}

onMounted(load);
</script>
