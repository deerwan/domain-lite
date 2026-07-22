<template>
  <div>
    <el-card v-loading="loading">
      <div class="flex justify-between items-center mb-4">
        <span class="text-lg font-medium">通知设置</span>
        <div>
          <el-button :loading="testing" @click="sendTest">发送测试</el-button>
          <el-button type="primary" :loading="saving" @click="save"
            >保存</el-button
          >
        </div>
      </div>

      <el-form :model="form" label-width="130px" class="mt-2">
        <el-form-item label="启用通知">
          <el-switch v-model="form.enabled" />
          <span class="text-gray-400 text-xs ml-3"
            >关闭后，定时同步仍会进行，但不会发送临期提醒</span
          >
        </el-form-item>

        <el-form-item label="通知渠道">
          <el-select v-model="form.type" class="w-60">
            <el-option label="飞书" value="feishu" />
            <el-option label="企业微信" value="wecom" />
            <el-option label="Telegram" value="telegram" />
            <el-option label="通用 Webhook" value="generic" />
          </el-select>
          <el-tag
            v-if="currentConfigured"
            type="success"
            size="small"
            class="ml-3"
            >当前渠道已配置</el-tag
          >
          <el-tag
            v-else-if="settings.has_webhook"
            type="info"
            size="small"
            class="ml-3"
            >已配置其他渠道</el-tag
          >
        </el-form-item>

        <!-- 实时跟随渠道显示的平台配置面板 -->
        <el-card shadow="never" class="platform-panel">
          <div class="flex items-center gap-2 mb-1">
            <el-icon class="text-blue-500"
              ><component :is="currentPlatform.icon"
            /></el-icon>
            <span class="font-medium">{{ currentPlatform.title }}</span>
            <span class="text-gray-400 text-xs">实时配置指引</span>
          </div>
          <p class="text-gray-500 text-sm mb-3">{{ currentPlatform.desc }}</p>

          <template v-if="currentPlatform.webhook">
            <el-form-item
              :label="currentPlatform.tokenMode ? 'BOT TOKEN' : 'Webhook 地址'"
              class="panel-item"
            >
              <el-input
                v-model="form.webhook_url"
                :type="currentPlatform.tokenMode ? 'text' : 'password'"
                :show-password="!currentPlatform.tokenMode"
                class="w-full"
                :placeholder="currentPlatform.webhookPlaceholder"
              />
              <div class="text-gray-400 text-xs mt-1">
                {{
                  settings.has_webhook && settings.type === form.type
                    ? "已配置（" +
                      settings.webhook_masked +
                      "），留空则保持不变"
                    : currentPlatform.webhookHint
                }}
              </div>
            </el-form-item>
          </template>

          <template v-if="currentPlatform.chatId">
            <el-form-item label="Chat ID" class="panel-item">
              <el-input
                v-model="form.chat_id"
                placeholder="Telegram 会话 / 频道 ID"
              />
            </el-form-item>
          </template>

          <el-alert
            :title="currentPlatform.howto"
            type="info"
            :closable="false"
            class="mt-1"
          />
        </el-card>

        <el-form-item label="临期阈值">
          <el-input-number v-model="form.threshold_days" :min="1" :max="365" />
          <span class="text-gray-400 text-xs ml-3"
            >剩余天数 ≤ 该值时提醒（天）</span
          >
        </el-form-item>

        <el-form-item label="自动同步间隔">
          <el-input-number
            v-model="form.sync_interval_min"
            :min="10"
            :max="10080"
            :step="10"
          />
          <span class="text-gray-400 text-xs ml-3"
            >自动查询 WHOIS 的周期（分钟），修改后下一轮生效</span
          >
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import {
  ChatDotRound,
  Connection,
  Promotion,
  Link
} from "@element-plus/icons-vue";
import {
  getNotifySettings,
  updateNotifySettings,
  testNotifySettings,
  type NotifySettings
} from "@/api/domain-lite";

interface PlatformMeta {
  title: string;
  desc: string;
  webhook: boolean;
  chatId: boolean;
  tokenMode?: boolean; // true 时 Webhook 框改填 Bot Token 并自动拼接地址
  webhookPlaceholder: string;
  webhookHint: string;
  howto: string;
  icon: any;
}

const platformMeta: Record<string, PlatformMeta> = {
  feishu: {
    title: "飞书",
    desc: "通过飞书群机器人推送临期提醒，支持 Markdown 富文本。",
    webhook: true,
    chatId: false,
    webhookPlaceholder: "https://open.feishu.cn/open-apis/bot/v2/hook/xxxx",
    webhookHint: "粘贴群机器人 Webhook 地址（open.feishu.cn 域名）",
    howto: "飞书群 → 设置 → 群机器人 → 添加机器人 → 复制 Webhook 地址",
    icon: ChatDotRound
  },
  wecom: {
    title: "企业微信",
    desc: "通过企业微信群机器人推送消息到指定群聊。",
    webhook: true,
    chatId: false,
    webhookPlaceholder:
      "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxx",
    webhookHint:
      "粘贴企业微信群机器人 Webhook 地址（qyapi.weixin.qq.com 域名）",
    howto: "企业微信客户端 → 群聊 → 添加群机器人 → 获取 Webhook 地址",
    icon: Promotion
  },
  telegram: {
    title: "Telegram",
    desc: "通过 Bot API 发送私信或频道消息，填写 Bot Token 与 Chat ID 即可。",
    webhook: true,
    chatId: true,
    tokenMode: true,
    webhookPlaceholder: "123456789:AAExxxxx（BotFather 提供的 Token）",
    webhookHint: "仅填写 BotFather 提供的 Token，系统自动拼接发送地址",
    howto: "向 @BotFather 创建机器人获取 Token；Chat ID 可用 @userinfobot 查询",
    icon: Connection
  },
  generic: {
    title: "通用 Webhook",
    desc: '向任意支持接收 JSON 的端点 POST { text: "..." }，便于接入自有系统。',
    webhook: true,
    chatId: false,
    webhookPlaceholder: "https://example.com/webhook",
    webhookHint: "填写任意可接收 POST JSON 的地址即可",
    howto: "填写任意可接收 POST JSON 的地址即可",
    icon: Link
  }
};

const currentPlatform = computed<PlatformMeta>(
  () => platformMeta[form.type] || platformMeta.feishu
);
// 当前所选渠道是否已配置
const currentConfigured = computed(
  () => settings.has_webhook && settings.type === form.type
);

// Telegram 填写的是 Bot Token，提交时自动拼成完整 sendMessage 地址；
// 若用户已粘贴完整 URL（以 http 开头）则原样提交。
function resolveWebhookUrl(type: string, raw: string): string {
  if (!raw) return raw;
  if (type === "telegram" && !/^https?:\/\//i.test(raw.trim())) {
    return `https://api.telegram.org/bot${raw.trim()}/sendMessage`;
  }
  return raw;
}

const settings = reactive<NotifySettings>({
  enabled: false,
  type: "feishu",
  chat_id: "",
  threshold_days: 30,
  sync_interval_min: 720,
  has_webhook: false,
  webhook_masked: ""
});

const form = reactive({
  enabled: false,
  type: "feishu",
  webhook_url: "",
  chat_id: "",
  threshold_days: 30,
  sync_interval_min: 720
});

const saving = ref(false);
const testing = ref(false);
const loading = ref(false);

async function load() {
  loading.value = true;
  try {
    const res: any = await getNotifySettings();
    if (res.code === 0 && res.data) {
      Object.assign(settings, res.data);
      form.enabled = res.data.enabled;
      form.type = res.data.type || "feishu";
      form.chat_id = res.data.chat_id || "";
      form.threshold_days = res.data.threshold_days || 30;
      form.sync_interval_min = res.data.sync_interval_min || 720;
      // webhook_url 不回显明文，保持为空表示沿用已保存值
    }
  } finally {
    loading.value = false;
  }
}

async function save() {
  saving.value = true;
  try {
    const res: any = await updateNotifySettings({
      enabled: form.enabled,
      type: form.type,
      webhook_url: resolveWebhookUrl(form.type, form.webhook_url),
      chat_id: form.chat_id,
      threshold_days: form.threshold_days,
      sync_interval_min: form.sync_interval_min
    });
    if (res.code === 0) {
      ElMessage.success("已保存");
      form.webhook_url = "";
      await load();
    } else {
      ElMessage.error(res.message || "保存失败");
    }
  } finally {
    saving.value = false;
  }
}

async function sendTest() {
  testing.value = true;
  try {
    const res: any = await testNotifySettings({
      type: form.type,
      webhook_url: resolveWebhookUrl(form.type, form.webhook_url),
      chat_id: form.chat_id,
      threshold_days: form.threshold_days
    });
    if (res.code === 0) {
      ElMessage.success("测试消息已发送，请检查收件端");
      form.webhook_url = "";
    } else {
      ElMessage.error(res.message || "发送失败");
    }
  } finally {
    testing.value = false;
  }
}

onMounted(load);
</script>

<style scoped>
.platform-panel {
  margin-bottom: 18px;
  background: #fafafa;
  border: 1px solid #ebeef5;
}

.platform-panel :deep(.panel-item) {
  margin-bottom: 12px;
}

.platform-panel :deep(.el-form-item__label) {
  color: #606266;
}
</style>
